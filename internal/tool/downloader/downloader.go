package downloader

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/cheggaaa/pb/v3"
	"github.com/docker/go-units"
	"github.com/mattn/go-isatty"
	gerrors "github.com/pkg/errors"
	"io"
	"k8s.io/klog/v2"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/containerd/continuity/fs"
	"github.com/opencontainers/go-digest"
)

func New(size int64) (*pb.ProgressBar, error) {
	bar := pb.New64(size)

	bar.Set(pb.Bytes, true)
	if isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd()) {
		bar.SetTemplateString(`{{counters . }} {{bar . | green }} {{percent .}} {{speed . "%s/s"}}`)
		bar.SetRefreshRate(200 * time.Millisecond)
	} else {
		bar.Set(pb.Terminal, false)
		bar.Set(pb.ReturnSymbol, "\n")
		bar.SetTemplateString(`{{counters . }} ({{percent .}}) {{speed . "%s/s"}}`)
		bar.SetRefreshRate(5 * time.Second)
	}
	bar.SetWidth(80)
	bar.SetWriter(os.Stderr)
	if err := bar.Err(); err != nil {
		return nil, err
	}

	return bar, nil
}

// HideProgress is used only for testing.
var HideProgress bool

// hideBar is used only for testing.
func hideBar(bar *pb.ProgressBar) {
	bar.Set(pb.ReturnSymbol, "")
	bar.SetTemplateString("")
}

type Status = string

const (
	StatusUnknown    Status = ""
	StatusDownloaded Status = "downloaded"
	StatusSkipped    Status = "skipped"
	StatusUsedCache  Status = "used-cache"
)

type Result struct {
	Status          Status
	CachePath       string // "/Users/foo/Library/Caches/lima/download/by-url-sha256/<SHA256_OF_URL>/data"
	LastModified    time.Time
	ContentType     string
	ValidatedDigest bool
}

type options struct {
	cacheDir       string // default: empty (disables caching)
	decompress     bool   // default: false (keep compression)
	description    string // default: url
	canonical      string
	expectedDigest digest.Digest
	downloadBar    *pb.ProgressBar
	decompressBar  *pb.ProgressBar
}

type Opt func(*options) error

// WithCache enables caching using filepath.Join(os.UserCacheDir(), "lima") as the cache dir.
func WithCache() Opt {
	return func(o *options) error {
		ucd, err := os.UserCacheDir()
		if err != nil {
			return err
		}
		cacheDir := filepath.Join(ucd, "meridian")
		return WithCacheDir(cacheDir)(o)
	}
}

// WithCacheDir enables caching using the specified dir.
// Empty value disables caching.
func WithCacheDir(cacheDir string) Opt {
	return func(o *options) error {
		o.cacheDir = cacheDir
		return nil
	}
}

// WithDescription adds a user description of the download.
func WithDescription(description string) Opt {
	return func(o *options) error {
		o.description = description
		return nil
	}
}

// WithDecompress decompress the download from the cache.
func WithDecompress(decompress bool) Opt {
	return func(o *options) error {
		o.decompress = decompress
		return nil
	}
}

func WithDecompressBar(bar *pb.ProgressBar) Opt {
	return func(o *options) error {
		o.decompressBar = bar
		return nil
	}
}

func WithDownloadBar(bar *pb.ProgressBar) Opt {
	return func(o *options) error {
		o.downloadBar = bar
		return nil
	}
}

// WithExpectedDigest is used to validate the downloaded file against the expected digest.
//
// The digest is not verified in the following cases:
//   - The digest was not specified.
//   - The file already exists in the local target path.
//
// When the `data` file exists in the cache dir with `<ALGO>.digest` file,
// the digest is verified by comparing the content of `<ALGO>.digest` with the expected
// digest string. So, the actual digest of the `data` file is not computed.
func WithExpectedDigest(expectedDigest digest.Digest) Opt {
	return func(o *options) error {
		if expectedDigest != "" {
			if !expectedDigest.Algorithm().Available() {
				return fmt.Errorf("expected digest algorithm %q is not available", expectedDigest.Algorithm())
			}
			if err := expectedDigest.Validate(); err != nil {
				return err
			}
		}

		o.expectedDigest = expectedDigest
		return nil
	}
}

