package v1

import (
	"errors"
	"fmt"
	"k8s.io/klog/v2"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/docker/go-units"
)

// Expand expands a path like "~", "~/", "~/foo".
// Paths like "~foo/bar" are unsupported.
//
// FIXME: is there an existing library for this?
func Expand(orig string) (string, error) {
	s := orig
	if s == "" {
		return "", errors.New("empty path")
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	if strings.HasPrefix(s, "~") {
		if s == "~" || strings.HasPrefix(s, "~/") {
			s = strings.Replace(s, "~", homeDir, 1)
		} else {
			// Paths like "~foo/bar" are unsupported.
			return "", fmt.Errorf("unexpandable path %q", orig)
		}
	}
	return filepath.Abs(s)
}

func validateFileObject(f File, fieldName string) error {
	if !strings.Contains(f.Location, "://") {
		if _, err := Expand(f.Location); err != nil {
			return fmt.Errorf("field `%s.location` refers to an invalid local file path: %q: %w", fieldName, f.Location, err)
		}
		// f.Location does NOT need to be accessible, so we do NOT check os.Stat(f.Location)
	}
	switch f.Arch {
	case X8664, AARCH64, ARMV7L, RISCV64:
	default:
		return fmt.Errorf("field `arch` must be %q, %q, %q, or %q; got %q", X8664, AARCH64, ARMV7L, RISCV64, f.Arch)
	}
	if f.Digest != "" {
		if !f.Digest.Algorithm().Available() {
			return fmt.Errorf("field `%s.digest` refers to an unavailable digest algorithm", fieldName)
		}
		if err := f.Digest.Validate(); err != nil {
			return fmt.Errorf("field `%s.digest` is invalid: %s: %w", fieldName, f.Digest.String(), err)
		}
	}
	return nil
}

func ValidateYAML(y *VirtualMachine, warn bool) error {
	switch y.Spec.OS {
	case LINUX:
	default:
		return fmt.Errorf("field `os` must be %q; got %q", LINUX, y.Spec.OS)
	}
	switch y.Spec.Arch {
	case X8664, AARCH64, ARMV7L, RISCV64:
	default:
		return fmt.Errorf("field `arch` must be %q, %q, %q or %q; got %q", X8664, AARCH64, ARMV7L, RISCV64, y.Spec.Arch)
	}

	switch y.Spec.VMType {
	case QEMU:
		// NOP
	case WSL2:
		// NOP
	case VZ:
		if !IsNativeArch(y.Spec.Arch) {
			return fmt.Errorf("field `arch` must be %q for VZ; got %q", NewArch(runtime.GOARCH), y.Spec.Arch)
		}
	default:
		return fmt.Errorf("field `vmType` must be %q, %q, %q; got %q", QEMU, VZ, WSL2, y.Spec.VMType)
	}
	f := y.Spec.Image
	//if f.Key == "" {
	//	return fmt.Errorf("unexpected empty image name")
	//}
	if f.Kernel != nil {
		if err := validateFileObject(f.Kernel.File, "images.kernel"); err != nil {
			return err
		}
		if f.Kernel.Arch != y.Spec.Arch {
			return fmt.Errorf("images.kernel has unexpected architecture %q, must be %q", f.Kernel.Arch, y.Spec.Arch)
		}
	} else if y.Spec.Arch == RISCV64 {
		return errors.New("riscv64 needs the kernel (e.g., \"uboot.elf\") to be specified")
	}
	if f.Initrd != nil {
		if err := validateFileObject(*f.Initrd, "images.initrd"); err != nil {
			return err
		}
		if f.Kernel == nil {
			return errors.New("initrd requires the kernel to be specified")
		}
		if f.Initrd.Arch != y.Spec.Arch {
			return fmt.Errorf("images.initrd has unexpected architecture %q, must be %q", f.Initrd.Arch, y.Spec.Arch)
		}
	}

	if y.Spec.CPUs == 0 {
		return errors.New("field `cpus` must be set")
	}

	if _, err := units.RAMInBytes(y.Spec.Memory); err != nil {
		return fmt.Errorf("field `memory` has an invalid value: %w", err)
	}

	if _, err := units.RAMInBytes(y.Spec.Disk); err != nil {
		return fmt.Errorf("field `memory` has an invalid value: %w", err)
	}

	for i, f := range y.Spec.Mounts {
		if !filepath.IsAbs(f.Location) && !strings.HasPrefix(f.Location, "~") {
			return fmt.Errorf("field `mounts[%d].location` must be an absolute path, got %q",
				i, f.Location)
		}
		loc, err := Expand(f.Location)
		if err != nil {
			return fmt.Errorf("field `mounts[%d].location` refers to an unexpandable path: %q: %w", i, f.Location, err)
		}
		switch loc {
		case "/", "/bin", "/dev", "/etc", "/home", "/opt", "/sbin", "/tmp", "/usr", "/var":
			return fmt.Errorf("field `mounts[%d].location` must not be a system path such as /etc or /usr", i)
		}

		st, err := os.Stat(loc)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("field `mounts[%d].location` refers to an inaccessible path: %q: %w", i, f.Location, err)
			}
		} else if !st.IsDir() {
			return fmt.Errorf("field `mounts[%d].location` refers to a non-directory path: %q: %w", i, f.Location, err)
		}

	}

	if y.Spec.SSH.LocalPort != 0 {
		if err := validatePort("ssh.localPort", y.Spec.SSH.LocalPort); err != nil {
			return err
		}
	}

	// y.Firmware.LegacyBIOS is ignored for aarch64, but not a fatal error.

	if y.Spec.HostResolver.Enabled && len(y.Spec.DNS) > 0 {
		return fmt.Errorf("field `dns` must be empty when field `HostResolver.Enabled` is true")
	}

	if err := validateNetwork(y); err != nil {
		return err
	}
	if warn {
		warnExperimental(y)
	}
	return nil
}

