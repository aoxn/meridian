package meta

import (
	"encoding/json"
	"fmt"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/containerd/containerd/identifiers"
	"github.com/lima-vm/go-qcow2reader"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog/v2"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
)

type PidCondition struct {
	Name  string    `json:"name"`
	PID   int       `json:"pid"`
	Stamp time.Time `json:"stamp"`
}

func (p PidCondition) String() string {
	return fmt.Sprintf("%s:%d, %s", p.Name, p.PID, p.Stamp)
}

type Machine struct {
	Name       string                 `json:"name"`
	AbsDir     string                 `json:"absDir"`
	Created    metav1.Time            `json:"created"`
	SandboxPID int                    `json:"sandboxPid"`
	Login      string                 `json:"lgoin"`
	Spec       *v1.VirtualMachineSpec `json:"spec"`
	Protected  bool                   `json:"protected"`
	State      string                 `json:"state"`
	Message    string                 `json:"message,omitempty"`
	Address    []string               `json:"address,omitempty"`
	Stage      []Stage                `json:"stage,omitempty"`
}

type Stage struct {
	Phase       string `json:"phase"`
	Description string `json:"description"`
}

type StageUtil struct {
	root string
}

const (
	StagePending      = "pending"
	StageInitializing = "initializing"
	StageInitialized  = "initialized"
)

func (g *StageUtil) Get() string {
	var stage = StagePending
	stageFile := path.Join(g.root, "stage")
	_, err := os.Stat(stageFile)
	if err != nil {
		if os.IsNotExist(err) {
			_ = g.Set(stage)
			return stage
		}
		return stage
	}
	data, err := os.ReadFile(stageFile)
	if err != nil {
		klog.Errorf("failed to read stage file: %v", err)
		return stage
	}
	return string(data)
}

func (g *StageUtil) Initialized() bool {
	return g.Get() == StageInitialized
}

func (g *StageUtil) Set(stage string) error {
	return os.WriteFile(path.Join(g.root, "stage"), []byte(stage), 0755)
}

func (m *Machine) Dir() string {
	return m.AbsDir
}

func (m *Machine) DockerSock() string {
	return path.Join(m.Dir(), "docker.sock")
}

func (m *Machine) SandboxSock() string {
	return path.Join(m.Dir(), "sandbox.sock")
}

func (m *Machine) GuestSock() string {
	return path.Join(m.Dir(), fmt.Sprintf("%s.sock", m.Name))
}

func (m *Machine) StageUtil() *StageUtil {
	return &StageUtil{root: m.AbsDir}
}

func (m *Machine) SavePID() error {
	pidFile := m.PIDFile()
	_, err := os.Stat(pidFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		klog.Errorf("stat pidfile %q : %v", pidFile, err)
	}
	data, _ := json.MarshalIndent(PidCondition{
		Name:  m.Name,
		PID:   os.Getpid(),
		Stamp: time.Now(),
	}, "", "    ")
	return os.WriteFile(pidFile, data, 0o644)
}

func (m *Machine) LoadPID() (PidCondition, error) {
	pidFile := m.PIDFile()
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return PidCondition{}, err
	}
	pid := PidCondition{}
	err = json.Unmarshal(data, &pid)
	if err != nil {
		return PidCondition{}, err
	}
	_, err = validatePid(pid.PID)
	if err != nil {
		_ = os.Remove(pidFile)
		return PidCondition{}, err
	}
	return pid, nil
}

func validatePid(pid int) (int, error) {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return 0, err
	}
	if runtime.GOOS == "windows" {
		return pid, nil
	}
	err = proc.Signal(syscall.Signal(0))
	if err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			return 0, fmt.Errorf("pid %d is already done", pid)
		}
		// We may not have permission to send the signal (e.g. to network daemon running as root).
		// But if we get a permissions error, it means the process is still running.
		if !errors.Is(err, os.ErrPermission) {
			return 0, err
		}
	}
	return pid, nil
}

func (m *Machine) RemovePID() {
	pidFile := m.PIDFile()
	_ = os.RemoveAll(pidFile)
	klog.Infof("removing pid file %s", pidFile)
}

func (m *Machine) PIDFile() string {
	return filepath.Join(m.Dir(), v1.HostAgentPID)
}

// Protect protects the instance to prohibit accidental removal.
// Protect does not return an error even when the instance is already protected.
func (m *Machine) Protect() error {
	protected := filepath.Join(m.Dir(), v1.Protected)
	// TODO: Do an equivalent of `chmod +a "everyone deny delete,delete_child,file_inherit,directory_inherit"`
	// https://github.com/lima-vm/lima/issues/1595
	if err := os.WriteFile(protected, nil, 0o400); err != nil {
		return err
	}
	m.Protected = true
	return nil
}

// Unprotect unprotects the instance.
// Unprotect does not return an error even when the instance is already unprotected.
func (m *Machine) Unprotect() error {
	protected := filepath.Join(m.Dir(), v1.Protected)
	if err := os.RemoveAll(protected); err != nil {
		return err
	}
	m.Protected = false
	return nil
}

