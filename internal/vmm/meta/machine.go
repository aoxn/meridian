package meta

import (
	"context"
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
	"strconv"
	"strings"
	"syscall"
	"time"

	proc "github.com/shirou/gopsutil/v4/process"
)

type PidCondition struct {
	Name  string    `json:"name"`
	PID   int       `json:"pid"`
	Stamp time.Time `json:"stamp"`
}

func (p PidCondition) String() string {
	return fmt.Sprintf("%s:%d, %s", p.Name, p.PID, p.Stamp)
}

type machine struct {
	root string
}

func (m *machine) Dir() string {
	return m.rootLocation()
}

func (m *machine) rootLocation(name ...string) string {
	return path.Join(m.root, "vms", path.Join(name...))
}

func (m *machine) Get(key string) (*Machine, error) {
	mch, err := m.get(key)
	if err != nil {
		return mch, err
	}
	pid, _ := mch.LoadPID()
	mch.SandboxPID = pid
	return mch, nil
}

func (m *machine) get(key string) (*Machine, error) {
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
	return m.load(path.Join(pathName, machineJson))
}

func (m *machine) List() ([]*Machine, error) {
	var machines []*Machine
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
		mch, err := m.Get(dirName)
		if err != nil {
			continue
		}
		machines = append(machines, mch)
	}
	return machines, nil
}

func (m *machine) Create(machine *Machine) error {
	pathName := m.rootLocation(machine.Name)
	_, err := os.Stat(pathName)
	if err == nil {
		return fmt.Errorf("%s already exists", pathName)
	}
	err = os.MkdirAll(pathName, 0755)
	if err != nil {
		return err
	}
	machine.AbsDir = pathName

	data, err := json.MarshalIndent(machine, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path.Join(pathName, machineJson), data, 0644)
}

func (m *machine) Update(machine *Machine) error {
	pathName := m.rootLocation(machine.Name)
	_, err := os.Stat(pathName)
	if err != nil {
		return fmt.Errorf("%s not exists", pathName)
	}
	data, err := json.MarshalIndent(machine, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path.Join(pathName, machineJson), data, 0644)
}

func (m *machine) Destroy(machine *Machine) error {
	if machine.Name == "" {
		return fmt.Errorf("machine name is empty")
	}
	return os.RemoveAll(m.rootLocation(machine.Name))
}

func (m *machine) load(machineUri string) (*Machine, error) {
	data, err := os.ReadFile(machineUri)
	if err != nil {
		return nil, err
	}
	var mch Machine
	err = json.Unmarshal(data, &mch)
	if err != nil {
		return nil, err
	}
	mch.AbsDir = m.rootLocation(mch.Name)
	return &mch, nil
}

type Machine struct {
	Name       string                 `json:"name"`
	AbsDir     string                 `json:"absDir"`
	Created    metav1.Time            `json:"created"`
	SandboxPID int32                  `json:"sandboxPid"`
	Login      string                 `json:"lgoin"`
	Spec       *v1.VirtualMachineSpec `json:"spec"`
	Protected  bool                   `json:"protected"`
	State      string                 `json:"state"`
	Message    string                 `json:"message,omitempty"`
	Address    []string               `json:"address,omitempty"`
	Stage      []Stage                `json:"stage,omitempty"`
}

type Stage struct {
	Phase       string    `json:"phase"`
	Timestamp   time.Time `json:"timestamp"`
	Description string    `json:"description"`
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

func (m *Machine) LoadPID() (int32, error) {
	pidFile := m.PIDFile()
	data, err := os.ReadFile(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, fmt.Errorf("NotFound")
		}
		_ = os.Remove(pidFile)
		return 0, err
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		_ = os.Remove(pidFile)
		return 0, errors.Wrapf(err, "parse pid from [%s]", data)
	}

	_, err = validatePid(int32(pid))
	if err != nil {
		_ = os.Remove(pidFile)
		return 0, err
	}
	return int32(pid), nil
}

func (m *Machine) WaitStop(ctx context.Context, timeout time.Duration) error {

	after := time.After(timeout)
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context done: %s", m.Name)
		case <-after:
			return fmt.Errorf("timeout wait for stop: %s", m.Name)
		default:
		}

		_, err := m.LoadPID()
		if err != nil {
			return nil
		}
	}
}

func validatePid(pid int32) (int32, error) {
	p, err := proc.NewProcess(pid)
	if err != nil {
		return 0, err
	}
	n, err := p.Name()
	if err != nil {
		return 0, errors.Wrapf(err, "get process name by pid %d", pid)
	}
	if !strings.Contains(n, "meridian") {
		return 0, fmt.Errorf("NotFound: %d", pid)
	}
	if runtime.GOOS == "windows" {
		return pid, nil
	}
	err = p.SendSignal(syscall.Signal(0))
	if err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			return 0, fmt.Errorf("NotFound")
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

func (m *Machine) Stop(ctx context.Context) error {
	klog.Infof("stop instance: [%s]", m.Name)
	pid, err := m.LoadPID()
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") {
			return nil
		}
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if pid == 0 {
		return os.Remove(m.PIDFile())
	}
	proc, err := os.FindProcess(int(pid))
	if err != nil {
		return errors.Wrapf(err, "find pid %d", pid)
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
	klog.V(5).Infof("signal pid %d with [os.Interupt] signal", pid)
	return m.WaitStop(ctx, 3*time.Minute)
}

func (m *Machine) Destroy(ctx context.Context) error {
	err := m.Stop(ctx)
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
