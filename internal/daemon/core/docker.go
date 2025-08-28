package core

import (
	"context"
	"fmt"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/client"
	"github.com/aoxn/meridian/internal/tool/cmd"
	"github.com/aoxn/meridian/internal/tool/downloader"
	"github.com/aoxn/meridian/internal/vmm/meta"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog/v2"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func NewLocalDockerMgr(stateMgr *vmStateMgr) (*LocalDockerMgr, error) {
	return &LocalDockerMgr{
		stateMgr: stateMgr,
	}, nil
}

type LocalDockerMgr struct {
	tskMgr   *taskMgr
	stateMgr *vmStateMgr
	//sshMgr   *sshutil.SSHMgr
}

func (mgr *LocalDockerMgr) Create(ctx context.Context, at string) error {
	vm := mgr.stateMgr.Get(at)
	if vm.machine == nil {
		return fmt.Errorf("vm %s not found", at)
	}
	l := mgr.stateMgr.meta.Docker()
	_, err := l.Get(at)
	if err == nil {
		return fmt.Errorf("docker already exists %s", at)
	}
	var (
		version  = "1.6.28"
		registry = "registry.cn-hangzhou.aliyuncs.com"
	)
	out, err := vm.SSH().RunCommand(at, getCmd(ActionInstall, version, registry))
	if err != nil {
		return errors.Wrap(err, "run install docker command")
	}
	klog.Infof("install command result: %s", string(out))
	err = mgr.setDockerContext(vm.machine)
	if err != nil {
		return errors.Wrapf(err, "set docker context")
	}
	err = mgr.ForwardDocker(ctx, vm)
	if err != nil {
		return errors.Wrapf(err, "forward docker to host")
	}
	return l.Create(&meta.Docker{
		Name: at, VmName: at, Version: version, State: "Installed",
	})
}

func (mgr *LocalDockerMgr) Destroy(ctx context.Context, at string) error {
	vm := mgr.stateMgr.Get(at)
	if vm.machine == nil {
		klog.Infof("vm %s not found", at)
		return nil
	}
	l := mgr.stateMgr.meta.Docker()
	_, err := l.Get(at)
	if err != nil {
		klog.Infof("docker not found: %s", at)
		return nil
	}
	var (
		version  = ""
		registry = ""
	)

	_ = mgr.removeDockerContext(at)
	out, err := vm.SSH().RunCommand(at, getCmd(ActionDestroy, version, registry))
	if err != nil {
		klog.Infof("command result: %s", string(out))
		return errors.Wrap(err, "run destroy docker command")
	}
	return mgr.UnForwardDocker(ctx, vm)
}

func (mgr *LocalDockerMgr) ForwardDocker(ctx context.Context, vm *vmState) error {

	fwd := newDockerForward(vm.machine)

	vm.machine.Spec.SetForward(fwd...)

	err := vm.meta.Machine().Update(vm.machine)
	if err != nil {
		return errors.Wrapf(err, "update machine metadata")
	}

	sdbx, err := client.Client(vm.machine.SandboxSock())
	if err != nil {
		return errors.Wrapf(err, "get client sandbox sdbx")
	}
	return sdbx.Create(ctx, "forward", "docker", &fwd)
}

func (mgr *LocalDockerMgr) UnForwardDocker(ctx context.Context, vm *vmState) error {

	fwd := newDockerForward(vm.machine)

	vm.machine.Spec.RemoveForward(fwd...)

	err := vm.meta.Machine().Update(vm.machine)
	if err != nil {
		return errors.Wrapf(err, "update machine metadata")
	}

	sdbx, err := client.Client(vm.machine.SandboxSock())
	if err != nil {
		return errors.Wrapf(err, "get client sandbox sdbx")
	}
	return sdbx.Delete(ctx, "forward", "docker", fwd)
}

func newDockerForward(vm *meta.Machine) []v1.PortForward {
	return []v1.PortForward{
		{
			SrcProto: "unix",
			SrcAddr:  intstr.FromString(vm.DockerSock()),
			DstProto: "vsock",
			DstAddr:  intstr.FromInt32(10240),
		},
	}
}

func (mgr *LocalDockerMgr) initBinary(ctx context.Context) {
	err := setLocalBinary(ctx, "docker")
	if err != nil {
		klog.Errorf("failed to install docker: %v", err)
	}
	err = setLocalBinary(ctx, "kubectl")
	if err != nil {
		klog.Errorf("failed to install kubectl: %v", err)
	}
}