func readFile(path string) string {
	if path == "" {
		return ""
	}
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

func readTime(path string) time.Time {
	if path == "" {
		return time.Time{}
	}
	if _, err := os.Stat(path); err != nil {
		return time.Time{}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}
	}
	t, err := time.Parse(http.TimeFormat, string(b))
	if err != nil {
		return time.Time{}
	}
	return t
}

// Download downloads the remote resource into the local path.
//
// Download caches the remote resource if WithCache or WithCacheDir option is specified.
// Local files are not cached.
//
// When the local path already exists, Download returns Result with StatusSkipped.
// (So, the local path cannot be set to /dev/null for "caching only" mode.)
//
// The local path can be an empty string for "caching only" mode.
func Download(ctx context.Context, local, remote string, opts ...Opt) (*Result, error) {
	var o options
	for _, f := range opts {
		if err := f(&o); err != nil {
			return nil, err
		}
	}
	var localPath string
	if local == "" {
		if o.cacheDir == "" {
			return nil, fmt.Errorf("caching-only mode requires the cache directory to be specified")
		}
	} else {
		var err error
		localPath, err = canonicalLocalPath(local)
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(localPath); err == nil {
			klog.Infof("file %q already exists, skipping downloading from %q (and skipping digest validation)", localPath, remote)
			res := &Result{
				Status:          StatusSkipped,
				ValidatedDigest: false,
			}
			return res, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}

		localPathDir := filepath.Dir(localPath)
		if err := os.MkdirAll(localPathDir, 0o755); err != nil {
			return nil, err
		}
	}

	ext := path.Ext(remote)
	if IsLocal(remote) {
		if err := copyLocal(ctx, localPath, remote, ext, o.decompress, o.description, o.expectedDigest, o.decompressBar); err != nil {
			return nil, err
		}
		res := &Result{
			Status:          StatusDownloaded,
			ValidatedDigest: o.expectedDigest != "",
		}
		return res, nil
	}

	o.canonical = localPath

	if o.cacheDir == "" {
		err := downloadFrom(ctx, remote, o)
		if err != nil {
			return nil, err
		}
		res := &Result{
			Status:          StatusDownloaded,
			ValidatedDigest: o.expectedDigest != "",
		}
		return res, nil
	}

	shad := cacheDirectoryPath(o.cacheDir, remote)
	shadData := filepath.Join(shad, "data")
	shadTime := filepath.Join(shad, "time")
	shadType := filepath.Join(shad, "type")
	shadDigest, err := cacheDigestPath(shad, o.expectedDigest)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(shadData); err == nil {
		klog.Infof("file %q is cached as %q", localPath, shadData)
		if _, err := os.Stat(shadDigest); err == nil {
			klog.Infof("Comparing digest %q with the cached digest file %q, not computing the actual digest of %q",
				o.expectedDigest, shadDigest, shadData)
			if err := validateCachedDigest(shadDigest, o.expectedDigest); err != nil {
				return nil, err
			}
			if err := copyLocal(ctx, localPath, shadData, ext, o.decompress, "", "", o.decompressBar); err != nil {
				return nil, err
			}
		} else {
			if err := copyLocal(ctx, localPath, shadData, ext, o.decompress, o.description, o.expectedDigest, o.decompressBar); err != nil {
				return nil, err
			}
		}
		res := &Result{
			Status:          StatusUsedCache,
			CachePath:       shadData,
			LastModified:    readTime(shadTime),
			ContentType:     readFile(shadType),
			ValidatedDigest: o.expectedDigest != "",
		}
		return res, nil
	}
	if err := os.MkdirAll(shad, 0o755); err != nil {
		return nil, err
	}
	shadURL := filepath.Join(shad, "url")
	if err := os.WriteFile(shadURL, []byte(remote), 0o644); err != nil {
		return nil, err
	}
	err = downloadFrom(ctx, remote, o)
	if err != nil {
		return nil, err
	}
	// no need to pass the digest to copyLocal(), as we already verified the digest
	if err := copyLocal(ctx, localPath, shadData, ext, o.decompress, "", "", o.decompressBar); err != nil {
		return nil, err
	}
	if shadDigest != "" && o.expectedDigest != "" {
		if err := os.WriteFile(shadDigest, []byte(o.expectedDigest.String()), 0o644); err != nil {
			return nil, err
		}
	}
	res := &Result{
		Status:          StatusDownloaded,
		CachePath:       shadData,
		LastModified:    readTime(shadTime),
		ContentType:     readFile(shadType),
		ValidatedDigest: o.expectedDigest != "",
	}
	return res, nil
}

