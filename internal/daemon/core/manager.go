package core

import (
	"fmt"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/tool"
	"github.com/aoxn/meridian/internal/vmm/meta"
	"github.com/aoxn/meridian/internal/vmm/model"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type Context struct {
	vmMgr    *LocalVMMgr
	imageMgr *LocalImageMgr
}

func (ctx *Context) VMMgr() *LocalVMMgr {
	return ctx.vmMgr
}

func (ctx *Context) ImageMgr() *LocalImageMgr {
	return ctx.imageMgr
}

type LocalVMMgr struct {
	backend  meta.Backend
	stateMgr *vmStateMgr
}

func (mgr *LocalVMMgr) Run(name string) error {
	vm, err := mgr.backend.Machine().Get(name)
	if err != nil {
		return errors.Wrap(err, "read machine config error")
	}
	state := mgr.stateMgr.Get(name)
	switch state {
	case Running, Stopping, Starting:
		return fmt.Errorf("UnexpectedCurrentState: %s=[%s]", name, state)
	default:
	}
}

func (mgr *LocalVMMgr) Stop(name string) error {
	panic("not implement")
}

func (mgr *LocalVMMgr) Start(name string) error {
	panic("not implement")
}

func (mgr *LocalVMMgr) Destroy(name string) error {
	panic("not implement")
}

func (mgr *LocalVMMgr) RunCommand(name string, cmd Command) error {
	panic("not implement")
}

type LocalImageMgr struct {
}

func (img *LocalImageMgr) Pull(name string) error {
	panic("not implement")
}

type vmStateMgr struct {
	vms map[string]*vmState
}

type vmState struct {
	name    string
	state   string
	machine *meta.Machine
}

const (
	Unknown  = "UnKnown"
	Running  = "Running"
	Stopping = "Stopping"
	Stopped  = "Stopped"
	Starting = "Starting"
)

func (mgr *vmStateMgr) Get(name string) *vmState {
	vm, ok := mgr.vms[name]
	if ok {
		return vm
	}
	return &vmState{name: name, state: Unknown}
}

func (vm *vmState) runVm() error {
	inst, err := model.NewInstance(vm.machine.Spec)
	if err != nil {
		return err
	}
	begin := time.Now() // used for logrus propagation

	pid, err := inst.LoadPID()
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		// not exist
		klog.Infof("[%-10s]pid file not exist: [%s], %s", vm.Name, pid.PID, err.Error())
	} else {
		klog.Infof("[%-10s]got existing pid [%s]", vm.Name, pid.PID)
	}
	if pid.PID == "" || time.Now().After(pid.Stamp.Add(30*time.Second)) {
		klog.Infof("[%-10s]pid not found or expired: [%s]", vm.Name, pid)
		_ = os.MkdirAll(inst.Dir, 0o700)
		vmBin, err := vmBinaryPath()
		if err != nil {
			return err
		}
		klog.Infof("boot vm from: %s", vmBin)
		haStdoutPath := filepath.Join(inst.Dir, v1.HostAgentStdoutLog)
		haStderrPath := filepath.Join(inst.Dir, v1.HostAgentStderrLog)
		if err := os.RemoveAll(haStdoutPath); err != nil {
			return err
		}
		if err := os.RemoveAll(haStderrPath); err != nil {
			return err
		}
		haStdoutW, err := os.Create(haStdoutPath)
		if err != nil {
			return err
		}
		// no defer haStdoutW.Close()
		haStderrW, err := os.Create(haStderrPath)
		if err != nil {
			return err
		}
		// no defer haStderrW.Close()
		var args = []string{"start"}
		haCmd := exec.CommandContext(ctx, vmBin, args...)

		haCmd.Stdin = strings.NewReader(tool.PrettyYaml(vm))
		haCmd.Stdout = haStdoutW
		haCmd.Stderr = haStderrW
		haCmd.SysProcAttr = &syscall.SysProcAttr{
			Setsid: true,
		}

		if err := haCmd.Start(); err != nil {
			return err
		}
		klog.Infof("[%-10s]vm started: [%s], %s", vm.Name, pid.PID, strings.Join(append([]string{vmBin}, args...), " "))
	} else {
		klog.Infof("[%-10s]virtual machine already started: [%s]", vm.Name, pid.PID)
	}
	klog.Infof("[%-10s]vm started in %f(s)", vm.Name, time.Now().Sub(begin).Seconds())
	merdiand, err := client.Client(endpoint(vm.Name))
	if err != nil {
		return err
	}
	if err := m.WaitHostAgentStart(ctx, merdiand, vm); err != nil {
		return errors.Wrapf(err, "wait host agent: %s", vm.Name)
	}
	_, err = m.Store.Update(ctx, vm, &metav1.UpdateOptions{})

	if err = m.EnsureKubernetes(ctx, merdiand, vm); err != nil {
		return errors.Wrapf(err, "ensure kubernetes: %s", vm.Name)
	}

	if err = m.EnsureDockerContext(vm); err != nil {
		return errors.Wrapf(err, "ensure docker context: %s", vm.Name)
	}
	_, err = m.Store.Update(ctx, vm, &metav1.UpdateOptions{})
	return err
}

type Command struct {
}
