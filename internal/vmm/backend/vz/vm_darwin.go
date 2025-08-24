//go:build darwin && !no_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"context"
	"fmt"
	"github.com/aoxn/meridian/api/v1"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
	"k8s.io/klog/v2"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"

	"github.com/Code-Hex/vz/v3"
	"github.com/aoxn/meridian/internal/tool/iso9660util"
	"github.com/aoxn/meridian/internal/vmm/backend"
	nativeimgutil "github.com/aoxn/meridian/internal/vmm/nativeimg"
	"github.com/docker/go-units"
	"github.com/lima-vm/go-qcow2reader"

	"github.com/lima-vm/go-qcow2reader/image/raw"
	"github.com/pkg/term/termios"
)

// diskImageCachingMode is set to DiskImageCachingModeCached so as to avoid disk corruption on ARM:
// - https://github.com/utmapp/UTM/issues/4840#issuecomment-1824340975
// - https://github.com/utmapp/UTM/issues/4840#issuecomment-1824542732
//
// Eventually we may bring this back to DiskImageCachingModeAutomatic when the corruption issue is properly fixed.
const diskImageCachingMode = vz.DiskImageCachingModeCached

type vmWrapper struct {
	*vz.VirtualMachine
	mu      *sync.RWMutex
	stopped bool
	errCh   chan error
}

// Hold all *os.File created via socketpair() so that they won't get garbage collected. f.FD() gets invalid if f gets garbage collected.
var vmNetworkFiles = make([]*os.File, 1)

func startVM(ctx context.Context, driver *backend.BaseDriver) (*vmWrapper, chan error, error) {

	machine, err := createVM(driver)
	if err != nil {
		return nil, nil, err
	}

	wrapper := &vmWrapper{
		errCh:          make(chan error),
		stopped:        false,
		mu:             &sync.RWMutex{},
		VirtualMachine: machine,
	}

	err = wrapper.Start()
	if err != nil {
		return nil, nil, err
	}

	go wrapper.Loop(ctx, driver)
	return wrapper, wrapper.errCh, err
}

func (vm *vmWrapper) Loop(ctx context.Context, driver *backend.BaseDriver) {
	// Handle errors via errCh and handle stop vm during context close
	defer func() {
		for i := range vmNetworkFiles {
			vmNetworkFiles[i].Close()
		}
	}()
	go func() {
		select {
		case <-ctx.Done():
			klog.Info("Context closed, stopping vm")
			if !vm.CanStop() {
				klog.Errorf("can not request stop")
				return
			}
			ret, err := vm.RequestStop()
			klog.Errorf("request stopping the vm with: [%t][error=%v]", ret, err)
		}
	}()
	filesToRemove := make(map[string]struct{})
	for {
		select {
		case newState := <-vm.StateChangedNotify():
			klog.Infof("[VZ] - vm state changed: [%s]", newState)
			switch newState {
			case vz.VirtualMachineStateRunning:
				err := driver.I.SavePID()
				if err != nil {
					klog.Errorf("save pidfile %q already exists", driver.I.PIDFile())
					vm.errCh <- err
				}
				filesToRemove[driver.I.PIDFile()] = struct{}{}
			case vz.VirtualMachineStateStopped:
				for f := range filesToRemove {
					err := os.RemoveAll(f)
					klog.Infof("remove pid file: %s,[%v]", f, err)
				}
				vm.mu.Lock()
				vm.stopped = true
				vm.mu.Unlock()
				vm.errCh <- errors.New("vz driver state stopped")
			default:
			}
		}
	}
}

