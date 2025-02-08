//go:build linux || darwin || windows
// +build linux darwin windows

package kubeadm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/node/block"
	"github.com/aoxn/meridian/internal/node/host"
	"github.com/aoxn/meridian/internal/tool"
	"github.com/aoxn/meridian/internal/tool/kubeclient"
	"github.com/aoxn/meridian/internal/tool/sign"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"os"
)

var defaultTpl = `
kind: Config
contexts:
- context:
    cluster: kubernetes
    user: defaultUser
  name: default@kubernetes
current-context: default@kubernetes
users:
- name: defaultUser
  user:
    client-certificate-data: {{ .ClientCRT }}
    client-key-data: {{ .ClientKey }}
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: {{ .AuthCA}}
    server: {{ .IntranetLB }}
  name: kubernetes
`

type ccmConfig struct {
	req  *v1.Request
	host host.Host
}

// NewCCMBlock returns a new actionInit for kubeadm init
func NewCCMBlock(req *v1.Request, host host.Host) (block.Block, error) {
	return &ccmConfig{req: req, host: host}, nil
}

// Ensure runs the actionInit
func (a *ccmConfig) Ensure(ctx context.Context) error {

	klog.Info("try write ccm auth config")
	err := os.MkdirAll("/etc/kubernetes/", 0755)
	if err != nil {
		return fmt.Errorf("ensure dir /etc/kubernetes :%s", err.Error())
	}

	err = GenKubeConfig(a.req, "system:cloud-controller-manager", "ccm-kubeconfig", "https://127.0.0.1:6443")
	if err != nil {
		return fmt.Errorf("ensure kube-config :%s", err.Error())
	}
	return GenKubeConfig(a.req, "csi-admin", "csi-kubeconfig", fmt.Sprintf("https://%s:6443", a.req.Spec.AccessPoint.APIDomain))
}

func GenKubeConfig(req *v1.Request, name, sname, server string) error {
	root := req.Spec.Config.TLS["root"]
	key, crt, err := sign.SignBy(root.Cert, root.Key, name, []string{}, []string{}, []string{})
	if err != nil {
		return fmt.Errorf("sign kubernetes client crt: %s", err.Error())
	}
	cfg, err := tool.RenderConfig(
		sname, defaultTpl,
		struct {
			AuthCA     string
			IntranetLB string
			ClientCRT  string
			ClientKey  string
			Port       string
		}{
			AuthCA:     base64.StdEncoding.EncodeToString(root.Cert),
			IntranetLB: server,
			Port:       "6443",
			ClientCRT:  base64.StdEncoding.EncodeToString(crt),
			ClientKey:  base64.StdEncoding.EncodeToString(key),
		},
	)
	if err != nil {
		return fmt.Errorf("render config error: %s", err.Error())
	}
	homecfg, err := block.HomeKubeCfg()
	if err != nil {
		return err
	}
	sec := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      sname,
			Namespace: "kube-system",
		},
		StringData: map[string]string{
			"kubeconfig": cfg,
		},
	}
	data, _ := json.Marshal(sec)
	return kubeclient.ApplyBy(string(data), homecfg)
}

func (a *ccmConfig) Name() string {
	return fmt.Sprintf("ccm config: [%s]", a.host.NodeID())
}

func (a *ccmConfig) Purge(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (a *ccmConfig) CleanUp(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}
