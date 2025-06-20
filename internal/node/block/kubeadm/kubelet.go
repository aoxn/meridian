//go:build linux || darwin
// +build linux darwin

package kubeadm

import (
	"context"
	"encoding/base64"
	"fmt"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/node/block"
	"github.com/aoxn/meridian/internal/node/block/file"
	"github.com/aoxn/meridian/internal/node/host"
	"github.com/aoxn/meridian/internal/tool"
	"github.com/aoxn/meridian/internal/tool/cmd"
	"github.com/aoxn/meridian/internal/tool/nvidia"
	"github.com/aoxn/meridian/internal/tool/sign"
	"github.com/pkg/errors"
	"io/ioutil"
	"k8s.io/klog/v2"
	"os"
	"path/filepath"
	"strings"
)

const (
	ObjectName        = "config"
	KUBELET_UNIT_FILE = "/etc/systemd/system/kubelet.service"
)

type kubeletInit struct {
	file      *file.File
	req       *v1.Request
	host      host.Host
	role      v1.NodeRole
	labels    []string
	nodeGroup string
}

func NewKubeletBlock(req *v1.Request, host host.Host, role v1.NodeRole, ng string, labels []string) (block.Block, error) {
	info := file.PathInfo{
		InnerAddr: false,
		Arch:      host.Arch(),
		OSRelease: host.OS(),
		Region:    host.Region(),
	}
	err := info.Validate()
	if err != nil {
		return nil, err
	}
	return &kubeletInit{
		labels:    labels,
		role:      role,
		req:       req,
		host:      host,
		nodeGroup: ng,
		file: &file.File{
			Path:    info,
			Pkg:     file.PKG_KUBERNETES,
			Ftype:   file.FILE_BINARY,
			Version: req.Spec.Config.Kubernetes.Version,
		},
	}, nil
}

// Ensure runs the actionInit
func (a *kubeletInit) Ensure(ctx context.Context) error {
	klog.Info("try load kubernetes ca cert")
	switch a.role {
	case v1.NodeRoleMaster:
		if err := LoadCert(a.req); err != nil {
			return fmt.Errorf("sign: %s", err.Error())
		}
	default:
		klog.Infof("not master role, skip apply cert")
	}

	ip, err := tool.GetDNSIP(a.req.Spec.Config.Network.SVCCIDR, 10)
	if err != nil {
		return fmt.Errorf("get cluster dns ip fail %s", err.Error())
	}
	if err = a.file.Ensure(ctx); err != nil {
		return errors.Wrapf(err, "install kubelet: %s", a.req.Name)
	}
	if err := os.WriteFile(
		KUBELET_UNIT_FILE,
		[]byte(a.KubeletUnitFile(a.req, ip.String())),
		0644,
	); err != nil {
		return fmt.Errorf("write file %s: %s", KUBELET_UNIT_FILE, err.Error())
	}
	err = a.createRavenKubeconfig()
	if err != nil {
		return errors.Wrapf(err, "create raven kube config")
	}
	addr := a.host.NodeIP()
	switch a.role {
	case v1.NodeRoleMaster:
	default:
		addr = a.req.Spec.AccessPoint.Internet
		if addr == "" {
			return fmt.Errorf("access point internet address is empty")
		}
	}
	err = tool.AddHostResolve(v1.APIServerDomain, addr)
	if err != nil {
		return errors.Wrapf(err, "add host resolve %s", addr)
	}
	return a.host.Service().Stop("kubelet")
}

func (a *kubeletInit) createRavenKubeconfig() error {
	var (
		addr = "127.0.0.1"
		port = "6443"
	)

	switch a.role {
	case v1.NodeRoleMaster:
	default:
		addr, port = v1.APIServerDomain, a.req.Spec.AccessPoint.APIPort
	}
	root := a.req.Spec.Config.TLS["root"]
	key, crt, err := sign.SignRaven(root.Cert, root.Key, []string{})
	if err != nil {
		return fmt.Errorf("sign raven client crt: %s", err.Error())
	}
	err = os.MkdirAll("/etc/raven", 0755)
	if err != nil {
		return fmt.Errorf("make wdrip dir: %s", err.Error())
	}

	cfg, err := tool.RenderConfig(
		"raven.kubeconfig",
		tool.KubeConfigTpl,
		tool.RenderParam{
			AuthCA:      base64.StdEncoding.EncodeToString(root.Cert),
			Address:     addr,
			Port:        port,
			ClusterName: "raven.cluster",
			UserName:    "raven.user",
			ClientCRT:   base64.StdEncoding.EncodeToString(crt),
			ClientKey:   base64.StdEncoding.EncodeToString(key),
		},
	)
	if err != nil {
		return fmt.Errorf("render raven config error: %s", err.Error())
	}
	return os.WriteFile(fmt.Sprintf("/etc/raven/kubeconfig"), []byte(cfg), 0644)
}

