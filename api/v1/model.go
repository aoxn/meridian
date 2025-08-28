package v1

import (
	"fmt"
	"github.com/opencontainers/go-digest"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type (
	OS        string
	Arch      string
	MountType string
	VMType    string
)

type CPUType map[Arch]string

const (
	XDPIN_BACKUP       = "xdpin.cn/mark"
	MERIDIAN_NODEGROUP = "xdpin.cn/nodegroup"
)

const (
	LINUX OS = "Linux"

	X8664   Arch = "x86_64"
	AARCH64 Arch = "aarch64"
	ARMV7L  Arch = "armv7l"
	RISCV64 Arch = "riscv64"

	REVSSHFS MountType = "reverse-sshfs"
	NINEP    MountType = "9p"
	VIRTIOFS MountType = "virtiofs"
	WSLMount MountType = "wsl2"

	QEMU VMType = "qemu"
	VZ   VMType = "vz"
	WSL2 VMType = "wsl2"
)

type BaseLine struct {
	Docker   []File `yaml:"docker,omitempty" json:"docker,omitempty"`
	Kubectl  []File `yaml:"kubectl,omitempty" json:"kubectl,omitempty"`
	Images   []File `yaml:"images,omitempty" json:"images,omitempty"`
	GuestBin []File `yaml:"guestBin,omitempty" json:"guestBin,omitempty"`
}

type File struct {
	Name     string            `yaml:"name" json:"name"`
	Location string            `yaml:"location" json:"location"` // REQUIRED
	OS       string            `yaml:"os" json:"os"`
	Arch     Arch              `yaml:"arch,omitempty" json:"arch,omitempty"`
	Digest   digest.Digest     `yaml:"digest,omitempty" json:"digest,omitempty"`
	Labels   map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
}

type FileWithVMType struct {
	File   `yaml:",inline"`
	VMType VMType `yaml:"vmType,omitempty" json:"vmType,omitempty"`
}

type Kernel struct {
	File    `yaml:",inline"`
	Cmdline string `yaml:"cmdline,omitempty" json:"cmdline,omitempty"`
}

type ImageLocation struct {
	// File   `yaml:",inline"`
	Name   string  `yaml:"name" json:"name"`
	Kernel *Kernel `yaml:"kernel,omitempty" json:"kernel,omitempty"`
	Initrd *File   `yaml:"initrd,omitempty" json:"initrd,omitempty"`
}

type Disk struct {
	Name   string   `yaml:"name" json:"name"` // REQUIRED
	Format *bool    `yaml:"format,omitempty" json:"format,omitempty"`
	FSType *string  `yaml:"fsType,omitempty" json:"fsType,omitempty"`
	FSArgs []string `yaml:"fsArgs,omitempty" json:"fsArgs,omitempty"`
}

type Mount struct {
	MountType  string `yaml:"mountType,omitempty" json:"mountType,omitempty"`
	Location   string `yaml:"location" json:"location"` // REQUIRED
	MountPoint string `yaml:"mountPoint,omitempty" json:"mountPoint,omitempty"`
	Writable   bool   `yaml:"writable,omitempty" json:"writable,omitempty"`
}

type SSH struct {
	LocalPort int `yaml:"localPort,omitempty" json:"localPort,omitempty"`

	// LoadDotSSHPubKeys loads ~/.ssh/*.pub in addition to $LIMA_HOME/_config/user.pub .
	LoadDotSSHPubKeys bool `yaml:"loadDotSSHPubKeys,omitempty" json:"loadDotSSHPubKeys,omitempty"` // default: true
	ForwardAgent      bool `yaml:"forwardAgent,omitempty" json:"forwardAgent,omitempty"`           // default: false
	ForwardX11        bool `yaml:"forwardX11,omitempty" json:"forwardX11,omitempty"`               // default: false
	ForwardX11Trusted bool `yaml:"forwardX11Trusted,omitempty" json:"forwardX11Trusted,omitempty"` // default: false
}

type Firmware struct {
	// LegacyBIOS disables UEFI if set.
	// LegacyBIOS is ignored for aarch64.
	LegacyBIOS bool `yaml:"legacyBIOS,omitempty" json:"legacyBIOS,omitempty"`

	// Images specify UEFI images (edk2-aarch64-code.fd.gz).
	// Defaults to built-in UEFI.
	Images []FileWithVMType `yaml:"images,omitempty" json:"images,omitempty"`
}

type Audio struct {
	// Device is a QEMU audiodev string
	Device string `yaml:"device,omitempty" json:"device,omitempty"`
}

type VNCOptions struct {
	Display string `yaml:"display,omitempty" json:"display,omitempty"`
}

type Video struct {
	// Display is a QEMU display string
	Display string     `yaml:"display,omitempty" json:"display,omitempty"`
	VNC     VNCOptions `yaml:"vnc" json:"vnc"`
}

type Proto string

const (
	TCP Proto = "tcp"
)

type PortForward struct {
	SrcProto string             `yaml:"srcProto" json:"srcProto"`
	SrcAddr  intstr.IntOrString `yaml:"srcAddr" json:"srcAddr"`
	DstProto string             `yaml:"dstProto" json:"dstProto"`
	DstAddr  intstr.IntOrString `yaml:"dstAddr,omitempty" json:"dstAddr,omitempty"`
}

func (p *PortForward) Rule() string {
	return fmt.Sprintf("%s://%s->%s://%v", p.SrcProto, p.SrcAddr.String(), p.DstProto, p.DstAddr.String())
}

type Network struct {
	// `Lima` and `Socket` are mutually exclusive; exactly one is required
	Lima string `yaml:"lima,omitempty" json:"lima,omitempty"`
	// Socket is a QEMU-compatible socket
	Socket string `yaml:"socket,omitempty" json:"socket,omitempty"`
	// VZNAT uses VZNATNetworkDeviceAttachment. Needs VZ. No root privilege is required.
	VZNAT bool `yaml:"vzNAT,omitempty" json:"vzNAT,omitempty"`

	MACAddress string `yaml:"macAddress,omitempty" json:"macAddress,omitempty"`
	Interface  string `yaml:"interface,omitempty" json:"interface,omitempty"`
	Address    string `yaml:"address,omitempty" json:"address,omitempty"`
	IpGateway  string `yaml:"ipGateway,omitempty" json:"ipGateway,omitempty"`
}

type HostResolver struct {
	Enabled bool              `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	IPv6    bool              `yaml:"ipv6,omitempty" json:"ipv6,omitempty"`
	Hosts   map[string]string `yaml:"hosts,omitempty" json:"hosts,omitempty"`
}

type CACertificates struct {
	RemoveDefaults bool     `yaml:"removeDefaults,omitempty" json:"removeDefaults,omitempty"` // default: false
	Files          []string `yaml:"files,omitempty" json:"files,omitempty"`
	Certs          []string `yaml:"certs,omitempty" json:"certs,omitempty"`
}

type Healthy struct {
	Status string `yaml:"status,omitempty" json:"status,omitempty"`
}
