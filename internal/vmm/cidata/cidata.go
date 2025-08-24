package cidata

import (
	"fmt"
	"github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/vmm/meta"
	"github.com/aoxn/meridian/internal/vmm/sshutil"
	"io"
	"k8s.io/klog/v2"
	"net"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/diskfs/go-diskfs/filesystem/iso9660"
)

func NewCloudInit(i *meta.Machine, sshmgr *sshutil.SSHMgr) *CloudInit {
	return &CloudInit{ii: i, sshMgr: sshmgr}
}

type CloudInit struct {
	sshMgr *sshutil.SSHMgr
	ii     *meta.Machine
}

func (i *CloudInit) GenCIISO() error {
	u := &user.User{
		Uid:      "1000",
		Username: i.ii.Name,
		HomeDir:  fmt.Sprintf("/home/%s", i.ii.Name),
	}
	var pubs []string
	pub, err := i.sshMgr.LoadPubKey()
	if err != nil {
		return err
	}
	for _, p := range pub {
		pubs = append(pubs, p.Content)
		klog.Infof("append pubkey: %s", p.Content)
	}

	vmInfo := i.ii.Spec
	args := TemplateArgs{
		Name:       i.ii.Name,
		User:       u,
		VMType:     string(vmInfo.VMType),
		TimeZone:   vmInfo.TimeZone,
		SSHPubKeys: pubs,
		MountType:  "virtiofs",
		CACerts: CACerts{
			RemoveDefaults: false,
		},
		Home: u.HomeDir,
	}
	for k, n := range vmInfo.Mounts {
		mount := Mount{
			MountPoint: n.MountPoint,
			Type:       "virtiofs",
			Tag:        fmt.Sprintf("mount%d", k),
		}
		if vmInfo.VMType == "vz" {
			mount.Type = "virtiofs"
		}
		args.Mounts = append(args.Mounts, mount)
	}

	for _, n := range vmInfo.Networks {
		network := Network{
			Interface:  "enp0s1",
			MACAddress: n.MACAddress,
			IpAddress:  n.Address,
			IpGateway:  n.IpGateway,
		}
		args.Networks = append(args.Networks, network)
	}
	klog.Infof("network addresses: %+v", args.Networks[0])
	// change instance id on every boot so network config will be processed again
	args.IID = fmt.Sprintf("iid-%d", time.Now().Unix())

	if err := ValidateTemplateArgs(args); err != nil {
		return err
	}

	layout, err := ExecuteTemplate(args)
	if err != nil {
		return err
	}

	gfile := path.Join(i.ii.Dir(), "bin", fmt.Sprintf("meridian-guest.%s.%s", strings.ToLower(string(vmInfo.OS)), withArch(vmInfo.Arch)))
	var guest io.ReadCloser
	guest, err = os.Open(gfile)
	if err != nil {
		return err
	}
	defer guest.Close()
	layout = append(layout, entry{
		path:   "md-guest",
		reader: guest,
		closer: guest,
	})
	instDir := i.ii.Dir()
	_ = ensurePath(instDir, true)
	klog.Infof("write iso file path: %s", filepath.Join(instDir, v1.CIDataISO))
	if args.VMType == string(v1.WSL2) {
		layout = append(layout, entry{
			path:   "ssh_authorized_keys",
			reader: strings.NewReader(strings.Join(args.SSHPubKeys, "\n")),
		})
		return writeDir(filepath.Join(instDir, "cidata"), layout)
	}

	return writeISO(filepath.Join(instDir, v1.CIDataISO), "cidata", layout)
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

func writeDir(rootPath string, layout []entry) error {
	slices.SortFunc(layout, func(a, b entry) int {
		return strings.Compare(strings.ToLower(a.path), strings.ToLower(b.path))
	})

	err := os.RemoveAll(rootPath)
	if err != nil {
		return err
	}

	for _, f := range layout {
		dir := path.Dir(f.path)
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
		err = os.WriteFile(filepath.Join(rootPath, f.path), buf, 0o700)
		if err != nil {
			return err
		}
		if f.closer != nil {
			_ = f.closer.Close()
		}
	}

	return nil
}

func writeISO(isoPath, label string, layout []entry) error {
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
		// go-embed unfortunately needs unix path
		workdir = filepath.ToSlash(workdir)
	}

	fs, err := iso9660.Create(iso, 0, 0, 0, workdir)
	if err != nil {
		return err
	}

	// write to fs
	for _, f := range layout {
		dir := path.Dir(f.path)
		if dir != "" && dir != "/" {
			err = fs.Mkdir(dir)
			if err != nil {
				return err
			}
		}
		data, err := fs.OpenFile(f.path, os.O_CREATE|os.O_RDWR)
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
		klog.V(6).Infof("debug: write iso part, %s", f.path)
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

type entry struct {
	path   string
	reader io.Reader
	closer io.Closer
}

var netLookupIP = func(host string) []net.IP {
	ips, err := net.LookupIP(host)
	if err != nil {
		klog.Infof("net.LookupIP %s: %s", host, err)
		return nil
	}

	return ips
}