func createVM(driver *backend.BaseDriver) (*vz.VirtualMachine, error) {
	vmConfig, err := createInitialConfig(driver)
	if err != nil {
		return nil, err
	}

	if err = attachPlatformConfig(driver, vmConfig); err != nil {
		return nil, err
	}

	if err = attachSerialPort(driver, vmConfig); err != nil {
		return nil, err
	}

	if err = attachNetwork(driver, vmConfig); err != nil {
		return nil, err
	}

	if err = attachDisks(driver, vmConfig); err != nil {
		return nil, err
	}

	if err = attachDisplay(driver, vmConfig); err != nil {
		return nil, err
	}

	if err = attachFolderMounts(driver, vmConfig); err != nil {
		return nil, err
	}

	if err = attachAudio(driver, vmConfig); err != nil {
		return nil, err
	}

	if err = attachOtherDevices(driver, vmConfig); err != nil {
		return nil, err
	}

	validated, err := vmConfig.Validate()
	if !validated || err != nil {
		return nil, err
	}

	return vz.NewVirtualMachine(vmConfig)
}

func createInitialConfig(driver *backend.BaseDriver) (*vz.VirtualMachineConfiguration, error) {
	efiVariableStore, err := getEFI(driver)
	if err != nil {
		return nil, err
	}

	bootLoader, err := vz.NewEFIBootLoader(vz.WithEFIVariableStore(efiVariableStore))
	if err != nil {
		return nil, err
	}

	bytes, err := units.RAMInBytes(driver.I.Spec.Memory)
	if err != nil {
		return nil, err
	}

	vmConfig, err := vz.NewVirtualMachineConfiguration(
		bootLoader,
		uint(driver.I.Spec.CPUs),
		uint64(bytes),
	)
	if err != nil {
		return nil, err
	}
	return vmConfig, nil
}

func attachPlatformConfig(driver *backend.BaseDriver, vmConfig *vz.VirtualMachineConfiguration) error {
	machineIdentifier, err := getMachineIdentifier(driver)
	if err != nil {
		return err
	}

	platformConfig, err := vz.NewGenericPlatformConfiguration(vz.WithGenericMachineIdentifier(machineIdentifier))
	if err != nil {
		return err
	}
	vmConfig.SetPlatformVirtualMachineConfiguration(platformConfig)
	return nil
}

// https://developer.apple.com/documentation/virtualization/running_linux_in_a_virtual_machine?language=objc#:~:text=Configure%20the%20Serial%20Port%20Device%20for%20Standard%20In%20and%20Out
func setRawMode(f *os.File) {
	var attr unix.Termios

	// Get settings for terminal
	err := termios.Tcgetattr(f.Fd(), &attr)
	if err != nil {
		klog.Errorf("get fd attribute failed: %v", err)
	}
	// Put stdin into raw mode, disabling local echo, input canonicalization,
	// and CR-NL mapping.
	attr.Iflag &^= syscall.ICRNL
	attr.Lflag &^= syscall.ICANON | syscall.ECHO

	// Set minimum characters when reading = 1 char
	attr.Cc[syscall.VMIN] = 1

	// set timeout when reading as non-canonical mode
	attr.Cc[syscall.VTIME] = 0

	// reflects the changed settings
	termios.Tcsetattr(f.Fd(), termios.TCSANOW, &attr)
	if err != nil {
		klog.Errorf("set fd attribute failed: %v", err)
	}
}

func attachSerialPort(driver *backend.BaseDriver, config *vz.VirtualMachineConfiguration) error {
	path := filepath.Join(driver.I.Dir(), v1.SerialVirtioLog)
	serialPortAttachment, err := vz.NewFileSerialPortAttachment(path, false)
	if err != nil {
		return err
	}
	setRawMode(os.Stdin)
	//serialPortAttachment, err := vz.NewFileHandleSerialPortAttachment(os.Stdin, os.Stdout)
	//if err != nil {
	//	return err
	//}
	consoleConfig, err := vz.NewVirtioConsoleDeviceSerialPortConfiguration(serialPortAttachment)
	config.SetSerialPortsVirtualMachineConfiguration([]*vz.VirtioConsoleDeviceSerialPortConfiguration{
		consoleConfig,
	})
	return err
}

