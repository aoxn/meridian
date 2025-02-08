package model

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aoxn/meridian/api/v1"
	"github.com/lima-vm/go-qcow2reader"
	"io/fs"
	"k8s.io/klog/v2"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"text/template"
	"time"

	"github.com/docker/go-units"
)

type Status = string

const (
	StatusUnknown       Status = ""
	StatusUninitialized Status = "Uninitialized"
	StatusInstalling    Status = "Installing"
	StatusBroken        Status = "Broken"
	StatusStopped       Status = "Stopped"
	StatusRunning       Status = "Running"
)

//
//type Instance struct {
//	Key            string            `json:"name"`
//	Status          Status            `json:"status"`
//	Dir             string            `json:"dir"`
//	VMType          v1.VMType         `json:"vmType"`
//	Arch            v1.Arch           `json:"arch"`
//	CPUType         string            `json:"cpuType"`
//	CPUs            int               `json:"cpus,omitempty"`
//	Memory          int64             `json:"memory,omitempty"` // bytes
//	Disk            int64             `json:"disk,omitempty"`   // bytes
//	Message         string            `json:"message,omitempty"`
//	AdditionalDisks []v1.Disk         `json:"additionalDisks,omitempty"`
//	Networks        []v1.Network      `json:"network,omitempty"`
//	SSHLocalPort    int               `json:"sshLocalPort,omitempty"`
//	SSHConfigFile   string            `json:"sshConfigFile,omitempty"`
//	HostAgentPID    int               `json:"hostAgentPID,omitempty"`
//	DriverPID       int               `json:"driverPID,omitempty"`
//	Errors          []error           `json:"errors,omitempty"`
//	Y               *v1.GuestInfo  `json:"config,omitempty"`
//	SSHAddress      string            `json:"sshAddress,omitempty"`
//	Protected       bool              `json:"protected"`
//	LimaVersion     string            `json:"limaVersion"`
//	Param           map[string]string `json:"param,omitempty"`
//}

type PidCondition struct {
	Name  string    `json:"name"`
	PID   string    `json:"pid"`
	Stamp time.Time `json:"stamp"`
}

func (p PidCondition) String() string {
	return fmt.Sprintf("%s:%s, %s", p.Name, p.PID, p.Stamp)
}

type Instance struct {
	Dir string `json:"dir"`

	CPUs    int    `json:"cpus,omitempty"`
	Memory  int64  `json:"memory,omitempty"` // bytes
	Disk    int64  `json:"disk,omitempty"`   // bytes
	Message string `json:"message,omitempty"`
	//AdditionalDisks []v1.Disk         `json:"additionalDisks,omitempty"`
	//Networks        []v1.Network      `json:"network,omitempty"`
	SSHConfigFile      string            `json:"sshConfigFile,omitempty"`
	HostAgentPID       int               `json:"hostAgentPID,omitempty"`
	DriverPID          int               `json:"driverPID,omitempty"`
	Errors             []error           `json:"errors,omitempty"`
	SSHAddress         string            `json:"sshAddress,omitempty"`
	Protected          bool              `json:"protected"`
	Param              map[string]string `json:"param,omitempty"`
	*v1.VirtualMachine `json:"virtualMachine,omitempty"`
}

func (i *Instance) HealthyTick(ctx context.Context) {
	tick := time.NewTicker(15 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			_ = i.SavePID()
		}
	}
}

func (i *Instance) SavePID() error {
	pidFile := i.PIDFile()
	_, err := os.Stat(pidFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		klog.Errorf("stat pidfile %q : %v", pidFile, err)
	}
	pid := strconv.Itoa(os.Getpid())
	data, _ := json.MarshalIndent(PidCondition{
		Name:  i.Name,
		PID:   pid,
		Stamp: time.Now(),
	}, "", "    ")
	return os.WriteFile(pidFile, data, 0o644)
}

