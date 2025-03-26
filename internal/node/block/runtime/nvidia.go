package runtime

import (
	"context"
	"fmt"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/node/block"
	"github.com/aoxn/meridian/internal/node/block/file"
	"github.com/aoxn/meridian/internal/node/host"
	"github.com/aoxn/meridian/internal/tool/nvidia"
	apt "github.com/arduino/go-apt-client"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"
	"os"
)

type nvidiaBlock struct {
	req  *v1.Request
	host host.Host
	file *file.File
}

func NewNvidiaBlock(req *v1.Request, host host.Host) (block.Block, error) {

	info := file.PathInfo{
		InnerAddr: false,
		Arch:      host.Arch(),
		OSRelease: host.OS(),
		Region:    host.Region(),
	}
	err := info.Validate()
	if err != nil {
		return nil, err
	}
	return &nvidiaBlock{
		req:  req,
		host: host,
		file: &file.File{
			Path:    info,
			Pkg:     file.PKG_NVIDIA_TOOLKIT,
			Ftype:   file.FILE_PKG,
			Version: req.Spec.Config.Runtime.NvidiaToolKitVersion,
		},
	}, nil
}

// Ensure runs the action
func (a *nvidiaBlock) Ensure(ctx context.Context) error {
	if has, err := nvidia.HasNvidiaDevice(); err != nil || !has {
		klog.Infof("find nvidia with error: %v", err)
		return nil
	}
	if err := a.file.Ensure(ctx); err != nil {
		return errors.Wrapf(err, "install containerd runtime: %s", a.req.Name)
	}

	msg, err := a.host.Command("nvidia-ctk", "runtime", "configure", "--runtime=containerd", "--set-as-default")
	if err != nil {
		return errors.Wrapf(err, "config nvidia containerd runtime: %s, %s", a.req.Name, msg)
	}

	return a.host.Service().Restart("containerd")
}

func (a *nvidiaBlock) Purge(ctx context.Context) error {
	switch a.file.Path.OSRelease {
	case host.Ubuntu:
		var pkg []*apt.Package
		for _, i := range []string{
			"libnvidia-container1",
			"libnvidia-container-tools",
			"nvidia-container-toolkit-base",
			"nvidia-container-toolkit",
		} {
			found, err := apt.Search(i)
			if err != nil {
				klog.Warningf("find package[%s] with error: %s", i, err.Error())
				continue
			}
			if len(found) <= 0 {
				continue
			}
			for _, p := range found {
				klog.Infof("meridian found package: [name=%s], [version=%s]", p.Name, p.Version)
			}
			pkg = append(pkg, &apt.Package{Name: i})
		}
		if len(pkg) > 0 {
			data, err := apt.Remove(pkg...)
			if err != nil {
				return errors.Wrapf(err, "remove package: %s", toPkg(pkg))
			}
			klog.Infof("do remove pkg: %s, %s", toPkg(pkg), data)
		}
	default:
		return fmt.Errorf("unimplemented os [%s] for uninstall pkg", a.file.Path.OSRelease)
	}

	files := []string{
		"/etc/nvidia-container-runtime/",
	}
	files = append(files, keys(cfgs)...)

	for _, r := range files {
		klog.Infof("remove nvidia config file: [%s]", r)
		err := os.RemoveAll(r)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *nvidiaBlock) CleanUp(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (a *nvidiaBlock) Name() string {
	return fmt.Sprintf("nvidia container environment init [%s]", a.host.NodeID())
}