func (mgr *LocalDockerMgr) setDockerContext(vm *meta.Machine) error {
	docker := "/usr/local/bin/docker"
	content := []string{"context", "inspect", vm.Name}
	r := <-cmd.NewCmd(docker, content...).Start()
	err := cmd.CmdError(r)
	if err != nil {
		content = []string{
			"context", "create", vm.Name,
			"--docker", fmt.Sprintf("host=unix://%s", vm.DockerSock()),
			"--description", "meridian docker endpoint",
		}
		r = <-cmd.NewCmd(docker, content...).Start()
		return cmd.CmdError(r)
	}
	content = []string{
		"context",
		"update",
		vm.Name,
		"--docker", fmt.Sprintf("host=unix://%s", vm.DockerSock()),
	}
	r = <-cmd.NewCmd(docker, content...).Start()
	return cmd.CmdError(r)

}

func (mgr *LocalDockerMgr) removeDockerContext(name string) error {
	content := []string{
		"context",
		"rm", "-f", name,
	}
	r := <-cmd.NewCmd(
		"/usr/local/bin/docker", content...).Start()
	err := cmd.CmdError(r)
	if err != nil {
		klog.Warningf("[%-10s]docker context rm %s: %s", name, name, err.Error())
	}
	return nil
}

func setLocalBinary(ctx context.Context, bin string) error {
	klog.Infof("try to install [%s] binary", bin)
	switch runtime.GOOS {
	case "darwin":
		return setDarwin(ctx, bin)
	case "linux":
	case "windows":
	}
	return fmt.Errorf("unsupported platform")
}

func setDarwin(ctx context.Context, bin string) error {
	var dest = "/usr/local/bin"
	_, err := os.Stat(filepath.Join(dest, bin))
	if err == nil {
		klog.Infof("[%s] found in[%s]", bin, filepath.Join(dest, bin))
		return nil
	}

	f, err := v1.FindBinary(bin, v1.NewArch(runtime.GOARCH))
	if err != nil {
		return errors.Wrapf(err, "find %s runtime location", bin)
	}
	_ = os.MkdirAll(dest, 0775)
	klog.Infof("[%s] not found, install", bin)
	res, err := downloader.Download(ctx, filepath.Join(dest, bin), f.Location,
		downloader.WithCache(),
		downloader.WithDecompress(true),
		downloader.WithDescription("download docker"),
		downloader.WithExpectedDigest(f.Digest),
	)
	if err != nil {
		return errors.Wrapf(err, "download %s failed", f.Location)
	}
	klog.Infof("[%s] download success with status: %s", bin, res.Status)
	return os.Chmod(filepath.Join(dest, bin), 0755)
}

var base = `
#!/bin/bash
set -e
version=0.1.0
OS=$(uname|tr '[:upper:]' '[:lower:]')
arch=$(uname -m|tr '[:upper:]' '[:lower:]')
case $arch in
"amd64")
        arch=amd64
        ;;
"arm64")
        arch=arm64
        ;;
"x86_64")
	arch=amd64
	;;
*)
        echo "unknown arch: ${arch} for ${OS}"; exit 1
        ;;
esac

server=http://host-wdrip-cn-hangzhou.oss-cn-hangzhou.aliyuncs.com

need_install=0
if [[ -f /usr/local/bin/meridian-node ]];
then
        wget -q -O /tmp/meridian-node.${OS}.${arch}.tar.gz.sum \
                $server/bin/${OS}/${arch}/${version}/meridian-node.${OS}.${arch}.tar.gz.sum
        m1=$(cat /tmp/meridian-node.${OS}.${arch}.tar.gz.sum |awk '{print $1}')
        m2=$(md5sum /usr/local/bin/meridian-node |awk '{print $1}')
        if [[ "$m1" == "$m2" ]];
        then
                need_install=0
        else
                need_install=1
        fi
else
        need_install=1
fi

if [[ "$need_install" == "1" ]];
then
        wget -q -O /tmp/meridian-node.${OS}.${arch}.tar.gz \
                $server/bin/${OS}/${arch}/${version}/meridian-node.${OS}.${arch}.tar.gz

        wget -q -O /tmp/meridian-node.${OS}.${arch}.tar.gz.sum \
                $server/bin/${OS}/${arch}/${version}/meridian-node.${OS}.${arch}.tar.gz.sum
        tar xf /tmp/meridian-node.${OS}.${arch}.tar.gz -C /tmp
        sudo mv -f /tmp/bin/meridian-node.${OS}.${arch} /usr/local/bin/meridian-node
        rm -rf /tmp/meridian-node.${OS}.${arch}.tar.gz /tmp/meridian-node.${OS}.${arch}.tar.gz.sum
fi

# /usr/local/bin/meridian-node
`

const (
	ActionInstall = "install"
	ActionDestroy = "destroy"
)

func getCmd(action string, version, registry string) string {
	var command []string
	switch action {
	case ActionInstall:
		command = []string{
			base,
			fmt.Sprintf("sudo /usr/local/bin/meridian-node create docker --version %s --registry %s", version, registry),
		}
	case ActionDestroy:
		command = []string{
			base,
			"sudo /usr/local/bin/meridian-node destroy docker",
		}
	}
	return strings.Join(command, "\n")
}
