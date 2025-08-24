package meta

import (
	"encoding/json"
	"fmt"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/containerd/containerd/identifiers"
	"github.com/lima-vm/go-qcow2reader"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"
)

type PidCondition struct {
	Name  string    `json:"name"`
	PID   string    `json:"pid"`
	Stamp time.Time `json:"stamp"`
}

func (p PidCondition) String() string {
	return fmt.Sprintf("%s:%s, %s", p.Name, p.PID, p.Stamp)
}

type Machine struct {
	Name      string                 `json:"name"`
	AbsDir    string                 `json:"absDir"`
	Protected bool                   `json:"protected"`
	Spec      *v1.VirtualMachineSpec `json:"spec"`
}

func (m *Machine) Dir() string {
	return m.AbsDir
}

func (m *Machine) SavePID() error {
	pidFile := m.PIDFile()
	_, err := os.Stat(pidFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		klog.Errorf("stat pidfile %q : %v", pidFile, err)
	}
	pid := strconv.Itoa(os.Getpid())
	data, _ := json.MarshalIndent(PidCondition{
		Name:  m.Name,
		PID:   pid,
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
	id, err := strconv.Atoi(pid.PID)
	if err != nil {
		return PidCondition{}, err
	}
	_, err = validatePid(id)
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
	if pid.PID == "" {
		return os.Remove(m.PIDFile())
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
