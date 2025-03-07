// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package wsl2

import (
	"context"
	"fmt"
	"github.com/aoxn/meridian/api/v1"
	"k8s.io/klog/v2"
	"net"
	"regexp"
	"strconv"

	"github.com/Microsoft/go-winio"
	"github.com/Microsoft/go-winio/pkg/guid"

	"github.com/aoxn/meridian/internal/vma/backend"
	dialer "golang.org/x/net/proxy"
)

type MdWslDriver struct {
	*backend.BaseDriver
}

func New(driver *backend.BaseDriver) *MdWslDriver {
	return &MdWslDriver{
		BaseDriver: driver,
	}
}

func (l *MdWslDriver) Validate() error {

	if !v1.IsNativeArch(l.Yaml.Arch) {
		return fmt.Errorf("unsupported arch: %q", l.Yaml.Arch)
	}

	// TODO: real filetype checks
	tarFileRegex := regexp.MustCompile(`.*tar\.*`)
	for _, image := range l.Yaml.Images {
		match := tarFileRegex.MatchString(image.Location)
		if image.Arch == l.Yaml.Arch && !match {
			return fmt.Errorf("unsupported image type for vmType: %s, tarball root file system required: %q", l.Yaml.VMType, image.Location)
		}
	}

	audioDevice := l.Yaml.Audio.Device
	if audioDevice != "" {
		klog.Infof("Ignoring: vmType %s: `audio.device`: %+v", l.Yaml.VMType, audioDevice)
	}

	return nil
}

func (l *MdWslDriver) Start(ctx context.Context) (chan error, error) {
	klog.Info("Starting WSL vm")
	status, err := GetWslStatus(l.I.Name)
	if err != nil {
		return nil, err
	}

	distroName := "md-" + l.I.Name

	if status == StatusUninitialized {
		if err := EnsureFs(ctx, l.BaseDriver); err != nil {
			return nil, err
		}
		if err := initVM(ctx, l.BaseDriver.I.Dir, distroName); err != nil {
			return nil, err
		}
	}

	errCh := make(chan error)

	if err := startVM(ctx, distroName); err != nil {
		return nil, err
	}

	if err := provisionVM(
		ctx,
		l.BaseDriver.I.Dir,
		l.BaseDriver.I.Name,
		distroName,
		&errCh,
	); err != nil {
		return nil, err
	}

	keepAlive(ctx, distroName, &errCh)

	return errCh, err
}

func (l *MdWslDriver) CanRunGUI() bool {
	// return *l.Yaml.Video.Display == "wsl"
	return false
}

func (l *MdWslDriver) RunGUI() error {
	return fmt.Errorf("RunGUI is not supported for the given driver '%s' and display '%s'", "wsl", l.Yaml.Video.Display)
}

func (l *MdWslDriver) Stop(ctx context.Context) error {
	klog.Info("Shutting down WSL2 vm")
	distroName := "md-" + l.I.Name
	return stopVM(ctx, distroName)
}

func (l *MdWslDriver) Unregister(ctx context.Context) error {
	distroName := "md-" + l.I.Name
	status, err := GetWslStatus(l.I.Name)
	if err != nil {
		return err
	}
	switch status {
	case StatusRunning, StatusStopped, StatusBroken, StatusInstalling:
		return unregisterVM(ctx, distroName)
	}

	klog.Info("vm not registered, skipping unregistration")
	return nil
}

// GuestAgentConn returns the guest agent connection, or nil (if forwarded by ssh).
// As of 08-01-2024, github.com/mdlayher/vsock does not natively support vsock on
// Windows, so use the winio library to create the connection.
func (l *MdWslDriver) GuestAgentConn(ctx context.Context) (net.Conn, error) {
	VMIDStr, err := GetInstanceVMID(fmt.Sprintf("md-%s", l.I.Name))
	if err != nil {
		return nil, err
	}
	VMIDGUID, err := guid.FromString(VMIDStr)
	if err != nil {
		return nil, err
	}
	sockAddr := &winio.HvsockAddr{
		VMID:      VMIDGUID,
		ServiceID: winio.VsockServiceID(uint32(l.VSockPort)),
	}
	return winio.Dial(ctx, sockAddr)
}

func (l *MdWslDriver) Dialer(_ context.Context) (dialer.Dialer, error) {
	return &winDialer{
		vmName: l.I.Name,
	}, nil
}

type winDialer struct {
	vmName string
}

func (d *winDialer) Dial(network, addr string) (c net.Conn, err error) {
	id, err := GetInstanceVMID(fmt.Sprintf("md-%s", d.vmName))
	if err != nil {
		return nil, err
	}
	VMIDGUID, err := guid.FromString(id)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(addr)
	if err != nil {
		return nil, err
	}
	sockAddr := &winio.HvsockAddr{
		VMID:      VMIDGUID,
		ServiceID: winio.VsockServiceID(uint32(port)),
	}
	return winio.Dial(context.TODO(), sockAddr)
}