// Cached checks if the remote resource is in the cache.
//
// Download caches the remote resource if WithCache or WithCacheDir option is specified.
// Local files are not cached.
//
// When the cache path already exists, Cached returns Result with StatusUsedCache.
func Cached(remote string, opts ...Opt) (*Result, error) {
	var o options
	for _, f := range opts {
		if err := f(&o); err != nil {
			return nil, err
		}
	}
	if o.cacheDir == "" {
		return nil, fmt.Errorf("caching-only mode requires the cache directory to be specified")
	}
	if IsLocal(remote) {
		return nil, fmt.Errorf("local files are not cached")
	}

	shad := cacheDirectoryPath(o.cacheDir, remote)
	shadData := filepath.Join(shad, "data")
	shadTime := filepath.Join(shad, "time")
	shadType := filepath.Join(shad, "type")
	shadDigest, err := cacheDigestPath(shad, o.expectedDigest)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(shadData); err != nil {
		return nil, err
	}
	if _, err := os.Stat(shadDigest); err != nil {
		if err := validateCachedDigest(shadDigest, o.expectedDigest); err != nil {
			return nil, err
		}
	} else {
		if err := validateLocalFileDigest(shadData, o.expectedDigest); err != nil {
			return nil, err
		}
	}
	res := &Result{
		Status:          StatusUsedCache,
		CachePath:       shadData,
		LastModified:    readTime(shadTime),
		ContentType:     readFile(shadType),
		ValidatedDigest: o.expectedDigest != "",
	}
	return res, nil
}

// cacheDirectoryPath returns the cache subdirectory path.
//   - "url" file contains the url
//   - "data" file contains the data
//   - "time" file contains the time (Last-Modified header)
//   - "type" file contains the type (Content-Type header)
func cacheDirectoryPath(cacheDir, remote string) string {
	return filepath.Join(cacheDir, "download", "by-url-sha256", fmt.Sprintf("%x", sha256.Sum256([]byte(remote))))
}

// cacheDigestPath returns the cache digest file path.
//   - "<ALGO>.digest" contains the digest
func cacheDigestPath(shad string, expectedDigest digest.Digest) (string, error) {
	shadDigest := ""
	if expectedDigest != "" {
		algo := expectedDigest.Algorithm().String()
		if strings.Contains(algo, "/") || strings.Contains(algo, "\\") {
			return "", fmt.Errorf("invalid digest algorithm %q", algo)
		}
		shadDigest = filepath.Join(shad, algo+".digest")
	}
	return shadDigest, nil
}

func IsLocal(s string) bool {
	return !strings.Contains(s, "://") || strings.HasPrefix(s, "file://")
}

