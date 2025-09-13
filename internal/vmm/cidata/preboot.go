package cidata

import (
	"embed"
	"fmt"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/vmm/meta"
	"github.com/aoxn/meridian/internal/vmm/sshutil"
	"github.com/pkg/errors"
	"io"
	"k8s.io/klog/v2"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

//go:embed preboot.tpl.d
var pbFS embed.FS

const pbFSRoot = "preboot.tpl.d"

func NewPreBoot(i *meta.Machine, sshmgr *sshutil.SSHMgr) BootDisk {
	return &PreBoot{ii: i, sshMgr: sshmgr}
}

type PreBoot struct {
	sshMgr *sshutil.SSHMgr
	ii     *meta.Machine
}

func (i *PreBoot) CreateBootDisk() error {
	pub, err := i.sshMgr.LoadPubKey()
	if err != nil {
		return errors.Wrap(err, "load public key for vm")
	}
	tplModel, err := NewTpl(i.ii, pub)
	if err != nil {
		return errors.Wrapf(err, "build init disk template")
	}

	layout, err := tplModel.Build(&pbFS, pbFSRoot, ValidateTemplateArgs)
	if err != nil {
		return err
	}

	bin, err := i.buildGuestBin()
	if err != nil {
		return errors.Wrap(err, "failed to build guest binary reader")
	}
	layout = append(layout, bin)

	var instDir = i.ii.Dir()
	_ = ensurePath(instDir, true)
	klog.Infof("make boot disk file Path: %s", filepath.Join(instDir, v1.CIDataDMG))
	return makeBootDisk(i.ii.Name, filepath.Join(instDir, v1.CIDataDMG), layout)
}

func (i *PreBoot) buildGuestBin() (*Entry, error) {
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
		Path:   path.Join("usr", "local", "bin", "md-guest"),
	}
	return entry, nil
}

const (
	volName = "Preboot"
)

func makeBootDisk(name, at string, layout []*Entry) error {
	var mountPath = path.Join("/Volumes", volName)

	err := os.MkdirAll(mountPath, 0755)
	if err != nil {
		klog.Errorf("mount boot disk dmg: mkdir %s failed, %v", mountPath, err)
	}
	if !exist(at) {
		var args = []string{
			"create",
			"-size", "100m",
			"-layout", "GPTSPUD",
			"-fs", "APFS",
			"-volname", volName, at,
		}
		cmd := exec.Command("hdiutil", args...)
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		err := cmd.Run()
		if err != nil {
			return errors.Wrapf(err, "create empty boot disk")
		}
	}
	// attach
	err = attach(at, mountPath)
	if err != nil {
		return errors.Wrapf(err, "attach boot disk")
	}
	defer func() {
		err = detach(mountPath)
		if err != nil {
			klog.Errorf("dettach boot disk[%s]: %v", mountPath, err)
		}
	}()
	return writeContent(mountPath, layout)
}

func writeContent(dst string, layout []*Entry) error {
	// write to fs
	for _, f := range layout {
		var (
			dir    = path.Join(dst, path.Dir(f.Path))
			target = path.Join(dst, f.Path)
		)

		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return errors.Wrapf(err, "mkdir %s", dir)
		}
		data, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, 0755)
		if err != nil {
			return errors.Wrapf(err, "init bootdisk, open %s", target)
		}
		defer data.Close()
		_, err = io.Copy(data, f.reader)
		if err != nil {
			return errors.Wrapf(err, "init bootdisk, copy")
		}
		if f.closer != nil {
			_ = f.closer.Close()
		}
		klog.V(6).Infof("debug generate bootdisk: write iso part, %s, %s", dst, f.Path)
	}
	return nil
}

func exist(at string) bool {
	_, err := os.Stat(at)
	return err == nil
}

func attach(src string, dst string) error {
	if exist(dst) {
		return nil
	}
	args := []string{
		"attach", src,
		"-mountpoint", dst,
	}
	klog.Infof("run command: hdiutil %s", strings.Join(args, " "))
	return exec.Command("hdiutil", args...).Run()
}

func detach(dst string) error {
	args := []string{
		"detach", dst,
	}
	klog.Infof("run command: hdiutil %s", strings.Join(args, " "))
	return exec.Command("hdiutil", args...).Run()
}