func createSpiceAgentConsoleDeviceConfiguration(driver *backend.BaseDriver, config *vz.VirtualMachineConfiguration) error {
	consoleDevice, err := vz.NewVirtioConsoleDeviceConfiguration()
	if err != nil {
		return fmt.Errorf("failed to create a new console device: %w", err)
	}

	spiceAgentAttachment, err := vz.NewSpiceAgentPortAttachment()
	if err != nil {
		return fmt.Errorf("failed to create a new spice agent attachment: %w", err)
	}
	spiceAgentName, err := vz.SpiceAgentPortAttachmentName()
	if err != nil {
		return fmt.Errorf("failed to get spice agent name: %w", err)
	}
	spiceAgentPort, err := vz.NewVirtioConsolePortConfiguration(
		vz.WithVirtioConsolePortConfigurationAttachment(spiceAgentAttachment),
		vz.WithVirtioConsolePortConfigurationName(spiceAgentName),
	)
	if err != nil {
		return fmt.Errorf("failed to create a new console port for spice agent: %w", err)
	}

	consoleDevice.SetVirtioConsolePortConfiguration(0, spiceAgentPort)

	config.SetConsoleDevicesVirtualMachineConfiguration([]vz.ConsoleDeviceConfiguration{
		consoleDevice,
	})

	return nil
}

func newVirtioFileNetworkDeviceConfiguration(file *os.File, macStr string) (*vz.VirtioNetworkDeviceConfiguration, error) {
	fileAttachment, err := vz.NewFileHandleNetworkDeviceAttachment(file)
	if err != nil {
		return nil, err
	}
	return newVirtioNetworkDeviceConfiguration(fileAttachment, macStr)
}

func newVirtioNetworkDeviceConfiguration(attachment vz.NetworkDeviceAttachment, macStr string) (*vz.VirtioNetworkDeviceConfiguration, error) {
	networkConfig, err := vz.NewVirtioNetworkDeviceConfiguration(attachment)
	if err != nil {
		return nil, err
	}
	mac, err := net.ParseMAC(macStr)
	if err != nil {
		return nil, err
	}
	address, err := vz.NewMACAddress(mac)
	if err != nil {
		return nil, err
	}
	networkConfig.SetMACAddress(address)
	return networkConfig, nil
}

func attachNetwork(driver *backend.BaseDriver, vmConfig *vz.VirtualMachineConfiguration) error {
	var configurations []*vz.VirtioNetworkDeviceConfiguration

	for _, nw := range driver.I.Spec.Networks {
		var (
			err        error
			attachment vz.NetworkDeviceAttachment
		)
		if nw.VZNAT {
			attachment, err = vz.NewNATNetworkDeviceAttachment()
			if err != nil {
				return err
			}
		} else {
			itf := vz.NetworkInterfaces()
			for i, v := range itf {
				klog.Infof("found bridge interface [%d]: %s, %s", i, v.Identifier(), v.LocalizedDisplayName())
			}
			if len(itf) <= 0 {
				return fmt.Errorf("no network interface found")
			}
			klog.Infof("vz use bridge interface [%v]", itf[0])
			attachment, err = vz.NewBridgedNetworkDeviceAttachment(itf[0])
		}
		networkConfig, err := newVirtioNetworkDeviceConfiguration(attachment, nw.MACAddress)
		if err != nil {
			return err
		}
		configurations = append(configurations, networkConfig)
	}
	vmConfig.SetNetworkDevicesVirtualMachineConfiguration(configurations)
	return nil
}

func validateDiskFormat(diskPath string) error {
	f, err := os.Open(diskPath)
	if err != nil {
		return err
	}
	defer f.Close()
	img, err := qcow2reader.Open(f)
	if err != nil {
		return fmt.Errorf("failed to detect the format of %q: %w", diskPath, err)
	}
	if t := img.Type(); t != raw.Type {
		return fmt.Errorf("expected the format of %q to be %q, got %q", diskPath, raw.Type, t)
	}
	// TODO: ensure that the disk is formatted with GPT or ISO9660
	return nil
}

