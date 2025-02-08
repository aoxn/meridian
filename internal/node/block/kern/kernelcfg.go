//go:build linux
// +build linux

package docker

import (
	"context"
	"fmt"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/node/block"
	"github.com/aoxn/meridian/internal/node/host"
	"github.com/go-cmd/cmd"
	"io/ioutil"
	"os"
	"strings"
)

type kernelCfg struct {
	req  *v1.Request
	host host.Host
}

// NewBlock returns a new kernelCfg for kubeadm init
func NewBlock(req *v1.Request, host host.Host) (block.Block, error) {
	return &kernelCfg{req: req, host: host}, nil
}

// Ensure runs the kernelCfg
func (a *kernelCfg) Ensure(ctx context.Context) error {
	return applyKernelCfg()
}

func applyKernelCfg() error {

	if err := os.MkdirAll("/etc/sysctl.d/", 0755); err != nil {
		return err
	}
	var buffer []string
	for k, v := range map[string]string{
		"vm.max_map_count":                    "262144",
		"kernel.softlockup_panic":             "1",
		"net.core.somaxconn":                  "32768",
		"net.core.rmem_max":                   "16777216",
		"net.core.wmem_max":                   "16777216",
		"net.ipv4.tcp_wmem":                   "4096 12582912 16777216",
		"net.ipv4.tcp_rmem":                   "4096 12582912 16777216",
		"net.ipv4.tcp_max_syn_backlog":        "8096",
		"net.ipv4.tcp_slow_start_after_idle":  "0",
		"kernel.softlockup_all_cpu_backtrace": "1",
		"net.bridge.bridge-nf-call-iptables":  "1",
		"net.core.netdev_max_backlog":         "16384",
		"fs.file-max":                         "2097152",
		"fs.inotify.max_user_instances":       "8192",
		"fs.inotify.max_user_watches":         "524288",
		"fs.inotify.max_queued_events":        "16384",
		"net.ipv4.ip_forward":                 "1",
		"fs.may_detach_mounts":                "1",
	} {
		key := strings.Replace(k, ".", "/", -1)
		_, err := os.Stat(
			fmt.Sprintf("/proc/sys/%s", key),
		)
		if err == nil || os.IsExist(err) {
			buffer = append(buffer, fmt.Sprintf("%s=%s", k, v))
		}
	}
	err := ioutil.WriteFile(
		"/etc/sysctl.d/99-k8s.conf",
		[]byte(strings.Join(buffer, "\n")),
		0755,
	)
	if err != nil {
		return fmt.Errorf("write sysctl file error(/etc/sysctl.d/99-k8s.conf): %s", err.Error())
	}
	// ignore error
	_ = cmd.NewCmd("sysctl", "--system").Start()
	return nil
}

func (a *kernelCfg) Name() string {
	return fmt.Sprintf("kernel config: [%s]", a.host.NodeID())
}

func (a *kernelCfg) Purge(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (a *kernelCfg) CleanUp(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

var daemonjson = `
public::common::apply_sysctls() {
	declare -A sysctls_map=(
		["vm.max_map_count"]="262144"
		["kernel.softlockup_panic"]="1"
		["kernel.softlockup_all_cpu_backtrace"]="1"
		["net.core.somaxconn"]="32768"
		["net.core.rmem_max"]="16777216"
		["net.core.wmem_max"]="16777216"
		["net.ipv4.tcp_wmem"]="4096 12582912 16777216"
		["net.ipv4.tcp_rmem"]="4096 12582912 16777216"
		["net.ipv4.tcp_max_syn_backlog"]="8096"
		["net.ipv4.tcp_slow_start_after_idle"]="0"
		["net.core.netdev_max_backlog"]="16384"
		["fs.file-max"]="2097152"
		["fs.inotify.max_user_instances"]="8192"
		["fs.inotify.max_user_watches"]="524288"
		["fs.inotify.max_queued_events"]="16384"
		["net.ipv4.ip_forward"]="1"
		["net.bridge.bridge-nf-call-iptables"]="1"
		["fs.may_detach_mounts"]="1"
	)

	if [ ! -f /etc/sysctl.d/99-k8s.conf ]; then
		mkdir -p /etc/sysctl.d/ && touch /etc/sysctl.d/99-k8s.conf
		echo "#sysctls for k8s host config" >/etc/sysctl.d/99-k8s.conf
	fi
	for key in ${!sysctls_map[@]}; do
		sysctl_path="/proc/sys/"${key//./\/}
		if [ -f ${sysctl_path} ]; then
			sed -i "/${key}/ d" /etc/sysctl.d/99-k8s.conf
			echo "${key}=${sysctls_map[${key}]}" >>/etc/sysctl.d/99-k8s.conf
		fi
	done
	sysctl --system || true
}
`