func (a *kubeletInit) Purge(ctx context.Context) error {
	var (
		bin  = "kubeadm"
		args = []string{"reset"}
	)
	extract := <-cmd.NewCmd(bin, args...).StartWithStdin(strings.NewReader("y"))
	content, err := cmd.CmdResult(extract)
	if err != nil {
		klog.Infof("kubelet uninstall[kubeadm reset]: %s", content)
	}
	klog.Infof("run kubeadm reset: %s", content)
	for _, r := range []string{
		"kubeadm", "kubelet", "kubelet", "bandwidth", "bridge", "crictl",
		"dhcp", "firewall", "flannel", "host-device", "host-local", "ipvlan",
		"kubeadm", "kubectl", "kubelet", "loopback", "macvlan", "portmap",
		"ptp", "sbr", "static", "tuning", "vlan", "vrf",
	} {
		full := fmt.Sprintf("/usr/local/bin/%s", r)
		klog.Infof("remove kubelet file: [%s]", full)
		err = os.RemoveAll(full)
		if err != nil {
			return errors.Wrapf(err, "remove[%s]", full)
		}
	}
	for _, r := range []string{
		"/etc/kubernetes", "/etc/kubeadm",
	} {
		klog.Infof("remove kubernetes file: [%s]", r)
		err = os.RemoveAll(r)
		if err != nil {
			return errors.Wrapf(err, "remove[%s]", r)
		}
	}
	return nil
}

