//go:build darwin && !no_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"context"
	"errors"
	"fmt"
	"github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/tool/downloader"
	"github.com/aoxn/meridian/internal/tool/iso9660util"
	nativeimg "github.com/aoxn/meridian/internal/vmm/nativeimg"
	"github.com/docker/go-units"
	dialer "golang.org/x/net/proxy"
	"k8s.io/klog/v2"
	"net"
	"os"
	"path"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Code-Hex/vz/v3"
	"github.com/aoxn/meridian/internal/vmm/backend"
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

	if l.I.Spec.Firmware.LegacyBIOS {
		return fmt.Errorf("`firmware.legacyBIOS` configuration is not supported for VZ driver")
	}
	for _, f := range l.I.Spec.Firmware.Images {
		switch f.VMType {
		case "", v1.VZ:
			if f.Arch == l.I.Spec.Arch {
				return fmt.Errorf("`firmware.images` configuration is not supported for VZ driver")
			}
		}
	}

	if !v1.IsNativeArch(l.I.Spec.Arch) {
		return fmt.Errorf("unsupported arch: %q", l.I.Spec.Arch)
	}

	switch audioDevice := l.I.Spec.Audio.Device; audioDevice {
	case "":
	case "vz", "default", "none":
	default:
		klog.Infof("field `audio.device` must be \"vz\", \"default\", or \"none\" for VZ driver, got %q", audioDevice)
	}

	switch videoDisplay := l.I.Spec.Video.Display; videoDisplay {
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
	diffDisk := filepath.Join(l.I.Dir(), v1.DiffDisk)
	if _, err := os.Stat(diffDisk); err == nil || !errors.Is(err, os.ErrNotExist) {
		// disk is already ensured
		return err
	}
	vmInfo := l.I.Spec
	baseDisk := filepath.Join(l.I.Dir(), v1.BaseDisk)
	if _, err := os.Stat(baseDisk); errors.Is(err, os.ErrNotExist) {
		f := v1.FindImage(l.I.Spec.Image.Name)
		if f == nil {
			return fmt.Errorf("unexpected image name: [%s]", l.I.Spec.Image.Name)
		}
		if f.Arch != vmInfo.Arch {
			return fmt.Errorf("%q: unsupported arch: %q, expected=%q", f.Location, f.Arch, vmInfo.Arch)
		}
		res, err := downloader.Download(ctx, baseDisk, f.Location,
			downloader.WithCache(),
			downloader.WithDecompress(true),
			downloader.WithDescription(fmt.Sprintf("%s (%s)", "guest vm image", path.Base(f.Location))),
			downloader.WithExpectedDigest(f.Digest),
		)
		if err != nil {
			return fmt.Errorf("failed to download %q: %w", f.Location, err)
		}
		klog.Infof("download base disk for image: %s, from %s, [%s]", vmInfo.Image.Name, f.Location, res.Status)
	}
	diskSize, _ := units.RAMInBytes(l.I.Spec.Disk)
	if diskSize == 0 {
		return nil
	}
	isBaseDiskISO, err := iso9660util.IsISO9660(baseDisk)
	if err != nil {
		return err
	}
	if isBaseDiskISO {
		// Create an empty data volume (sparse)
		diffDiskF, err := os.Create(diffDisk)
		if err != nil {
			return err
		}
		if err = nativeimg.MakeSparse(diffDiskF, diskSize); err != nil {
			diffDiskF.Close()
			return err
		}
		return diffDiskF.Close()
	}
	if err = nativeimg.ConvertToRaw(baseDisk, diffDisk, &diskSize, false); err != nil {
		return fmt.Errorf("failed to convert %q to a raw disk %q: %w", baseDisk, diffDisk, err)
	}
	return err
}

func (l *VzDriver) Start(ctx context.Context) (chan error, error) {

	setNofileRlimit()

	klog.Infof("Starting VZ (hint: to watch the boot progress, see %q)", filepath.Join(l.I.Dir(), "serial*.log"))
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
	switch l.I.Spec.Video.Display {
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
	return fmt.Errorf("RunGUI is not supported for the given driver '%s' and display '%s'", "vz", l.I.Spec.Video.Display)
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