func (i *Instance) LoadPID() (PidCondition, error) {
	pidFile := i.PIDFile()
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return PidCondition{}, err
	}
	pid := PidCondition{}
	err = json.Unmarshal(data, &pid)
	if err != nil {
		return PidCondition{}, err
	}
	id, err := strconv.Atoi(pid.PID)
	if err != nil {
		return PidCondition{}, err
	}
	_, err = os.FindProcess(id)
	if err != nil {
		_ = os.Remove(pidFile)
		klog.Infof("find vm process [%d] failed: %v", id, err)
		return PidCondition{}, nil
	}
	return pid, nil
}

func (i *Instance) RemovePID() {
	pidFile := i.PIDFile()
	_ = os.RemoveAll(pidFile)
	klog.Infof("removing pid file %s", pidFile)
}

func (i *Instance) PIDFile() string {
	return filepath.Join(i.Dir, v1.HostAgentPID)
}

func (i *Instance) Vm() *v1.VirtualMachineSpec {
	return &i.Spec
}

func NewVMConfig(i string, yaml []byte) error {
	return v1.EnsureYAML(i, yaml, false)
}

// NewInstance returns err only when the instance does not exist (os.ErrNotExist).
// Other errors are returned as *Instance.Errors.
func NewInstance(vm *v1.VirtualMachine) (*Instance, error) {
	// InstanceDir validates the instName but does not check whether the instance exists
	instDir, err := InstanceDir(vm.Name)
	if err != nil {
		return nil, err
	}
	// Make sure inst.Dir is set, even when YAML validation fails
	inst := &Instance{
		Dir:            instDir,
		VirtualMachine: vm,
	}
	inst.SSHAddress = "127.0.0.1"
	inst.SSHConfigFile = filepath.Join(instDir, v1.SSHConfig)
	inst.HostAgentPID, err = ReadPIDFile(inst.PIDFile())
	if err != nil {
		inst.Status.Phase = StatusBroken
		inst.Errors = append(inst.Errors, err)
	}
	if len(inst.Vm().Networks) == 0 {
		network := []v1.Network{
			{
				VZNAT:      true,
				Interface:  "enp0s1",
				MACAddress: v1.GenMAC(),
			},
		}
		inst.Vm().Networks = network
	}
	memory, err := units.RAMInBytes(inst.Vm().Memory)
	if err == nil {
		inst.Memory = memory
	}
	disk, err := units.RAMInBytes(inst.Vm().Disk)
	if err == nil {
		inst.Disk = disk
	}
	// 0 out values since not configurable on WSL2
	if inst.Vm().VMType == v1.WSL2 {
		inst.Memory = 0
		inst.CPUs = 0
		inst.Disk = 0
	}

	protected := filepath.Join(instDir, v1.Protected)
	if _, err := os.Lstat(protected); !errors.Is(err, os.ErrNotExist) {
		inst.Protected = true
	}

	pid, err := inst.LoadPID()
	if err != nil {
		inst.Status.Phase = StatusBroken
		inst.Message = err.Error()
	} else {
		id, err := strconv.Atoi(pid.PID)
		if err != nil {
			inst.Status.Phase = StatusBroken
			inst.Message = err.Error()
		} else {
			_, err := os.FindProcess(id)
			if err != nil {
				inst.Status.Phase = StatusBroken
				inst.Message = err.Error()
			} else {
				inst.Status.Phase = StatusRunning
				inst.Message = fmt.Sprintf("VM running with pid: %s", pid)
			}
		}
	}
	return inst, nil
}

// Protect protects the instance to prohibit accidental removal.
// Protect does not return an error even when the instance is already protected.
func (i *Instance) Protect() error {
	protected := filepath.Join(i.Dir, v1.Protected)
	// TODO: Do an equivalent of `chmod +a "everyone deny delete,delete_child,file_inherit,directory_inherit"`
	// https://github.com/lima-vm/lima/issues/1595
	if err := os.WriteFile(protected, nil, 0o400); err != nil {
		return err
	}
	i.Protected = true
	return nil
}

