//go:build darwin && amd64

package vz

import (
	"context"
	"fmt"
	"github.com/Code-Hex/vz/v3"
	"github.com/aoxn/meridian/internal/vmm/backend"
	"runtime"
)

func installVm(ctx context.Context, vm *vz.VirtualMachine, image string) error {
	return fmt.Errorf("installVm on darwin %s/%s not implemented", runtime.GOOS, runtime.GOARCH)
}

func newPlatformConfigMac(driver *backend.BaseDriver, image string) (vz.PlatformConfiguration, error) {
	return nil, fmt.Errorf("new platform config mac not implemented on %s/%s", runtime.GOOS, runtime.GOARCH)
}

func createInitialConfigMac(driver *backend.BaseDriver) (*vz.VirtualMachineConfiguration, error) {
	return nil, fmt.Errorf("createInitialConfigMac, platform not supported for arch: %s/%s on darwin", runtime.GOOS, runtime.GOARCH)
}
