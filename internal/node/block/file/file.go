package file

import (
	"context"
	"fmt"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/node/host"
	"github.com/aoxn/meridian/internal/tool/cmd"
	"github.com/aoxn/meridian/internal/vma/model"
	"github.com/pkg/errors"
	tar "github.com/verybluebot/tarinator-go"
	"k8s.io/klog/v2"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	FILE_BINARY = "bin"
	FILE_PKG    = "pkg"
)

const (
	PKG_DOCKER     = "docker"
	PKG_CONTAINERD = "containerd"
	PKG_KUBERNETES = "kubernetes"
	PKG_CNI        = "kubernetes-cni"
	PKG_ETCD       = "etcd"
)

func NewFile(
	base string,
	dest string,
) Transfer {
	return Transfer{
		Base:  base,
		Cache: dest,
	}
}

type File struct {
	Path        PathInfo
	Pkg         string
	Version     string
	Ftype       string
	InstallPath string
}

func (f *File) RemoteLocation() string {
	//return path.Join(f.Path.BaseURL(), f.Pkg, f.Key())
	return fmt.Sprintf("%s/%s/%s", f.Path.BaseURL(), f.Pkg, f.Name())
}

func (f *File) String() string {
	return fmt.Sprintf("from[%s] to[%s]", f.RemoteLocation(), f.CacheDir())
}

func (f *File) Ensure(ctx context.Context) error {
	err := os.MkdirAll(f.CacheDir(), 0755)
	if err != nil {
		return fmt.Errorf("enusre dire %s : %s", f.CacheDir(), err.Error())
	}
	var untarErr error
	exist, err := fileExist(f.CachedLocation())
	if err != nil {
		exist = false
		klog.Errorf("find cache file[%s] error, continue download: %s", f.CachedLocation(), err.Error())
	} else {
		untarErr = f.Untar()
	}
	if untarErr != nil || !exist || !v1.G.Cache {
		switch f.Path.OSRelease {
		case host.Ubuntu, host.CentOS:
			args := []string{
				"--tries",
				"10",
				"--no-check-certificate",
				"-q",
				"-O",
				f.CachedLocation(),
				f.RemoteLocation(),
			}
			cm := cmd.NewCmd("wget", args...)
			result := <-cm.Start()
			err = cmd.CmdError(result)
			if err != nil {
				return errors.Wrapf(err, "download [%s] failed", f.Pkg)
			}
		default:
			return fmt.Errorf("unknown os: [%s]", f.Path.OSRelease)
		}
		err = f.Untar()
		if err != nil {
			return errors.Wrapf(err, "untar [%s]", f.CachedLocation())
		}
	}
	if exist {
		klog.Infof("cached file [%s] found, do not download", f.CachedLocation())
	}
	switch f.Ftype {
	case FILE_BINARY:
		err = f.doBinary()
		if err != nil {
			return err
		}
	case FILE_PKG:
		err := f.doPkg()
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown pkg type: [%s], [%s]", f.Ftype, f)
	}
	return nil
}