func (i *Instance) Stop() error {
	klog.Infof("stop instance: [%s]", i.Name)
	pid, err := i.LoadPID()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if pid.PID == "" {
		return os.Remove(i.PIDFile())
	}
	id, err := strconv.Atoi(pid.PID)
	if err != nil {
		return err
	}
	proc, err := os.FindProcess(id)
	if err != nil {
		return err
	}
	err = proc.Signal(os.Interrupt)
	if err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			_ = os.Remove(i.PIDFile())
			return nil
		}
		// We may not have permission to send the signal (e.g. to network daemon running as root).
		// But if we get a permissions error, it means the process is still running.
		if !errors.Is(err, os.ErrPermission) {
			return err
		}
	}
	// todo: wait process exit;
	return nil
}

func (i *Instance) Destroy() error {
	err := i.Stop()
	if err != nil {
		return err
	}
	klog.Infof("destroy instance: [%s/%s]", i.Dir, i.Name)
	return os.RemoveAll(i.Dir)
}

// Unprotect unprotects the instance.
// Unprotect does not return an error even when the instance is already unprotected.
func (i *Instance) Unprotect() error {
	protected := filepath.Join(i.Dir, v1.Protected)
	if err := os.RemoveAll(protected); err != nil {
		return err
	}
	i.Protected = false
	return nil
}

func (i *Instance) InspectDisk(diskName string) (*MountDisk, error) {
	disk := &MountDisk{
		Name: diskName,
	}

	diskDir, err := DiskDir(diskName)
	if err != nil {
		return nil, err
	}

	disk.Dir = diskDir
	dataDisk := filepath.Join(diskDir, v1.DataDisk)
	if _, err := os.Stat(dataDisk); err != nil {
		return nil, err
	}

	disk.Size, disk.Format, err = inspectDisk(dataDisk)
	if err != nil {
		return nil, err
	}

	instDir, err := os.Readlink(filepath.Join(diskDir, v1.InUseBy))
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	} else {
		disk.Instance = filepath.Base(instDir)
		disk.InstanceDir = instDir
	}

	disk.MountPoint = fmt.Sprintf("/mnt/lima-%s", diskName)

	return disk, nil
}

// inspectDisk attempts to inspect the disk size and format by itself,
// and falls back to inspectDiskWithQemuImg on an error.
func inspectDisk(fName string) (size int64, format string, _ error) {
	f, err := os.Open(fName)
	if err != nil {
		return inspectDiskWithQemuImg(fName)
	}
	defer f.Close()
	img, err := qcow2reader.Open(f)
	if err != nil {
		return inspectDiskWithQemuImg(fName)
	}
	sz := img.Size()
	if sz < 0 {
		return inspectDiskWithQemuImg(fName)
	}

	return sz, string(img.Type()), nil
}

// inspectDiskSizeWithQemuImg invokes `qemu-img` binary to inspect the disk size and format.
func inspectDiskWithQemuImg(fName string) (size int64, format string, _ error) {

	return -1, "", fmt.Errorf("unimplemented")
}

type FormatData struct {
	Instance
	HostOS       string
	HostArch     string
	LimaHome     string
	IdentityFile string
}

func AddGlobalFields(inst *Instance) (FormatData, error) {
	var data FormatData
	data.Instance = *inst
	// Add HostOS
	data.HostOS = runtime.GOOS
	// Add HostArch
	data.HostArch = string(v1.NewArch(runtime.GOARCH))
	// Add IdentityFile
	configDir, err := MdConfigDir()
	if err != nil {
		return FormatData{}, err
	}
	data.IdentityFile = filepath.Join(configDir, v1.UserPrivateKey)
	// Add LimaHome
	data.LimaHome, err = MdHOME()
	if err != nil {
		return FormatData{}, err
	}
	return data, nil
}