func (m *Machine) Stop() error {
	klog.Infof("stop instance: [%s]", m.Name)
	pid, err := m.LoadPID()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if pid.PID == 0 {
		return os.Remove(m.PIDFile())
	}
	proc, err := os.FindProcess(pid.PID)
	if err != nil {
		return errors.Wrapf(err, "find pid %d", pid.PID)
	}
	err = proc.Signal(os.Interrupt)
	if err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			_ = os.Remove(m.PIDFile())
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

func (m *Machine) Destroy() error {
	err := m.Stop()
	if err != nil {
		return err
	}
	klog.Infof("destroy instance: [%s/%s]", m.Dir(), m.Name)
	return os.RemoveAll(m.Dir())
}

func (m *Machine) InspectDisk(diskName string) (*MountDisk, error) {
	disk := &MountDisk{
		Name: diskName,
	}

	err := identifiers.Validate(diskName)
	if err != nil {
		return disk, err
	}
	diskDir := filepath.Join(m.Dir(), "_disks", diskName)

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
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		disk.Instance = filepath.Base(instDir)
		disk.InstanceDir = instDir
	}

	disk.MountPoint = fmt.Sprintf("/mnt/lima-%s", diskName)

	return disk, nil
}

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

func inspectDiskWithQemuImg(fName string) (size int64, format string, _ error) {
	return -1, "", fmt.Errorf("unimplemented")
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

func (m *Machine) SetDefault() error {
	dft, err := v1.LoadDft()
	if err != nil {
		return err
	}
	m.Created = metav1.Now()

	if m.Spec.Arch == "" {
		m.Spec.Arch = dft.Spec.Arch
	}
	if m.Spec.Memory == "" {
		m.Spec.Memory = dft.Spec.Memory
	}
	if m.Spec.OS == "" {
		m.Spec.OS = dft.Spec.OS
	}
	if m.Spec.CPUs == 0 {
		m.Spec.CPUs = dft.Spec.CPUs
	}
	if m.Spec.GuestVersion == "" {
		m.Spec.GuestVersion = dft.Spec.GuestVersion
	}
	if m.Spec.Image.Name == "" {
		m.Spec.Image = dft.Spec.Image
	}
	if m.Spec.VMType == "" {
		m.Spec.VMType = dft.Spec.VMType
	}
	if len(m.Spec.AdditionalDisks) == 0 {
		m.Spec.AdditionalDisks = append(m.Spec.AdditionalDisks, dft.Spec.AdditionalDisks...)
	}
	if m.Spec.Disk == "" {
		m.Spec.Disk = dft.Spec.Disk
	}
	if len(m.Spec.Mounts) == 0 {
		m.Spec.Mounts = append(m.Spec.Mounts, dft.Spec.Mounts...)
	}
	if len(m.Spec.Networks) == 0 {
		m.Spec.Networks = append(m.Spec.Networks, dft.Spec.Networks...)
	}
	// set macAddr
	for i, _ := range m.Spec.Networks {
		network := m.Spec.Networks[i]
		if network.MACAddress == "" {
			network.MACAddress = v1.GenMAC()
			klog.V(5).Infof("set mac address [%s] for %s", network.MACAddress, network.Interface)
		}
		m.Spec.Networks[i] = network
		klog.V(5).Infof("mac address is: %s for %s", network.MACAddress, network.Interface)
	}
	if m.Spec.Audio.Device == "" {
		m.Spec.Audio.Device = dft.Spec.Audio.Device
	}
	if m.Spec.Video.Display == "" {
		m.Spec.Video.Display = dft.Spec.Video.Display
	}
	if m.Spec.Video.VNC.Display == "" {
		m.Spec.Video.VNC.Display = dft.Spec.Video.VNC.Display
	}

	m.Spec.SetForward(v1.PortForward{
		SrcProto: "unix",
		SrcAddr:  intstr.FromString(m.GuestSock()),
		DstProto: "vsock",
		DstAddr:  intstr.FromInt32(10443),
	})
	m.Spec.SetMounts(v1.Mount{
		Writable:   true,
		Location:   fmt.Sprintf("~/mdata/%s", m.Name),
		MountPoint: "/mnt/disk0",
	})
	return m.Validate()
}

func (m *Machine) Validate() error {
	if m.Spec.Arch == "" {
		return fmt.Errorf("arch must be specified: x86_64, aarch64")
	}
	if m.Spec.Image.Name == "" {
		return fmt.Errorf("image name must be specified")
	}
	if m.Spec.VMType == "" {
		return fmt.Errorf("vm type must be specified")
	}
	if m.Spec.CPUs == 0 {
		return fmt.Errorf("cpus must be specified")
	}
	if m.Spec.OS == "" {
		return fmt.Errorf("os must be specified")
	}
	return nil
}
