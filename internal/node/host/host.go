package host

import (
	"fmt"
	"github.com/aoxn/meridian/internal/node/host/meta"
	"github.com/aoxn/meridian/internal/tool/cmd"
	"github.com/pkg/errors"
	"os"
	"runtime"
	"strings"
)

func NewLocalHost(
	meta meta.Meta,
) (*Local, error) {
	id, err := meta.InstanceID()
	if err != nil {
		return nil, errors.Wrap(err, "get local id")
	}
	ip, err := meta.PrivateIPv4()
	if err != nil {
		return nil, errors.Wrap(err, "get local ip")
	}
	os := "ubuntu"
	switch runtime.GOOS {
	case "darwin":
		return nil, fmt.Errorf("darwin not supported yet")
	case "linux":
		os, err = getRelease()
		if err != nil {
			return nil, errors.Wrap(err, "get local os")
		}
	case "windows":
		return nil, fmt.Errorf("windows not yet supported")
	}

	region, err := meta.Region()
	if err != nil {
		return nil, errors.Wrap(err, "get local region")
	}
	return &Local{
		os:     os,
		ip:     ip,
		id:     id,
		region: region,
		arch:   runtime.GOARCH,
		meta:   meta,
	}, nil
}

func getRelease() (string, error) {
	content, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "", errors.Wrap(err, "read os-release")
	}
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			continue
		}
		if strings.TrimSpace(kv[0]) != "ID" {
			continue
		}
		if strings.Contains(strings.ToLower(kv[1]), "centos") {
			return CentOS, nil
		}
		if strings.Contains(strings.ToLower(kv[1]), "alinux") {
			return CentOS, nil
		}
		if strings.Contains(strings.ToLower(kv[1]), "ubuntu") {
			return Ubuntu, nil
		}
	}
	return "", errors.New("release not found")
}

type Host interface {
	ProviderID() string
	NodeID() string
	NodeIP() string
	OS() string
	Arch() string
	Region() string
	Command(bin string, args ...string) (string, error)
	Service() Service
}

type Service interface {
	Start(name string) error
	Stop(name string) error
	Restart(name string) error
	Enable(name string) error
	Disable(name string) error
	DaemonReload() error
}

type LinuxService struct {
}

func (v *LinuxService) Start(name string) error {
	return cmd.Systemctl([]string{"start", name})
}

func (v *LinuxService) Stop(name string) error {
	return cmd.Systemctl([]string{"stop", name})
}

func (v *LinuxService) Restart(name string) error {
	return cmd.Systemctl([]string{"restart", name})
}

func (v *LinuxService) Enable(name string) error {
	return cmd.Systemctl([]string{"enable", name})
}

func (v *LinuxService) Disable(name string) error {
	return cmd.Systemctl([]string{"disable", name})
}
func (v *LinuxService) DaemonReload() error {
	return cmd.Systemctl([]string{"daemon-reload"})
}

type Local struct {
	ip     string
	os     string
	id     string
	arch   string
	region string
	meta   meta.Meta
}

func (i *Local) NodeID() string {
	return strings.ToLower(i.id)
}

func (i *Local) ProviderID() string {
	if i.region != "" && i.id != "" {
		return i.region + "." + i.id
	}
	return strings.ToLower(i.id)
}

func (i *Local) NodeIP() string { return i.ip }

func (i *Local) Region() string { return i.region }

func (i *Local) Arch() string { return i.arch }

func (i *Local) OS() string { return i.os }

func (i *Local) Command(bin string, args ...string) (string, error) {
	extract := <-cmd.NewCmd(bin, args...).Start()
	return cmd.CmdResult(extract)
}

func (i *Local) Service() Service {
	switch i.os {
	case Ubuntu, CentOS:
		return &LinuxService{}
	default:
	}
	panic(fmt.Sprintf("unimplemented os service for [%s]", i.os))
}

const (
	CentOS  = "centos"
	Ubuntu  = "ubuntu"
	Windows = "windows"
)