func attachDisks(driver *backend.BaseDriver, vmConfig *vz.VirtualMachineConfiguration) error {
	baseDiskPath := filepath.Join(driver.I.Dir(), v1.BaseDisk)
	diffDiskPath := filepath.Join(driver.I.Dir(), v1.DiffDisk)
	ciDataPath := filepath.Join(driver.I.Dir(), v1.CIDataISO)
	isBaseDiskCDROM, err := iso9660util.IsISO9660(baseDiskPath)
	if err != nil {
		return err
	}
	var configurations []vz.StorageDeviceConfiguration

	if isBaseDiskCDROM {
		if err = validateDiskFormat(baseDiskPath); err != nil {
			return err
		}
		baseDiskAttachment, err := vz.NewDiskImageStorageDeviceAttachment(baseDiskPath, true)
		if err != nil {
			return err
		}
		baseDisk, err := vz.NewUSBMassStorageDeviceConfiguration(baseDiskAttachment)
		if err != nil {
			return err
		}
		configurations = append(configurations, baseDisk)
	}
	if err = validateDiskFormat(diffDiskPath); err != nil {
		return err
	}
	diffDiskAttachment, err := vz.NewDiskImageStorageDeviceAttachmentWithCacheAndSync(diffDiskPath, false, diskImageCachingMode, vz.DiskImageSynchronizationModeFsync)
	if err != nil {
		return err
	}
	diffDisk, err := vz.NewVirtioBlockDeviceConfiguration(diffDiskAttachment)
	if err != nil {
		return err
	}
	configurations = append(configurations, diffDisk)

	for _, d := range driver.I.Spec.AdditionalDisks {
		diskName := d.Name
		disk, err := driver.I.InspectDisk(diskName)
		if err != nil {
			return fmt.Errorf("failed to run load disk %q: %w", diskName, err)
		}

		if disk.Instance != "" {
			return fmt.Errorf("failed to run attach disk %q, in use by instance %q", diskName, disk.Instance)
		}
		klog.Infof("Mounting disk %q on %q", diskName, disk.MountPoint)
		err = disk.Lock(driver.I.Dir())
		if err != nil {
			return fmt.Errorf("failed to run lock disk %q: %w", diskName, err)
		}
		extraDiskPath := filepath.Join(disk.Dir, v1.DataDisk)
		// ConvertToRaw is a NOP if no conversion is needed
		klog.Infof("Converting extra disk %q to a raw disk (if it is not a raw)", extraDiskPath)
		if err = nativeimgutil.ConvertToRaw(extraDiskPath, extraDiskPath, nil, true); err != nil {
			return fmt.Errorf("failed to convert extra disk %q to a raw disk: %w", extraDiskPath, err)
		}
		extraDiskPathAttachment, err := vz.NewDiskImageStorageDeviceAttachmentWithCacheAndSync(extraDiskPath, false, diskImageCachingMode, vz.DiskImageSynchronizationModeFsync)
		if err != nil {
			return fmt.Errorf("failed to create disk attachment for extra disk %q: %w", extraDiskPath, err)
		}
		extraDisk, err := vz.NewVirtioBlockDeviceConfiguration(extraDiskPathAttachment)
		if err != nil {
			return fmt.Errorf("failed to create new virtio block device config for extra disk %q: %w", extraDiskPath, err)
		}
		configurations = append(configurations, extraDisk)
	}

	if err = validateDiskFormat(ciDataPath); err != nil {
		return err
	}
	ciDataAttachment, err := vz.NewDiskImageStorageDeviceAttachment(ciDataPath, true)
	if err != nil {
		return err
	}
	ciData, err := vz.NewVirtioBlockDeviceConfiguration(ciDataAttachment)
	if err != nil {
		return err
	}
	configurations = append(configurations, ciData)

	vmConfig.SetStorageDevicesVirtualMachineConfiguration(configurations)
	return nil
}

