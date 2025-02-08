package v1

import (
	_ "embed"
	"encoding/xml"
	"github.com/pkg/errors"
	"io"
	"k8s.io/klog/v2"
	"os/exec"
	"sync"

	"bytes"
	"crypto/sha256"
	"fmt"
	"github.com/docker/go-units"
	"github.com/ghodss/yaml"
	"github.com/google/uuid"
	"github.com/pbnjay/memory"
	"golang.org/x/sys/cpu"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	// Default9pSecurityModel is "none" for supporting symlinks
	// https://gitlab.com/qemu-project/qemu/-/issues/173
	Default9pSecurityModel   string = "none"
	Default9pProtocolVersion string = "9p2000.L"
	Default9pMsize           string = "128KiB"
	Default9pCacheForRO      string = "fscache"
	Default9pCacheForRW      string = "mmap"

	DefaultVirtiofsQueueSize int = 1024
)

var IPv4loopback1 = net.IPv4(127, 0, 0, 1)

//go:embed default.yaml
var dftYaml []byte

//go:embed baseline.yaml
var baseLine []byte

func FindGuestBin(name, os, arch string) *File {
	base := &BaseLine{}
	err := yaml.Unmarshal(baseLine, base)
	if err != nil {
		return nil
	}
	for i, _ := range base.GuestBin {
		image := &base.GuestBin[i]
		if strings.ToLower(image.OS) != strings.ToLower(os) {
			continue
		}
		if strings.ToLower(string(image.Arch)) != strings.ToLower(arch) {
			continue
		}
		return image
	}
	return nil
}

func DftImages() []File {
	base := &BaseLine{}
	err := yaml.Unmarshal(baseLine, base)
	if err != nil {
		return nil
	}
	return base.Images
}

func FindDftImageBy(os, arch string) *File {
	base := &BaseLine{}
	err := yaml.Unmarshal(baseLine, base)
	if err != nil {
		return nil
	}
	for i, _ := range base.Images {
		image := &base.Images[i]
		if strings.ToLower(image.OS) != strings.ToLower(os) {
			continue
		}
		if strings.ToLower(string(image.Arch)) != strings.ToLower(arch) {
			continue
		}
		return image
	}
	return nil
}

func FindImage(name string) *File {
	base := &BaseLine{}
	err := yaml.Unmarshal(baseLine, base)
	if err != nil {
		return nil
	}
	for i, _ := range base.Images {
		image := &base.Images[i]
		if image.Name != name {
			continue
		}
		return image
	}
	return nil
}

func FindBinary(bin string, arch Arch) (File, error) {
	base := &BaseLine{}
	err := yaml.Unmarshal(baseLine, base)
	if err != nil {
		return File{}, err
	}
	var tab []File
	switch bin {
	case "kubectl":
		tab = base.Kubectl
	case "docker":
		tab = base.Docker
	}
	for _, f := range tab {
		if f.Arch != arch {
			continue
		}
		if f.OS != runtime.GOOS {
			continue
		}
		return f, nil
	}
	return File{}, fmt.Errorf("NotFound: %s %s, %s", bin, runtime.GOOS, arch)
}

// LoadDft loads the yaml and fulfills unspecified fields with the default values.
//
// Load does not validate. Use Validate for validation.
func LoadDft() (*VirtualMachine, error) {
	var y VirtualMachine
	// We need to use the absolute path because it may be used to determine hostSocket locations.
	if err := yaml.Unmarshal(dftYaml, &y); err != nil {
		return nil, err
	}
	// It should be called before the `y` parameter is passed to setDefault() that execute template.
	if err := ValidateParamIsUsed(&y); err != nil {
		return nil, err
	}

	setDefault(y.Name, &y, nil)

	//return &y, ValidateYAML(&y, false)
	return &y, nil
}