type MountDisk struct {
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	Format      string `json:"format"`
	Dir         string `json:"dir"`
	Instance    string `json:"instance"`
	InstanceDir string `json:"instanceDir"`
	MountPoint  string `json:"mountPoint"`
}

func (d *MountDisk) Lock(instanceDir string) error {
	inUseBy := filepath.Join(d.Dir, v1.InUseBy)
	return os.Symlink(instanceDir, inUseBy)
}

func (d *MountDisk) Unlock() error {
	inUseBy := filepath.Join(d.Dir, v1.InUseBy)
	return os.Remove(inUseBy)
}

func inspectStatusWithPIDFiles(instDir string, inst *Instance, y *v1.VirtualMachine) {
	var err error
	inst.DriverPID, err = ReadPIDFile(inst.PIDFile())
	if err != nil {
		inst.Status.Phase = StatusBroken
		inst.Errors = append(inst.Errors, err)
	}

	if inst.Status.Phase == StatusUnknown {
		switch {
		case inst.HostAgentPID > 0 && inst.DriverPID > 0:
			inst.Status.Phase = StatusRunning
		case inst.HostAgentPID == 0 && inst.DriverPID == 0:
			inst.Status.Phase = StatusStopped
		case inst.HostAgentPID > 0 && inst.DriverPID == 0:
			inst.Errors = append(inst.Errors, errors.New("host agent is running but driver is not"))
			inst.Status.Phase = StatusBroken
		default:
			inst.Errors = append(inst.Errors, fmt.Errorf("%s driver is running but host agent is not", inst.Vm().VMType))
			inst.Status.Phase = StatusBroken
		}
	}
}

// ReadPIDFile returns 0 if the PID file does not exist or the process has already terminated
// (in which case the PID file will be removed).
func ReadPIDFile(path string) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return 0, err
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return 0, err
	}
	// os.FindProcess will only return running processes on Windows, exit early
	if runtime.GOOS == "windows" {
		return pid, nil
	}
	err = proc.Signal(syscall.Signal(0))
	if err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			_ = os.Remove(path)
			return 0, nil
		}
		// We may not have permission to send the signal (e.g. to network daemon running as root).
		// But if we get a permissions error, it means the process is still running.
		if !errors.Is(err, os.ErrPermission) {
			return 0, err
		}
	}
	return pid, nil
}

func executeGuestTemplate(format, instDir string, param map[string]string) (bytes.Buffer, error) {
	tmpl, err := template.New("").Parse(format)
	if err == nil {
		user, _ := MdUser(false)
		data := map[string]interface{}{
			"Home":  fmt.Sprintf("/home/%s.linux", user.Username),
			"Key":   filepath.Base(instDir),
			"UID":   user.Uid,
			"User":  user.Username,
			"Param": param,
		}
		var out bytes.Buffer
		if err := tmpl.Execute(&out, data); err == nil {
			return out, nil
		}
	}
	return bytes.Buffer{}, err
}

func executeHostTemplate(format, instDir string, param map[string]string) (bytes.Buffer, error) {
	tmpl, err := template.New("").Parse(format)
	if err == nil {
		user, _ := MdUser(false)
		home, _ := MdHOME()
		limaHome, _ := MdHOME()
		data := map[string]interface{}{
			"Dir":   instDir,
			"Home":  home,
			"Key":   filepath.Base(instDir),
			"UID":   user.Uid,
			"User":  user.Username,
			"Param": param,

			"ii":       filepath.Base(instDir), // DEPRECATED, use `{{.Key}}`
			"LimaHome": limaHome,               // DEPRECATED, use `{{.Dir}}` instead of `{{.LimaHome}}/{{.ii}}`
		}
		var out bytes.Buffer
		if err := tmpl.Execute(&out, data); err == nil {
			return out, nil
		}
	}
	return bytes.Buffer{}, err
}