func attachDisplay(driver *backend.BaseDriver, vmConfig *vz.VirtualMachineConfiguration) error {
	switch driver.I.Spec.Video.Display {
	case "vz", "default":
		graphicsDeviceConfiguration, err := vz.NewVirtioGraphicsDeviceConfiguration()
		if err != nil {
			return err
		}
		scanoutConfiguration, err := vz.NewVirtioGraphicsScanoutConfiguration(1920, 1200)
		if err != nil {
			return err
		}
		graphicsDeviceConfiguration.SetScanouts(scanoutConfiguration)

		vmConfig.SetGraphicsDevicesVirtualMachineConfiguration([]vz.GraphicsDeviceConfiguration{
			graphicsDeviceConfiguration,
		})
		return nil
	case "none":
		return nil
	default:
		return fmt.Errorf("unexpected video display %q", driver.I.Spec.Video.Display)
	}
}

func attachFolderMounts(driver *backend.BaseDriver, vmConfig *vz.VirtualMachineConfiguration) error {
	var mounts []vz.DirectorySharingDeviceConfiguration

	for i, mount := range driver.I.Spec.Mounts {
		klog.Infof("process vz mount      : %s, %s, %t, %s", mount.Location, mount.MountPoint, mount.Writable, mount.MountType)
		expandedPath, err := v1.Expand(mount.Location)
		if err != nil {
			return err
		}
		if _, err := os.Stat(expandedPath); errors.Is(err, os.ErrNotExist) {
			err := os.MkdirAll(expandedPath, 0o750)
			if err != nil {
				return err
			}
		}

		directory, err := vz.NewSharedDirectory(expandedPath, !mount.Writable)
		if err != nil {
			return err
		}
		share, err := vz.NewSingleDirectoryShare(directory)
		if err != nil {
			return err
		}

		tag := fmt.Sprintf("mount%d", i)
		config, err := vz.NewVirtioFileSystemDeviceConfiguration(tag)
		if err != nil {
			return err
		}
		config.SetDirectoryShare(share)
		mounts = append(mounts, config)

		klog.Infof("process vz mount added: %s, %s, %t, %s", mount.Location, mount.MountPoint, mount.Writable, mount.MountType)
	}

	if len(mounts) > 0 {
		vmConfig.SetDirectorySharingDevicesVirtualMachineConfiguration(mounts)
	}
	return nil
}

func attachAudio(driver *backend.BaseDriver, config *vz.VirtualMachineConfiguration) error {
	switch driver.I.Spec.Audio.Device {
	case "vz", "default":
		outputStream, err := vz.NewVirtioSoundDeviceHostOutputStreamConfiguration()
		if err != nil {
			return err
		}
		soundDeviceConfiguration, err := vz.NewVirtioSoundDeviceConfiguration()
		if err != nil {
			return err
		}
		soundDeviceConfiguration.SetStreams(outputStream)
		config.SetAudioDevicesVirtualMachineConfiguration([]vz.AudioDeviceConfiguration{
			soundDeviceConfiguration,
		})
		return nil
	case "", "none":
		return nil
	default:
		return fmt.Errorf("unexpected audio device %q", driver.I.Spec.Audio.Device)
	}
}