func EnsureYAML(instDir string, yByte []byte, merge bool) error {
	cfgPath := filepath.Join(instDir, MeridianYAMLFile)
	absPath, err := filepath.Abs(cfgPath)
	if err != nil {
		return err
	}
	var y, d VirtualMachine
	bytes, err := os.ReadFile(absPath)
	if err == nil && !merge {
		// already exist, and not merge
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	klog.Infof("merge meridian vm config: %s", absPath)
	if len(yByte) == 0 {
		yByte = dftYaml
	}
	if len(bytes) == 0 {
		bytes = dftYaml
	}
	if err := yaml.Unmarshal(bytes, &y); err != nil {
		return err
	}
	if err := yaml.Unmarshal(yByte, &d); err != nil {
		return err
	}

	setDefault(instDir, &y, &d)

	if err := ValidateYAML(&y, true); err != nil {
		return err
	}
	err = os.MkdirAll(filepath.Dir(cfgPath), 0o700)
	if err != nil {
		return err
	}
	return os.WriteFile(cfgPath, yByte, 0o700)
}

// Load loads the yaml and fulfills unspecified fields with the default values.
//
// Load does not validate. Use Validate for validation.
func Load(file string) (*VirtualMachine, error) {
	var y VirtualMachine
	// We need to use the absolute path because it may be used to determine hostSocket locations.
	absPath, err := filepath.Abs(file)
	if err != nil {
		return nil, err
	}

	bytes, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(bytes, &y); err != nil {
		return nil, err
	}
	// It should be called before the `y` parameter is passed to setDefault() that execute template.
	if err := ValidateParamIsUsed(&y); err != nil {
		return nil, err
	}

	setDefault(file, &y, nil)

	return &y, ValidateYAML(&y, false)
}

// Save saves the yaml.
//
// Save does not fill defaults. Use FillDefaults.
func Save(y *VirtualMachine, filePath string) error {
	content, err := yaml.Marshal(y)
	if err != nil {
		return err
	}
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return err
	}

	return os.WriteFile(absPath, content, 0o700)
}

// setDefault updates undefined fields in y with defaults from d (or built-in default), and overwrites with values from o.
// Both d and o may be empty.
//
// Maps (`Env`) are being merged: first populated from d, overwritten by y, and again overwritten by o.
// Slices (e.g. `Mounts`, `Provision`) are appended, starting with o, followed by y, and finally d. This
// makes sure o takes priority over y over d, in cases it matters (e.g. `PortForwards`, where the first
// matching rule terminates the search).
//
// Exceptions:
//   - Mounts are appended in d, y, o order, but "merged" when the Location matches a previous entry;
//     the highest priority Writable setting wins.
//   - Networks are appended in d, y, o order
//   - XdpDomain are picked from the highest priority where XdpDomain is not empty.
//   - CACertificates Files and Certs are uniquely appended in d, y, o order
func setDefault(idx string, y, d *VirtualMachine) {
}

func defaultCPUType() CPUType {
	cpuType := map[Arch]string{
		AARCH64: "cortex-a72",
		ARMV7L:  "cortex-a7",
		// Since https://github.com/lima-vm/lima/pull/494, we use qemu64 cpu for better emulation of x86_64.
		X8664:   "qemu64",
		RISCV64: "rv64", // FIXME: what is the right choice for riscv64?
	}
	for arch := range cpuType {
		if IsNativeArch(arch) && IsAccelOS() {
			if HasHostCPU() {
				cpuType[arch] = "host"
			} else if HasMaxCPU() {
				cpuType[arch] = "max"
			}
		}
		if arch == X8664 && runtime.GOOS == "darwin" {
			switch cpuType[arch] {
			case "host", "max":
				// Disable pdpe1gb on Intel Mac
				// https://github.com/lima-vm/lima/issues/1485
				// https://stackoverflow.com/a/72863744/5167443
				cpuType[arch] += ",-pdpe1gb"
			}
		}
	}
	return cpuType
}

