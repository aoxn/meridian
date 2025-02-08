package local

import (
	"fmt"
	"github.com/aoxn/meridian/internal/node/host/meta"
	"github.com/aoxn/meridian/internal/node/host/meta/alibaba"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"
	"net"
	"os"
	"strings"
)

type Config struct {
	ZoneID    string
	Region    string
	VpcID     string
	VswitchID string
}

// NewMetaData return new metadata
func NewMetaData(cfg *Config) meta.Meta {
	if cfg.VpcID != "" &&
		cfg.VswitchID != "" {
		klog.Infof("use mocked metadata server.")
		return &localMetaData{
			config: cfg,
			base:   alibaba.NewMetaDataAlibaba(nil),
		}
	}
	return alibaba.NewMetaDataAlibaba(nil)
}

type localMetaData struct {
	config *Config
	base   meta.Meta
}

func (m *localMetaData) HostName() (string, error) {

	return "", fmt.Errorf("unimplemented")
}

func (m *localMetaData) ImageID() (string, error) {

	return "", fmt.Errorf("unimplemented")
}

func (m *localMetaData) InstanceID() (string, error) {
	host, err := os.Hostname()
	if err != nil {
		return host, err
	}
	return fmt.Sprintf("xdpin.%s", host), nil
}

func (m *localMetaData) Mac() (string, error) {

	return "", fmt.Errorf("unimplemented")
}

func (m *localMetaData) NetworkType() (string, error) {

	return "", fmt.Errorf("unimplemented")
}

func (m *localMetaData) OwnerAccountID() (string, error) {

	return "", fmt.Errorf("unimplemented")
}

func (m *localMetaData) PrivateIPv4() (string, error) {

	return GetLocalIP()
}

func (m *localMetaData) Region() (string, error) {
	if m.config.Region != "" {
		return m.config.Region, nil
	}
	return m.base.Region()
}

func (m *localMetaData) SerialNumber() (string, error) {

	return "", fmt.Errorf("unimplemented")
}

func (m *localMetaData) SourceAddress() (string, error) {

	return "", fmt.Errorf("unimplemented")

}

func (m *localMetaData) VpcCIDRBlock() (string, error) {

	return "", fmt.Errorf("unimplemented")
}

func (m *localMetaData) VpcID() (string, error) {
	if m.config.VpcID != "" {
		return m.config.VpcID, nil
	}
	return m.base.VpcID()
}

func (m *localMetaData) VswitchCIDRBlock() (string, error) {

	return "", fmt.Errorf("unimplemented")
}

// zone1:vswitchid1,zone2:vswitch2
func (m *localMetaData) VswitchID() (string, error) {

	if m.config.VswitchID == "" {
		// get vswitch id from meta server
		return m.base.VswitchID()
	}
	zlist := strings.Split(m.config.VswitchID, ",")
	if len(zlist) == 1 {
		klog.Infof("simple vswitchid mode, %s", m.config.VswitchID)
		return m.config.VswitchID, nil
	}
	zone, err := m.Zone()
	if err != nil {
		return "", fmt.Errorf("retrieve vswitchid error for %s", err.Error())
	}
	for _, zone := range zlist {
		vs := strings.Split(zone, ":")
		if len(vs) != 2 {
			return "", fmt.Errorf("cloud-config vswitch format error: %s", m.config.VswitchID)
		}
		if vs[0] == zone {
			return vs[1], nil
		}
	}
	klog.Infof("zone[%s] match failed, fallback with simple vswitch id mode, [%s]", zone, m.config.VswitchID)
	return m.config.VswitchID, nil
}

func (m *localMetaData) EIPv4() (string, error) {

	return "", fmt.Errorf("unimplemented")
}

func (m *localMetaData) DNSNameServers() ([]string, error) {

	return []string{""}, fmt.Errorf("unimplemented")
}

func (m *localMetaData) NTPConfigServers() ([]string, error) {

	return []string{""}, fmt.Errorf("unimplemented")
}

func (m *localMetaData) Zone() (string, error) {
	if m.config.ZoneID != "" {
		return m.config.ZoneID, nil
	}
	return m.base.Zone()
}

func (m *localMetaData) RoleName() (string, error) {

	return m.base.RoleName()
}

func (m *localMetaData) RamRoleToken(role string) (meta.RoleAuth, error) {

	return m.base.RamRoleToken(role)
}

// GetLocalIP returns the non loopback local IP of the host
func GetLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", errors.Wrap(err, "get local ip")
	}
	for _, address := range addrs {
		// check the address type and if it is not a loopback the display it
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}
	return "", fmt.Errorf("not found")
}
