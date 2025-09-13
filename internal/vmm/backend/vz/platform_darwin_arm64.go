//go:build darwin && arm64 && !no_vz

package vz

import (
	"context"
	"fmt"
	"github.com/Code-Hex/vz/v3"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/vmm/backend"
	"github.com/docker/go-units"
	gerrors "github.com/pkg/errors"
	"k8s.io/klog/v2"
	"os"
	"path"
	"path/filepath"
	"time"
)

func installVm(ctx context.Context, name string, vm *vz.VirtualMachine, image string, dir string) error {
	installer, err := vz.NewMacOSInstaller(vm, image)
	if err != nil {
		return gerrors.Wrap(err, "failed to create a new macOS installer")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				klog.Errorf("install has been cancelled: %s", name)
				return
			case <-installer.Done():
				err = setDiskInitialized(dir)
				if err != nil {
					klog.Errorf("failed to set the disk initialized: %v", err)
				}
				klog.Infof("install has completed")
				return
			case <-ticker.C:
				klog.Infof("[%s]macos installation progress: %.1f%%\r", name, installer.FractionCompleted()*100)
			}
		}
	}()

	return installer.Install(ctx)
}

func newPlatformConfigMac(driver *backend.BaseDriver, image string) (vz.PlatformConfiguration, error) {
	if image != "" {
		return newMacInstallPlatformCfg(driver, image)
	}
	var (
		hardware   = filepath.Join(driver.I.Dir(), v1.HardwareModel)
		identifier = filepath.Join(driver.I.Dir(), v1.VzIdentifier)
		auxiliary  = filepath.Join(driver.I.Dir(), v1.AuxiliaryStoraage)
	)
	auxiliaryStorage, err := vz.NewMacAuxiliaryStorage(auxiliary)
	if err != nil {
		return nil, fmt.Errorf("failed to create a new mac auxiliary storage: %w", err)
	}
	hardwareModel, err := vz.NewMacHardwareModelWithDataPath(hardware)
	if err != nil {
		return nil, fmt.Errorf("failed to create a new hardware model: %w", err)
	}
	machineIdentifier, err := vz.NewMacMachineIdentifierWithDataPath(identifier)
	if err != nil {
		return nil, fmt.Errorf("failed to create a new machine identifier: %w", err)
	}
	return vz.NewMacPlatformConfiguration(
		vz.WithMacAuxiliaryStorage(auxiliaryStorage),
		vz.WithMacHardwareModel(hardwareModel),
		vz.WithMacMachineIdentifier(machineIdentifier),
	)
}

func createInitialConfigMac(driver *backend.BaseDriver) (*vz.VirtualMachineConfiguration, error) {
	bootloader, err := vz.NewMacOSBootLoader()
	if err != nil {
		return nil, err
	}

	bytes, err := units.RAMInBytes(driver.I.Spec.Memory)
	if err != nil {
		return nil, err
	}

	return vz.NewVirtualMachineConfiguration(
		bootloader, uint(driver.I.Spec.CPUs), uint64(bytes))
}

// CreateFileAndWriteTo creates a new file and write data to it.
func save(data []byte, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file %q: %w", path, err)
	}
	defer f.Close()

	_, err = f.Write(data)
	return err
}

func newMacInstallPlatformCfg(driver *backend.BaseDriver, image string) (vz.PlatformConfiguration, error) {
	restoreImage, err := vz.LoadMacOSRestoreImageFromPath(image)
	if err != nil {
		return nil, fmt.Errorf("failed to load restore image:[%s] %w", image, err)
	}
	macOSConfiguration := restoreImage.MostFeaturefulSupportedConfiguration()

	hardwareModel := macOSConfiguration.HardwareModel()
	if err := save(
		hardwareModel.DataRepresentation(),
		path.Join(driver.I.Dir(), v1.HardwareModel),
	); err != nil {
		return nil, fmt.Errorf("failed to write hardware model data: %w", err)
	}

	machineIdentifier, err := vz.NewMacMachineIdentifier()
	if err != nil {
		return nil, err
	}
	if err := save(
		machineIdentifier.DataRepresentation(),
		path.Join(driver.I.Dir(), v1.VzIdentifier),
	); err != nil {
		return nil, fmt.Errorf("failed to write machine identifier data: %w", err)
	}

	auxiliaryStorage, err := vz.NewMacAuxiliaryStorage(
		path.Join(driver.I.Dir(), v1.AuxiliaryStoraage),
		vz.WithCreatingMacAuxiliaryStorage(hardwareModel),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create a new mac auxiliary storage: %w", err)
	}
	return vz.NewMacPlatformConfiguration(
		vz.WithMacAuxiliaryStorage(auxiliaryStorage),
		vz.WithMacHardwareModel(hardwareModel),
		vz.WithMacMachineIdentifier(machineIdentifier),
	)
}

func GetLatestRestoreImageURL() (string, error) {
	return vz.GetLatestSupportedMacOSRestoreImageURL()
}

func GetMacGDC() (vz.GraphicsDeviceConfiguration, error) {
	graphicsDeviceConfiguration, err := vz.NewMacGraphicsDeviceConfiguration()
	if err != nil {
		return nil, err
	}
	scanoutConfiguration, err := vz.NewMacGraphicsDisplayConfiguration(1920, 1200, 80)
	if err != nil {
		return nil, err
	}
	graphicsDeviceConfiguration.SetDisplays(scanoutConfiguration)

	return graphicsDeviceConfiguration, nil
}