func defaultContainerdArchives() []File {
	const nerdctlVersion = "1.7.6"
	location := func(goos string, goarch string) string {
		return "https://github.com/containerd/nerdctl/releases/download/v" + nerdctlVersion + "/nerdctl-full-" + nerdctlVersion + "-" + goos + "-" + goarch + ".tar.gz"
	}
	return []File{
		{
			Location: location("linux", "amd64"),
			Arch:     X8664,
			Digest:   "sha256:2c841e097fcfb5a1760bd354b3778cb695b44cd01f9f271c17507dc4a0b25606",
		},
		{
			Location: location("linux", "arm64"),
			Arch:     AARCH64,
			Digest:   "sha256:77c747f09853ee3d229d77e8de0dd3c85622537d82be57433dc1fca4493bab95",
		},
		// No arm-v7
		// No riscv64
	}
}

func GenMAC() string {
	id, err := uuid.NewRandom()
	if err != nil {
		panic(err)
	}
	sha := sha256.Sum256([]byte(id.String()))
	hw := append(net.HardwareAddr{0x52, 0x55, 0x55}, sha[0:3]...)
	return hw.String()
}

func MACAddress(uniqueID string) string {
	sha := sha256.Sum256([]byte(uniqueID))
	// "5" is the magic number in the Lima ecosystem.
	// (Visit https://en.wiktionary.org/wiki/lima and Command-F "five")
	//
	// But the second hex number is changed to 2 to satisfy the convention for
	// local MAC addresses (https://en.wikipedia.org/wiki/MAC_address#Ranges_of_group_and_locally_administered_addresses)
	//
	// See also https://gitlab.com/wireshark/wireshark/-/blob/release-4.0/manuf to confirm the uniqueness of this prefix.
	hw := append(net.HardwareAddr{0x52, 0x55, 0x55}, sha[0:3]...)
	klog.V(5).Infof("[%-10s]current mac address: %s", uniqueID, hw.String())
	return hw.String()
}

var (
	machineIDCached string
	machineIDOnce   sync.Once
)

func MachineID() string {
	machineIDOnce.Do(func() {
		x, err := machineID()
		if err == nil && x != "" {
			machineIDCached = x
			return
		}
		klog.Warningf("failed to get machine ID, falling back to use hostname instead")
		hostname, err := os.Hostname()
		if err != nil {
			panic(err)
		}
		machineIDCached = hostname
	})
	return machineIDCached
}

func machineID() (string, error) {
	if runtime.GOOS == "darwin" {
		ioPlatformExpertDeviceCmd := exec.Command("/usr/sbin/ioreg", "-a", "-d2", "-c", "IOPlatformExpertDevice")
		ioPlatformExpertDevice, err := ioPlatformExpertDeviceCmd.CombinedOutput()
		if err != nil {
			return "", err
		}
		return parseIOPlatformUUIDFromIOPlatformExpertDevice(bytes.NewReader(ioPlatformExpertDevice))
	}

	candidates := []string{
		"/etc/machine-id",
		"/var/lib/dbus/machine-id",
		// We don't use "/sys/class/dmi/id/product_uuid"
	}
	for _, f := range candidates {
		b, err := os.ReadFile(f)
		if err == nil {
			return strings.TrimSpace(string(b)), nil
		}
	}
	return "", fmt.Errorf("no machine-id found, tried %v", candidates)
}

func parseIOPlatformUUIDFromIOPlatformExpertDevice(r io.Reader) (string, error) {
	d := xml.NewDecoder(r)
	var (
		elem            string
		elemKeyCharData string
	)
	for {
		tok, err := d.Token()
		if err != nil {
			return "", err
		}
		switch v := tok.(type) {
		case xml.StartElement:
			elem = v.Name.Local
		case xml.EndElement:
			elem = ""
			if v.Name.Local != "key" {
				elemKeyCharData = ""
			}
		case xml.CharData:
			if elem == "string" && elemKeyCharData == "IOPlatformUUID" {
				return string(v), nil
			}
			if elem == "key" {
				elemKeyCharData = string(v)
			}
		}
	}
}