// canonicalLocalPath canonicalizes the local path string.
//   - Make sure the file has no scheme, or the `file://` scheme
//   - If it has the `file://` scheme, strip the scheme and make sure the filename is absolute
//   - Expand a leading `~`, or convert relative to absolute name
func canonicalLocalPath(s string) (string, error) {
	if s == "" {
		return "", fmt.Errorf("got empty path")
	}
	if !IsLocal(s) {
		return "", fmt.Errorf("got non-local path: %q", s)
	}
	if strings.HasPrefix(s, "file://") {
		res := strings.TrimPrefix(s, "file://")
		if !filepath.IsAbs(res) {
			return "", fmt.Errorf("got non-absolute path %q", res)
		}
		return res, nil
	}
	return v1.Expand(s)
}

func copyLocal(ctx context.Context, dst, src, ext string, decompress bool, description string, expectedDigest digest.Digest, bar *pb.ProgressBar) error {
	srcPath, err := canonicalLocalPath(src)
	if err != nil {
		return err
	}

	if expectedDigest != "" {
		klog.Infof("verifying digest of local file %q (%s)", srcPath, expectedDigest)
	}
	if err := validateLocalFileDigest(srcPath, expectedDigest); err != nil {
		return err
	}

	if dst == "" {
		// empty dst means caching-only mode
		return nil
	}
	dstPath, err := canonicalLocalPath(dst)
	if err != nil {
		return err
	}
	if decompress {
		command := decompressor(ext)
		if command != "" {
			switch command {
			case "tar":
				return decompressTar(ctx, command, dstPath, srcPath, ext, description, bar)
			}
			return decompressLocal(ctx, command, dstPath, srcPath, ext, description, bar)
		}
	}
	// TODO: progress bar for copy
	return fs.CopyFile(dstPath, srcPath)
}

func decompressor(ext string) string {
	switch ext {
	case ".gz":
		return "tar"
	case ".bz2":
		return "bzip2"
	case ".xz":
		return "xz"
	case ".zst":
		return "zstd"
	default:
		return ""
	}
}

func decompressTar(ctx context.Context, decompressCmd, dst, src, ext, description string, bar *pb.ProgressBar) error {
	klog.Infof("decompressing %s with %v", ext, decompressCmd)

	st, err := os.Stat(src)
	if err != nil {
		return err
	}

	if bar == nil {
		bar, err = New(st.Size())
		if err != nil {
			return err
		}
	} else {
		bar.SetTotal(st.Size())
	}
	if HideProgress {
		hideBar(bar)
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	buf := new(bytes.Buffer)
	cmd := exec.CommandContext(ctx, decompressCmd, "-xf", "-", "-C", filepath.Dir(dst)) // -d --decompress
	cmd.Stdin = bar.NewProxyReader(in)

	if !HideProgress {
		if description == "" {
			description = filepath.Base(src)
		}
		klog.Infof("Decompressing tar from [%s] \n\t\tinto [%s]", src, dst)
	}
	bar.Start()
	err = cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			ee.Stderr = buf.Bytes()
		}
		klog.Infof("decompressor tar with error: %s", err.Error())
	}
	bar.Finish()
	return err
}

func decompressLocal(ctx context.Context, decompressCmd, dst, src, ext, description string, bar *pb.ProgressBar) error {
	klog.Infof("decompressing %s with %v", ext, decompressCmd)

	st, err := os.Stat(src)
	if err != nil {
		return err
	}
	if bar == nil {
		bar, err = New(st.Size())
		if err != nil {
			return err
		}
	} else {
		bar.SetTotal(st.Size())
	}
	if HideProgress {
		hideBar(bar)
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	buf := new(bytes.Buffer)
	cmd := exec.CommandContext(ctx, decompressCmd, "-d") // -d --decompress
	cmd.Stdin = bar.NewProxyReader(in)
	cmd.Stdout = out
	cmd.Stderr = buf
	if !HideProgress {
		if description == "" {
			description = filepath.Base(src)
		}
		klog.Infof("Decompressing from [%s] \n\t\tinto [%s]", src, dst)
	}
	bar.Start()
	err = cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			ee.Stderr = buf.Bytes()
		}
		klog.Infof("decompressor with error: %s", err.Error())
	}
	bar.Finish()
	return err
}