func attachOtherDevices(_ *backend.BaseDriver, vmConfig *vz.VirtualMachineConfiguration) error {
	entropyConfig, err := vz.NewVirtioEntropyDeviceConfiguration()
	if err != nil {
		return err
	}
	vmConfig.SetEntropyDevicesVirtualMachineConfiguration([]*vz.VirtioEntropyDeviceConfiguration{
		entropyConfig,
	})

	configuration, err := vz.NewVirtioTraditionalMemoryBalloonDeviceConfiguration()
	if err != nil {
		return err
	}
	vmConfig.SetMemoryBalloonDevicesVirtualMachineConfiguration([]vz.MemoryBalloonDeviceConfiguration{
		configuration,
	})

	deviceConfiguration, err := vz.NewVirtioSocketDeviceConfiguration()
	vmConfig.SetSocketDevicesVirtualMachineConfiguration([]vz.SocketDeviceConfiguration{
		deviceConfiguration,
	})
	if err != nil {
		return err
	}

	// Set audio device
	inputAudioDeviceConfig, err := vz.NewVirtioSoundDeviceConfiguration()
	if err != nil {
		return err
	}
	inputStream, err := vz.NewVirtioSoundDeviceHostInputStreamConfiguration()
	if err != nil {
		return err
	}
	inputAudioDeviceConfig.SetStreams(
		inputStream,
	)

	outputAudioDeviceConfig, err := vz.NewVirtioSoundDeviceConfiguration()
	if err != nil {
		return err
	}
	outputStream, err := vz.NewVirtioSoundDeviceHostOutputStreamConfiguration()
	if err != nil {
		return err
	}
	outputAudioDeviceConfig.SetStreams(
		outputStream,
	)
	vmConfig.SetAudioDevicesVirtualMachineConfiguration([]vz.AudioDeviceConfiguration{
		inputAudioDeviceConfig,
		outputAudioDeviceConfig,
	})

	// Set pointing device
	pointingDeviceConfig, err := vz.NewUSBScreenCoordinatePointingDeviceConfiguration()
	if err != nil {
		return err
	}
	vmConfig.SetPointingDevicesVirtualMachineConfiguration([]vz.PointingDeviceConfiguration{
		pointingDeviceConfig,
	})

	// Set keyboard device
	keyboardDeviceConfig, err := vz.NewUSBKeyboardConfiguration()
	if err != nil {
		return err
	}
	vmConfig.SetKeyboardsVirtualMachineConfiguration([]vz.KeyboardConfiguration{
		keyboardDeviceConfig,
	})
	return nil
}

func getMachineIdentifier(driver *backend.BaseDriver) (*vz.GenericMachineIdentifier, error) {
	identifier := filepath.Join(driver.I.Dir(), v1.VzIdentifier)
	if _, err := os.Stat(identifier); os.IsNotExist(err) {
		machineIdentifier, err := vz.NewGenericMachineIdentifier()
		if err != nil {
			return nil, err
		}
		err = os.WriteFile(identifier, machineIdentifier.DataRepresentation(), 0o666)
		if err != nil {
			return nil, err
		}
		return machineIdentifier, nil
	}
	return vz.NewGenericMachineIdentifierWithDataPath(identifier)
}

func getEFI(driver *backend.BaseDriver) (*vz.EFIVariableStore, error) {
	efi := filepath.Join(driver.I.Dir(), v1.VzEfi)
	if _, err := os.Stat(efi); os.IsNotExist(err) {
		return vz.NewEFIVariableStore(efi, vz.WithCreatingEFIVariableStore())
	}
	return vz.NewEFIVariableStore(efi)
}

func createSockPair() (server, client *os.File, _ error) {
	pairs, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_DGRAM, 0)
	if err != nil {
		return nil, nil, err
	}
	serverFD := pairs[0]
	clientFD := pairs[1]

	if err = syscall.SetsockoptInt(serverFD, syscall.SOL_SOCKET, syscall.SO_SNDBUF, 1*1024*1024); err != nil {
		return nil, nil, err
	}
	if err = syscall.SetsockoptInt(serverFD, syscall.SOL_SOCKET, syscall.SO_RCVBUF, 4*1024*1024); err != nil {
		return nil, nil, err
	}
	if err = syscall.SetsockoptInt(clientFD, syscall.SOL_SOCKET, syscall.SO_SNDBUF, 1*1024*1024); err != nil {
		return nil, nil, err
	}
	if err = syscall.SetsockoptInt(clientFD, syscall.SOL_SOCKET, syscall.SO_RCVBUF, 4*1024*1024); err != nil {
		return nil, nil, err
	}
	server = os.NewFile(uintptr(serverFD), "server")
	client = os.NewFile(uintptr(clientFD), "client")
	runtime.SetFinalizer(server, func(*os.File) {
		klog.Info("Server network file GC'ed")
	})
	runtime.SetFinalizer(client, func(*os.File) {
		klog.Info("Client network file GC'ed")
	})
	vmNetworkFiles = append(vmNetworkFiles, server, client)
	return server, client, nil
}