func hostTimeZone() string {
	// WSL2 will automatically set the timezone
	if runtime.GOOS != "windows" {
		tz, err := os.ReadFile("/etc/timezone")
		if err == nil {
			return strings.TrimSpace(string(tz))
		}
		zoneinfoFile, err := filepath.EvalSymlinks("/etc/localtime")
		if err == nil {
			for baseDir := filepath.Dir(zoneinfoFile); baseDir != "/"; baseDir = filepath.Dir(baseDir) {
				if _, err = os.Stat(filepath.Join(baseDir, "Etc/UTC")); err == nil {
					return strings.TrimPrefix(zoneinfoFile, baseDir+"/")
				}
			}
			klog.Warningf("could not locate zoneinfo directory from %q", zoneinfoFile)
		}
	}
	return ""
}

func defaultCPUs() int {
	const x = 4
	if hostCPUs := runtime.NumCPU(); hostCPUs < x {
		return hostCPUs
	}
	return x
}

func defaultMemory() uint64 {
	const x uint64 = 4 * 1024 * 1024 * 1024
	if halfOfHostMemory := memory.TotalMemory() / 2; halfOfHostMemory < x {
		return halfOfHostMemory
	}
	return x
}

func defaultMemoryAsString() string {
	return units.BytesSize(float64(defaultMemory()))
}

func defaultDiskSizeAsString() string {
	// currently just hardcoded
	return "100GiB"
}

func defaultGuestInstallPrefix() string {
	return "/usr/local"
}

func NewOS(osname string) OS {
	switch osname {
	case "linux":
		return LINUX
	default:
		klog.Warningf("Unknown os: %s", osname)
		return OS(osname)
	}
}

func Exist(f string) (bool, error) {
	_, err := os.Stat(f)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func goarm() int {
	if runtime.GOOS != "linux" {
		return 0
	}
	if runtime.GOARCH != "arm" {
		return 0
	}
	if cpu.ARM.HasVFPv3 {
		return 7
	}
	if cpu.ARM.HasVFP {
		return 6
	}
	return 5 // default
}

func NewArch(arch string) Arch {
	switch arch {
	case "amd64":
		return X8664
	case "arm64":
		return AARCH64
	case "arm":
		arm := goarm()
		if arm == 7 {
			return ARMV7L
		}
		klog.Warningf("Unknown arm: %d", arm)
		return Arch(arch)
	case "riscv64":
		return RISCV64
	default:
		klog.Infof("Unknown arch: %s", arch)
		return Arch(arch)
	}
}

func NewVMType(driver string) VMType {
	switch driver {
	case "vz":
		return VZ
	case "qemu":
		return QEMU
	case "wsl2":
		return WSL2
	default:
		klog.Infof("Unknown driver: %s", driver)
		return VMType(driver)
	}
}

func ResolveVMType(s *string) VMType {
	if s == nil || *s == "" || *s == "default" {
		return QEMU
	}
	return NewVMType(*s)
}

func ResolveOS(s *string) OS {
	if s == nil || *s == "" || *s == "default" {
		return NewOS("linux")
	}
	return OS(*s)
}

func ResolveArch(s *string) Arch {
	if s == nil || *s == "" || *s == "default" {
		return NewArch(runtime.GOARCH)
	}
	return Arch(*s)
}

func IsAccelOS() bool {
	switch runtime.GOOS {
	case "darwin", "linux", "netbsd", "windows":
		// Accelerator
		return true
	}
	// Using TCG
	return false
}

func HasHostCPU() bool {
	switch runtime.GOOS {
	case "darwin", "linux":
		return true
	case "netbsd", "windows":
		return false
	}
	// Not reached
	return false
}

func HasMaxCPU() bool {
	// WHPX: Unexpected VP exit code 4
	return runtime.GOOS != "windows"
}

func IsNativeArch(arch Arch) bool {
	nativeX8664 := arch == X8664 && runtime.GOARCH == "amd64"
	nativeAARCH64 := arch == AARCH64 && runtime.GOARCH == "arm64"
	nativeARMV7L := arch == ARMV7L && runtime.GOARCH == "arm" && goarm() == 7
	nativeRISCV64 := arch == RISCV64 && runtime.GOARCH == "riscv64"
	return nativeX8664 || nativeAARCH64 || nativeARMV7L || nativeRISCV64
}

func unique(s []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range s {
		if _, found := keys[entry]; !found {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}