func validateCachedDigest(shadDigest string, expectedDigest digest.Digest) error {
	if expectedDigest == "" {
		return nil
	}
	shadDigestB, err := os.ReadFile(shadDigest)
	if err != nil {
		return err
	}
	shadDigestS := strings.TrimSpace(string(shadDigestB))
	if shadDigestS != expectedDigest.String() {
		return fmt.Errorf("expected digest %q, got %q", expectedDigest, shadDigestS)
	}
	return nil
}

func validateLocalFileDigest(localPath string, expectedDigest digest.Digest) error {
	if localPath == "" {
		return fmt.Errorf("validateLocalFileDigest: got empty localPath")
	}
	if expectedDigest == "" {
		return nil
	}
	algo := expectedDigest.Algorithm()
	if !algo.Available() {
		return fmt.Errorf("expected digest algorithm %q is not available", algo)
	}
	r, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer r.Close()
	actualDigest, err := algo.FromReader(r)
	if err != nil {
		return err
	}
	if actualDigest != expectedDigest {
		return fmt.Errorf("expected digest %q, got %q", expectedDigest, actualDigest)
	}
	return nil
}

func openAt(tmpFile string, resume bool) (*os.File, int64, error) {
	var f *os.File
	if resume {
		// 支持断点续传
		f, err := os.OpenFile(
			tmpFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			return f, 0, err
		}

		fileInfo, err := f.Stat()
		if err != nil {
			return f, 0, err
		}
		klog.V(5).Infof("download start from breaking point: %d/%s", fileInfo.Size(), units.HumanSize(float64(fileInfo.Size())))
		return f, fileInfo.Size(), nil
	}
	// 不支持断点续传
	klog.V(5).Infof("download from groud up: clean up cache")
	err := os.RemoveAll(tmpFile)
	if err != nil {
		return f, 0, gerrors.Wrapf(err, "clean tmp file %q", tmpFile)
	}
	f, err = os.Create(tmpFile)
	if err != nil {
		return f, 0, gerrors.Wrapf(err, "create tmp file %q", tmpFile)
	}
	return f, 0, nil
}

func (o *options) newBar(total, current int64) (*pb.ProgressBar, error) {
	var bar = o.downloadBar
	if bar == nil {
		pbar, err := New(total)
		if err != nil {
			return nil, err
		}
		bar = pbar
	}
	if HideProgress {
		hideBar(bar)
	}
	bar.SetTotal(total)
	bar.SetCurrent(current)
	return bar, nil
}

func setLastInfo(at, content string) {
	if content == "" {
		return
	}
	err := os.WriteFile(at, []byte(content), 0o644)
	if err != nil {
		klog.Errorf("write last modified error: %s", err.Error())
	}
}