func validateNetwork(y *VirtualMachine) error {
	if len(y.Spec.Networks) == 0 {
		network := []Network{
			{
				VZNAT:      true,
				Interface:  "enp0s1",
				MACAddress: GenMAC(),
			},
		}
		y.Spec.Networks = network
	}
	if y.Spec.Networks[0].VZNAT {
		if y.Spec.Networks[0].MACAddress == "" {
			y.Spec.Networks[0].MACAddress = GenMAC()
		}
	}
	klog.Infof("[%-10s]new network address: %s", y.Name, y.Spec.Networks[0].MACAddress)
	return nil
}

// ValidateParamIsUsed checks if the keys in the `param` field are used in any script, probe, copyToHost, or portForward.
// It should be called before the `y` parameter is passed to setDefault() that execute template.
func ValidateParamIsUsed(y *VirtualMachine) error {

	return nil
}

func validatePort(field string, port int) error {
	switch {
	case port < 0:
		return fmt.Errorf("field `%s` must be > 0", field)
	case port == 0:
		return fmt.Errorf("field `%s` must be set", field)
	case port == 22:
		return fmt.Errorf("field `%s` must not be 22", field)
	case port > 65535:
		return fmt.Errorf("field `%s` must be < 65536", field)
	}
	return nil
}

func warnExperimental(y *VirtualMachine) {

	if y.Spec.VMType == VZ {
		klog.Warningf("`vmType: vz` is experimental")
	}
	if y.Spec.Arch == RISCV64 {
		klog.Warningf("`arch: riscv64` is experimental")
	}
	if strings.Contains(y.Spec.Video.Display, "vnc") {
		klog.Warningf("`video.display: vnc` is experimental")
	}
	if y.Spec.Audio.Device != "" {
		klog.Warningf("`audio.device` is experimental")
	}
	if y.Spec.MountInotify {
		klog.Warningf("`mountInotify` is experimental")
	}
}