func (f *File) Purge(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (f *File) CleanUp(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (f *File) Tar() error {
	return tar.Tarinate([]string{}, "")
}

func (f *File) Untar() error {
	return tar.UnTarinate(f.ExtractDir(), f.CachedLocation())
}

func (f *File) doPkg() error {

	rpm := filepath.Join(f.ExtractDir(), f.pkgClass())
	info, err := os.ReadDir(rpm)
	if err != nil {
		return err
	}
	var (
		bin  = "yum"
		args = []string{
			"localinstall", "-y", "--skip-broken",
		}
	)
	switch f.Path.OSRelease {
	case host.Ubuntu:
		bin = "apt"
		args = []string{"install", "-y", "--allow-downgrades"}
	default:
	}
	for _, i := range info {
		if i.IsDir() {
			continue
		}
		args = append(args, filepath.Join(rpm, i.Name()))
	}

	extract := <-cmd.NewCmd(bin, args...).Start()
	return cmd.CmdError(extract)
}

func (f *File) doBinary() error {

	byDir := func(pkg string, dest string) error {

		dir := filepath.Join(f.ExtractDir(), pkg)
		dirs, err := os.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("list file error: %s, %s", dir, err.Error())
		}
		for _, v := range dirs {
			if v.IsDir() {
				continue
			}
			mpath := filepath.Join(dir, v.Name())
			status := <-cmd.NewCmd(
				"chmod",
				"+x", mpath,
			).Start()
			if err := cmd.CmdError(status); err != nil {
				return err
			}
			if dest == "" {
				dest = "/usr/local/bin"
			}
			if err := os.MkdirAll(dest, 0755); err != nil {
				return fmt.Errorf("enusre destination dir %s : %s", dest, err.Error())
			}
			err := os.Rename(
				filepath.Join(mpath),
				filepath.Join(dest, v.Name()),
			)
			if err != nil {
				return fmt.Errorf("mv file error: %s", err.Error())
			}
		}
		return nil
	}

	_, err := os.Stat(filepath.Join(f.ExtractDir(), "cni"))
	if err == nil {
		err = byDir("cni", "/opt/cni/bin")
		if err != nil {
			return errors.Wrapf(err, "install binary file: [%s]", f.Pkg)
		}
	} else {
		klog.Infof("cni dir not exist: %s. %t", err.Error(), os.IsNotExist(err))
	}
	return byDir("bin", "")
}

func (f *File) CacheDir() string {
	return filepath.Join(f.Path.CacheDir, f.Pkg)
}

func (f *File) CachedLocation() string {
	return path.Join(f.CacheDir(), f.Name())
}

func (f *File) pkgClass() string {
	switch f.Ftype {
	case FILE_BINARY:
		switch f.Path.OSRelease {
		case host.Ubuntu, host.CentOS:
			return "elf"
		case host.Windows:
			return "exe"
		}
	case FILE_PKG:
		switch f.Path.OSRelease {
		case host.Ubuntu:
			return "deb"
		case host.CentOS:
			return "rpm"
		case host.Windows:
			return "winpkg"
		}
	}
	return "unknown.format"
}

func (f *File) Name() string {
	name := []string{
		f.Pkg,
		f.Version,
		f.pkgClass(),
		f.Path.Arch,
	}
	return fmt.Sprintf("%s.tar", strings.Join(name, "_"))
}

func (f *File) ExtractDir() string {
	return filepath.Join(f.CacheDir(), "extract", f.Pkg)
}

func fileExist(filename string) (bool, error) {
	info, err := os.Stat(filename)
	if err != nil {

		// Checking if the given file exists or not
		// Using IsNotExist() function
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return info.Mode().IsRegular(), nil
}

type PathInfo struct {
	InnerAddr  bool
	Bucket     string
	Region     string
	BaseServer string
	CacheDir   string

	Project   string
	Namespace string
	// CloudType private public
	CloudType string
	Arch      string
	OSRelease string
}

func (k *PathInfo) setDefault() error {
	if k.Bucket == "" {
		k.Bucket = "host-wdrip"
	}
	if k.Project == "" {
		k.Project = "meridian"
	}
	if k.CloudType == "" {
		k.CloudType = "public"
	}
	if k.Namespace == "" {
		k.Namespace = "default"
	}
	if k.CacheDir == "" {
		home, err := model.MdHOME()
		if err != nil {
			home = os.TempDir()
		}
		k.CacheDir = path.Join(home, ".cache/meridian/download")
	}
	return nil
}

func (k *PathInfo) Validate() error {
	if err := k.setDefault(); err != nil {
		return errors.Wrap(err, "set default pkginfo")
	}
	return nil
}

func (k *PathInfo) BaseURL() string {
	if k.BaseServer == "" {
		inner := k.Region
		if k.InnerAddr {
			inner = fmt.Sprintf("%s-internal", k.Region)
		}
		k.BaseServer = fmt.Sprintf("http://%s-%s.oss-%s.aliyuncs.com", k.Bucket, k.Region, inner)
	}
	var paths []string
	paths = append(paths, k.BaseServer, k.Project)
	if k.Namespace != "" {
		paths = append(paths, k.Namespace)
	}
	paths = append(paths, k.CloudType)
	return strings.Join(paths, "/")
}
