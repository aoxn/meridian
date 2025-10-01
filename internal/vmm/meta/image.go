package meta

import (
	"context"
	"encoding/json"
	"fmt"
	api "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/tool/downloader"
	"github.com/cheggaaa/pb/v3"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"
	"os"
	"path"
)

type Status struct {
	Err  string       `json:"error"`
	Data []StatusData `json:"data"`
}

type StatusData struct {
	Id      string `json:"id"`
	Current int64  `json:"current"`
	Total   int64  `json:"total"`
}

type image struct {
	root string
}

func (m *image) Dir() string {
	return m.rootLocation()
}

func (m *image) rootLocation(name ...string) string {
	return path.Join(m.root, "images", path.Join(name...))
}

func (m *image) Get(key string) (*Image, error) {
	pathName := m.rootLocation(key)
	info, err := os.Stat(pathName)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		return nil, errors.Wrapf(err, "NotFound: %s", key)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", pathName)
	}
	return m.load(path.Join(pathName, imageJson))
}

func (m *image) List() ([]*Image, error) {
	var machines []*Image
	pathName := m.Dir()

	info, err := os.Stat(pathName)
	if err != nil {
		return machines, err
	}
	if !info.IsDir() {
		return machines, fmt.Errorf("%s is not a directory", pathName)
	}
	// walk directory
	en, err := os.ReadDir(pathName)
	if err != nil {
		return machines, err
	}
	for _, dir := range en {
		dirName := dir.Name()
		img, err := m.Get(dirName)
		if err != nil {
			klog.Errorf("unexpected image dir name: %s", dirName)
			continue
		}
		machines = append(machines, img)
	}
	return machines, nil
}

func (m *image) Create(image *Image) error {
	pathName := m.rootLocation(image.Name)
	_, err := os.Stat(pathName)
	if err == nil {
		return fmt.Errorf("%s already exists", pathName)
	}
	err = os.MkdirAll(pathName, 0755)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(image, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path.Join(pathName, imageJson), data, 0644)
}

func (m *image) Update(image *Image) error {
	pathName := m.rootLocation(image.Name)
	_, err := os.Stat(pathName)
	if err != nil {
		return fmt.Errorf("%s not exists", pathName)
	}
	data, err := json.MarshalIndent(image, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path.Join(pathName, imageJson), data, 0644)
}

func (m *image) Remove(name string) error {
	if name == "" {
		return fmt.Errorf("image name is empty")
	}
	return os.RemoveAll(m.rootLocation(name))
}

func (m *image) load(machineUri string) (*Image, error) {
	data, err := os.ReadFile(machineUri)
	if err != nil {
		return nil, err
	}
	var mch Image
	err = json.Unmarshal(data, &mch)
	return &mch, err
}

type PullOpt struct {
	Location      string
	Digest        digest.Digest
	DownloadBar   *pb.ProgressBar
	DecompressBar *pb.ProgressBar
}

func (m *image) Pull(ctx context.Context, name string, opt *PullOpt) error {
	var err error
	if opt == nil {
		return fmt.Errorf("empty location")
	}
	f := api.FindImage(name)
	if f == nil {
		return fmt.Errorf("image not found: %s", name)
	}

	var downloadOpts = []downloader.Opt{
		downloader.WithCache(),
		downloader.WithDecompress(false),
		downloader.WithDescription(fmt.Sprintf("%s (%s)", "Guest Vm Image", path.Base(opt.Location))),
		downloader.WithExpectedDigest(opt.Digest),
	}
	if opt.DownloadBar != nil {
		downloadOpts = append(downloadOpts, downloader.WithDownloadBar(opt.DownloadBar))
	}
	if opt.DecompressBar != nil {
		downloadOpts = append(downloadOpts, downloader.WithDecompressBar(opt.DecompressBar))
	}
	res, err := downloader.Download(ctx, "", opt.Location, downloadOpts...)
	klog.V(7).Infof("pull image %s from %s with r=[%v]", name, opt.Location, res)
	if err != nil {
		return errors.Wrapf(err, "failed to pull image %s", name)
	}
	img := &Image{
		Name:     name,
		Digest:   opt.Digest,
		OS:       f.OS,
		Arch:     string(f.Arch),
		Version:  f.Version,
		Labels:   f.Labels,
		Location: opt.Location,
	}
	// save image
	return m.Update(img)
}
