//go:build darwin && !no_vz

package vz

import (
	"context"
	"errors"
	"fmt"
	"github.com/aoxn/meridian/api/v1"
	dialer "golang.org/x/net/proxy"
	"k8s.io/klog/v2"
	"net"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Code-Hex/vz/v3"
	"github.com/aoxn/meridian/internal/vma/backend"
)

const Enabled = true

type VzDriver struct {
	*backend.BaseDriver

	machine *vmWrapper
}

func New(driver *backend.BaseDriver) *VzDriver {
	return &VzDriver{
		BaseDriver: driver,
	}
}

func (l *VzDriver) Validate() error {
	// Calling NewEFIBootLoader to do required version check for latest APIs
	_, err := vz.NewEFIBootLoader()
	if errors.Is(err, vz.ErrUnsupportedOSVersion) {
		return fmt.Errorf("VZ driver requires macOS 13 or higher to run")
	}

	if l.Yaml.Spec.Firmware.LegacyBIOS {
		return fmt.Errorf("`firmware.legacyBIOS` configuration is not supported for VZ driver")
	}
	for _, f := range l.Yaml.Spec.Firmware.Images {
		switch f.VMType {
		case "", v1.VZ:
			if f.Arch == l.Yaml.Spec.Arch {
				return fmt.Errorf("`firmware.images` configuration is not supported for VZ driver")
			}
		}
	}

	if !v1.IsNativeArch(l.Yaml.Spec.Arch) {
		return fmt.Errorf("unsupported arch: %q", l.Yaml.Spec.Arch)
	}

	switch audioDevice := l.Yaml.Spec.Audio.Device; audioDevice {
	case "":
	case "vz", "default", "none":
	default:
		klog.Infof("field `audio.device` must be \"vz\", \"default\", or \"none\" for VZ driver, got %q", audioDevice)
	}

	switch videoDisplay := l.Yaml.Spec.Video.Display; videoDisplay {
	case "vz", "default", "none":
	default:
		klog.Infof("field `video.display` must be \"vz\", \"default\", or \"none\" for VZ driver , got %q", videoDisplay)
	}
	return nil
}

func (l *VzDriver) Initialize(_ context.Context) error {
	_, err := getMachineIdentifier(l.BaseDriver)
	return err
}

func (l *VzDriver) CreateDisk(ctx context.Context) error {
	return EnsureDisk(ctx, l.BaseDriver)
}

func (l *VzDriver) Start(ctx context.Context) (chan error, error) {

	setNofileRlimit()

	klog.Infof("Starting VZ (hint: to watch the boot progress, see %q)", filepath.Join(l.I.Dir, "serial*.log"))
	vm, errCh, err := startVM(ctx, l.BaseDriver)
	if err != nil {
		if errors.Is(err, vz.ErrUnsupportedOSVersion) {
			return nil, fmt.Errorf("vz driver requires macOS 13 or higher to run: %w", err)
		}
		return nil, err
	}
	l.machine = vm

	return errCh, nil
}

func (l *VzDriver) CanRunGUI() bool {
	switch l.Yaml.Spec.Video.Display {
	case "vz", "default":
		return true
	default:
		return false
	}
}

func (l *VzDriver) RunGUI() error {
	if l.CanRunGUI() {
		return l.machine.StartGraphicApplication(1920, 1200)
	}
	//nolint:revive // error-strings
	return fmt.Errorf("RunGUI is not supported for the given driver '%s' and display '%s'", "vz", l.Yaml.Spec.Video.Display)
}

func (l *VzDriver) Stop(_ context.Context) error {
	klog.Info("Shutting down VZ")
	canStop := l.machine.CanRequestStop()

	if canStop {
		klog.Infof("request vm stop")
		_, err := l.machine.RequestStop()
		if err != nil {
			return err
		}

		timeout := time.After(60 * time.Second)
		ticker := time.NewTicker(500 * time.Millisecond)
		for {
			select {
			case <-timeout:
				return errors.New("vz timeout while waiting for stop status")
			case <-ticker.C:
				l.machine.mu.RLock()
				stopped := l.machine.stopped
				l.machine.mu.RUnlock()
				if stopped {
					return nil
				}
			}
		}
	}

	return errors.New("vz: CanRequestStop is not supported")
}

func (l *VzDriver) GuestAgentConn(_ context.Context) (net.Conn, error) {
	if len(l.machine.SocketDevices()) == 0 {
		return nil, fmt.Errorf("vz does not support guest agent")
	}
	klog.Infof("connect to guest agent through [%d]", l.VSockPort)
	device := l.machine.SocketDevices()[0]
	return device.Connect(uint32(l.VSockPort))
}

func (l *VzDriver) Dialer(_ context.Context) (dialer.Dialer, error) {
	dev := l.machine.SocketDevices()
	if len(dev) == 0 {
		return nil, fmt.Errorf("no devices found")
	}
	return NewDialer(dev[0]), nil
}

// Default nofile limit is too low on some system.
// For example in the macOS standard terminal is 256.
// It means that there are only ~240 connections available from the host to the vm.
// That function increases the nofile limit for child processes, especially the ssh process
//
// More about limits in go: https://go.dev/issue/46279
func setNofileRlimit() {
	var limit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
		klog.Errorf("failed to get RLIMIT_NOFILE: %s", err.Error())
	} else if limit.Cur != limit.Max {
		limit.Cur = limit.Max
		err = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &limit)
		if err != nil {
			klog.Warningf("failed to set RLIMIT_NOFILE (%+v), %s", limit, err.Error())
		}
	}
}
