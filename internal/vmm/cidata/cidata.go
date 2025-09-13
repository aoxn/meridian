package cidata

import (
	"embed"
	"fmt"
	"github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/vmm/meta"
	"github.com/aoxn/meridian/internal/vmm/sshutil"
	"github.com/diskfs/go-diskfs/filesystem/iso9660"
	"github.com/pkg/errors"
	"io"
	"k8s.io/klog/v2"
	"net"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

//go:embed cidata.TEMPLATE.d
var ciDataFS embed.FS

const ciFSRoot = "cidata.TEMPLATE.d"

func NewCloudInit(i *meta.Machine, sshmgr *sshutil.SSHMgr) BootDisk {
	return &CloudInit{ii: i, sshMgr: sshmgr}
}

type CloudInit struct {
	sshMgr *sshutil.SSHMgr
	ii     *meta.Machine
}

func (i *CloudInit) CreateBootDisk() error {
	pub, err := i.sshMgr.LoadPubKey()
	if err != nil {
		return errors.Wrap(err, "load public key for vm")
	}
	tplModel, err := NewTpl(i.ii, pub)
	if err != nil {
		return errors.Wrapf(err, "build init disk template")
	}

	layout, err := tplModel.Build(&ciDataFS, ciFSRoot, ValidateTemplateArgs)
	if err != nil {
		return err
	}

	bin, err := i.buildGuestBin()
	if err != nil {
		return errors.Wrap(err, "failed to build guest binary reader")
	}
	layout = append(layout, bin)

	instDir := i.ii.Dir()
	_ = ensurePath(instDir, true)
	klog.Infof("write iso file Path: %s", filepath.Join(instDir, v1.CIDataISO))
	if tplModel.VMType == string(v1.WSL2) {
		layout = append(layout, &Entry{
			Path:   "ssh_authorized_keys",
			reader: strings.NewReader(strings.Join(tplModel.SSHPubKeys, "\n")),
		})
		return writeDir(filepath.Join(instDir, "cidata"), layout)
	}

	return writeISO(filepath.Join(instDir, v1.CIDataISO), "cidata", layout)
}

func (i *CloudInit) buildGuestBin() (*Entry, error) {
	vmInfo := i.ii.Spec
	gfile := path.Join(
		i.ii.Dir(), "bin",
		fmt.Sprintf("meridian-guest.%s.%s",
			strings.ToLower(string(vmInfo.OS)), withArch(vmInfo.Arch)),
	)
	var guest io.ReadCloser
	guest, err := os.Open(gfile)
	if err != nil {
		return nil, err
	}
	entry := &Entry{
		reader: guest,
		closer: guest,
		Path:   "md-guest",
	}
	return entry, nil
}

func withArch(arch v1.Arch) string {
	switch arch {
	case v1.X8664:
		return "amd64"
	case v1.AARCH64:
		return "arm64"
	case v1.ARMV7L:
		return "arm"
	case v1.RISCV64:
		return string(arch)
	default:
		klog.Infof("Unknown arch: %s", arch)
		return string(arch)
	}
}

func writeDir(rootPath string, layout []*Entry) error {
	slices.SortFunc(layout, func(a, b *Entry) int {
		return strings.Compare(strings.ToLower(a.Path), strings.ToLower(b.Path))
	})

	err := os.RemoveAll(rootPath)
	if err != nil {
		return err
	}

	for _, f := range layout {
		dir := path.Dir(f.Path)
		if dir != "" && dir != "/" {
			pathl := filepath.Join(rootPath, dir)
			err := os.MkdirAll(pathl, 0o700)
			if err != nil {
				return err
			}
		}
		buf, err := io.ReadAll(f.reader)
		if err != nil {
			return err
		}
		err = os.WriteFile(filepath.Join(rootPath, f.Path), buf, 0o700)
		if err != nil {
			return err
		}
		if f.closer != nil {
			_ = f.closer.Close()
		}
	}

	return nil
}

func writeISO(isoPath, label string, layout []*Entry) error {
	cleanUp(isoPath)

	iso, err := os.Create(isoPath)
	if err != nil {
		return err
	}

	defer iso.Close()

	workdir, err := os.MkdirTemp("", "diskfs_iso")
	if err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		// go-embed unfortunately needs unix Path
		workdir = filepath.ToSlash(workdir)
	}

	fs, err := iso9660.Create(iso, 0, 0, 0, workdir)
	if err != nil {
		return err
	}

	// write to fs
	for _, f := range layout {
		dir := path.Dir(f.Path)
		if dir != "" && dir != "/" {
			err = fs.Mkdir(dir)
			if err != nil {
				return err
			}
		}
		data, err := fs.OpenFile(f.Path, os.O_CREATE|os.O_RDWR)
		if err != nil {
			return err
		}
		_, err = io.Copy(data, f.reader)
		if err != nil {
			return err
		}
		if f.closer != nil {
			_ = f.closer.Close()
		}
		klog.V(6).Infof("debug generate cloud-init disk: write iso part, %s", f.Path)
	}

	finalizeOptions := iso9660.FinalizeOptions{
		RockRidge:        true,
		VolumeIdentifier: label,
	}
	return fs.Finalize(finalizeOptions)
}

func IsISO9660(imagePath string) (bool, error) {
	imageFile, err := os.Open(imagePath)
	if err != nil {
		return false, err
	}
	defer imageFile.Close()

	fileInfo, err := imageFile.Stat()
	if err != nil {
		return false, err
	}
	_, err = iso9660.Read(imageFile, fileInfo.Size(), 0, 0)
	return err == nil, nil
}

func ensurePath(name string, isdir bool) error {
	if isdir {
		return os.MkdirAll(name, 0o700)
	}
	dir := path.Dir(name)
	if dir != "" && dir != "/" {
		return os.MkdirAll(dir, 0o700)
	}
	return nil
}

func cleanUp(name string) {
	_ = os.RemoveAll(name)
}

var netLookupIP = func(host string) []net.IP {
	ips, err := net.LookupIP(host)
	if err != nil {
		klog.Infof("net.LookupIP %s: %s", host, err)
		return nil
	}

	return ips
}

func ValidateTemplateArgs(args *TemplateArgs) error {
	if args.User.Username == "root" {
		return errors.New("field User must not be \"root\"")
	}
	if args.User.Uid == "" {
		return errors.New("field UID must not be 0")
	}
	if args.Home == "" {
		return errors.New("field Home must be set")
	}
	if len(args.SSHPubKeys) == 0 {
		return errors.New("field SSHPubKeys must be set")
	}
	for i, m := range args.Mounts {
		f := m.MountPoint
		if !path.IsAbs(f) {
			return fmt.Errorf("field mounts[%d] must be absolute, got %q", i, f)
		}
	}
	return nil
}