func downloadFrom(ctx context.Context, url string, o options) error {

	shad := o.canonical
	shadData := filepath.Join(shad, "data")
	shadTime := filepath.Join(shad, "time")
	shadType := filepath.Join(shad, "type")

	var locaTmp = shadData + ".tmp"

	if shadData == "" {
		return fmt.Errorf("downloadHTTP: got empty localPath")
	}

	klog.Infof("downloading from: [%q] -> [%q]", url, shadData)

	total, support, err := resumeInfo(url)
	if err != nil {
		return gerrors.Wrapf(err, "decide resumable=[%t], total=[%d], %q", support, total, url)
	}

	f, current, err := openAt(locaTmp, support)
	if err != nil {
		return gerrors.Wrapf(err, "open tmp file")
	}
	defer f.Close()

	if current != 0 && total == current {
		// 断点续传，并且已经完成
		return renameTo(locaTmp, shadData)
	}

	r, err := getStream(ctx, url, current)
	if err != nil {
		return gerrors.Wrapf(err, "download with header")
	}

	defer r.Body.Close()

	setLastInfo(shadTime, r.Header.Get("Last-Modified"))
	setLastInfo(shadType, r.Header.Get("Content-Type"))

	bar, err := o.newBar(total, current)
	if err != nil {
		return gerrors.Wrapf(err, "new progress bar: [%d/%d]", current, total)
	}

	writers := []io.Writer{f}
	var digester digest.Digester
	if o.expectedDigest != "" {
		algo := o.expectedDigest.Algorithm()
		if !algo.Available() {
			return fmt.Errorf("unsupported digest algorithm %q", algo)
		}
		digester = algo.Digester()
		hasher := digester.Hash()
		writers = append(writers, hasher)
	}
	multiWriter := io.MultiWriter(writers...)

	bar.Start()
	if _, err := io.Copy(multiWriter, bar.NewProxyReader(r.Body)); err != nil {
		return err
	}
	bar.Finish()

	if digester != nil {
		actualDigest := digester.Digest()
		if actualDigest != o.expectedDigest {
			return fmt.Errorf("expected digest %q, got %q", o.expectedDigest, actualDigest)
		}
	}

	if err := f.Sync(); err != nil {
		return err
	}
	return renameTo(locaTmp, shadData)
}

func getStream(ctx context.Context, url string, current int64) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, http.NoBody)
	if err != nil {
		return nil, gerrors.Wrapf(err, "get request")
	}

	setDownloadUserAgent(req)

	req.Header.Set("Range", fmt.Sprintf("bytes=%d-", current))

	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, gerrors.Wrapf(err, "do http request")
	}
	if r == nil {
		return nil, errors.New("nil response")
	}

	var ReadBodyMaxLength = 64 * 1024

	if r.StatusCode/100 != 2 {
		defer r.Body.Close()
		b, _ := readAtMost(r.Body, ReadBodyMaxLength)
		klog.Infof("http get with unexpected status code: %d", r.StatusCode)
		return nil, fmt.Errorf("get stream: %s", b)
	}
	return r, nil
}

func readAtMost(r io.Reader, maxBytes int) ([]byte, error) {
	lr := &io.LimitedReader{
		R: r,
		N: int64(maxBytes),
	}
	b, err := io.ReadAll(lr)
	if err != nil {
		return b, err
	}
	if lr.N == 0 {
		return b, fmt.Errorf("expected at most %d bytes, got more", maxBytes)
	}
	return b, nil
}

func renameTo(localTmp, local string) error {
	if err := os.RemoveAll(local); err != nil {
		return err
	}
	return os.Rename(localTmp, local)
}

// resumeInfo 返回 true 表示 Apple CDN 支持断点续传
func resumeInfo(url string) (int64, bool, error) {
	var client = http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return http.ErrUseLastResponse
			}
			return nil
		},
		Timeout: 10 * time.Second,
	}
	// 1. 发 Range 试探
	req, _ := http.NewRequest("GET", url, nil)

	setDownloadUserAgent(req)

	req.Header.Set("Range", "bytes=0-")

	r, err := client.Do(req)
	if err != nil {
		return 0, false, gerrors.Wrapf(err, "connect to")
	}

	_ = r.Body.Close()

	klog.V(5).Infof("remote server break point infomation: HttpCode=[%d], "+
		"Header=[ContentLength=%s]/[AcceptRange=%s] Length=[%d]", r.StatusCode, r.Header.Get("Content-Length"), r.Header.Get("Accept-Ranges"), r.ContentLength)

	return r.ContentLength, r.StatusCode == http.StatusPartialContent || r.Header.Get("Accept-Ranges") != "" ||
		r.Header.Get("Content-Range") != "", nil
}

func setDownloadUserAgent(req *http.Request) {
	req.Header.Set("User-Agent", fmt.Sprintf("Safari/18615.3.12.11.%d", rand.Int()))
}