func (a *kubeletInit) CleanUp(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (a *kubeletInit) Name() string {
	return fmt.Sprintf("kubelet init [%s]", a.host.NodeID())
}

func (a *kubeletInit) crictl() error {
	content := `
runtime-service: unix:///run/containerd/containerd.sock
image-service: unix:///run/containerd/containerd.sock
timeout: 2
debug: false
pull-image-on-create: false
`
	return os.WriteFile("/etc/crictl.yaml", []byte(content), 0755)
}

func LoadCert(node *v1.Request) error {
	err := os.MkdirAll("/etc/kubernetes/pki", 0755)
	if err != nil {
		return fmt.Errorf("mkdir error: %s", err.Error())
	}

	var (
		root    = node.Spec.Config.TLS["root"]
		control = node.Spec.Config.TLS["control"]
		front   = node.Spec.Config.TLS["front-proxy"]
		svc     = node.Spec.Config.TLS["svc"]
	)

	apica := root.Cert

	if control != nil {
		apica = append(apica, control.Cert...)
	}
	for name, v := range map[string][]byte{
		"front-proxy-ca.crt": front.Cert,
		"front-proxy-ca.key": front.Key,
		"ca.crt":             root.Cert,
		"ca.key":             root.Key,
		"sa.key":             svc.Key,
		"sa.pub":             svc.Cert,
		"apiserver-ca.crt":   apica,
		"apiserver-ca.key":   root.Key,
	} {
		if err := ioutil.WriteFile(certHome(name), v, 0644); err != nil {
			return fmt.Errorf("write file %s: %s", name, err.Error())
		}
	}

	// do cert clean up
	// let kubeadm do the sign work for the service
	for _, name := range []string{
		"apiserver.crt", "apiserver.key",
		"front-proxy-client.crt", "front-proxy-client.key",
		"apiserver-kubelet-client.crt", "apiserver-kubelet-client.key",
		"../admin.conf", "../controller-manager.conf",
		"../kubelet.conf", "../scheduler.conf",
		"/var/lib/kubelet/pki/",
	} {
		err := os.Remove(certHome(name))
		if err != nil {
			if strings.Contains(err.Error(), "no such file or directory") {
				continue
			}
			return fmt.Errorf("clean up existing cert fail: %s", err.Error())
		}
	}
	// clean up pki dir for kubelet to renew.
	rm := <-cmd.NewCmd(
		"rm",
		"-rf",
		"/var/lib/kubelet/pki/",
	).Start()
	return cmd.CmdError(rm)
}

func certHome(key string) string {
	return filepath.Join("/etc/kubernetes/pki/", key)
}

func (a *kubeletInit) KubeletUnitFile(node *v1.Request, ip string) string {
	up := []string{
		"[Unit]",
		"Description=kubelet: The Kubernetes NodeObject Agent",
		"Documentation=http://kubernetes.io/docs/",
		"",
		"[Service]",
	}
	down := []string{
		"StartLimitInterval=0",
		"Restart=always",
		"RestartSec=15s",
		"[Install]",
		"WantedBy=multi-user.target",
	}
	var (
		mid  []string
		keys []string
	)
	cfg := NewConfigTpl(node, a.host)
	paraMap := map[string]string{
		"KUBELET_CLUSTER_DNS":      fmt.Sprintf("--cluster-dns=%s", ip),
		"KUBELET_DOMAIN":           "--cluster-domain=cluster.local",
		"KUBELET_CGROUP_DRIVER":    "--cgroup-driver=systemd",
		"KUBELET_BOOTSTRAP_ARGS":   "--bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf",
		"KUBELET_KUBECONFIG_ARGS":  "--kubeconfig=/etc/kubernetes/kubelet.conf",
		"KUBELET_SYSTEM_PODS_ARGS": "--pod-manifest-path=/etc/kubernetes/manifests",
		//"KUBELET_ALLOW_PRIVILEGE":     "--allow-privileged=true",
		//"KUBELET_NETWORK_ARGS":        "--network-plugin=cni --cni-conf-dir=/etc/cni/net.d --cni-bin-dir=/opt/cni/bin",
		"KUBELET_POD_INFRA_CONTAINER": fmt.Sprintf("--pod-infra-container-image=%s/acs/pause-amd64:3.5", node.Spec.Config.Registry),
		"KUBELET_HOSTNAME_OVERRIDE":   fmt.Sprintf("--hostname-override=%s --provider-id=%s", cfg.NodeName, a.host.ProviderID()),
		"KUBELET_CERTIFICATE_ARGS":    "--anonymous-auth=false --rotate-certificates=true --cert-dir=/var/lib/kubelet/pki --fail-swap-on=false",
		"KUBELET_AUTHZ_ARGS":          "--authorization-mode=Webhook --client-ca-file=/etc/kubernetes/pki/ca.crt",
		"KUBELET_SYSTEM_RESERVED":     "--system-reserved=memory=300Mi --kube-reserved=memory=400Mi --eviction-hard=imagefs.available<15%,memory.available<300Mi,nodefs.available<10%,nodefs.inodesFree<5%",
	}

	var labels []string
	exist, err := nvidia.HasNvidiaDevice()
	if err != nil {
		klog.Infof("failed to check nvidia device: %v", err)
	}
	if exist {
		labels = append(labels, fmt.Sprintf("%s=%s", nvidia.LabelNvidiaDevice, "gpu"))
	}
	if a.nodeGroup != "" {
		labels = append(labels, fmt.Sprintf("%s=%s", v1.MERIDIAN_NODEGROUP, a.nodeGroup))
	}

	if len(a.labels) > 0 {
		labels = append(labels, a.labels...)
	}
	if len(labels) > 0 {
		paraMap["KUBELET_LABELS"] = fmt.Sprintf("--node-labels=%s", strings.Join(labels, ","))
	}
	for k, v := range paraMap {
		keys = append(keys, fmt.Sprintf("$%s", k))
		mid = append(mid, fmt.Sprintf("Environment=\"%s=%s\"", k, v))
	}
	down = append(
		[]string{fmt.Sprintf("ExecStart=/usr/local/bin/kubelet %s", strings.Join(keys, " "))},
		down...,
	)
	tmp := append(
		append(up, mid...),
		down...,
	)
	return strings.Join(tmp, "\n")
}
