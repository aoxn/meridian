//go:build linux || darwin || windows
// +build linux darwin windows

package kubeadm

import (
	"context"
	"encoding/base64"
	"fmt"
	api "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/crds"
	"github.com/aoxn/meridian/internal/node/block"
	"github.com/aoxn/meridian/internal/node/host"
	"github.com/aoxn/meridian/internal/tool"
	"github.com/aoxn/meridian/internal/tool/kubeclient"
	"github.com/aoxn/meridian/internal/tool/sign"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"os"
	"time"
)

type kubeAuthBlock struct {
	req  *api.Request
	host host.Host
}

// NewKubeAuthBlock returns a new kubeAuthBlock for kubeadm init
func NewKubeAuthBlock(req *api.Request, host host.Host) (block.Block, error) {
	return &kubeAuthBlock{req: req, host: host}, nil
}

// Ensure runs the NewKubeAuthBlock
func (a *kubeAuthBlock) Ensure(ctx context.Context) error {

	klog.Info("try write admin.local auth config")
	err := os.MkdirAll("/etc/kubernetes/", 0755)
	if err != nil {
		return fmt.Errorf("ensure dir /etc/kubernetes for admin.local:%s", err.Error())
	}

	root := a.req.Spec.Config.TLS["root"]
	key, crt, err := sign.SignKubernetesClient(root.Cert, root.Key, []string{})
	if err != nil {
		return fmt.Errorf("sign kubernetes client crt: %s", err.Error())
	}
	err = os.MkdirAll("/etc/meridian", 0755)
	if err != nil {
		return fmt.Errorf("make wdrip dir: %s", err.Error())
	}
	err = os.WriteFile(
		"/etc/meridian/meridian.cfg.gen",
		[]byte(tool.PrettyYaml(ctx)), 0755,
	)
	if err != nil {
		klog.Warningf("write bach config failed: %s", err.Error())
	}
	cfg, err := tool.RenderConfig(
		"admin.authconfig",
		tool.KubeConfigTpl,
		tool.RenderParam{
			AuthCA:      base64.StdEncoding.EncodeToString(root.Cert),
			Address:     a.host.NodeIP(),
			Port:        "6443",
			ClusterName: "kubernetes.cluster",
			UserName:    "kubernetes.user",
			ClientCRT:   base64.StdEncoding.EncodeToString(crt),
			ClientKey:   base64.StdEncoding.EncodeToString(key),
		},
	)
	if err != nil {
		return fmt.Errorf("render admin.local config error: %s", err.Error())
	}
	err = os.WriteFile(tool.AUTH_FILE, []byte(cfg), 0755)
	if err != nil {
		return err
	}
	homecfg, err := block.HomeKubeCfg()
	if err != nil {
		return err
	}
	err = os.WriteFile(homecfg, []byte(cfg), 0755)
	if err != nil {
		return err
	}
	if err = a.fixClusterInfo(); err != nil {
		return err
	}
	err = kubeclient.ApplyBy(konnectRole, homecfg)
	if err != nil {
		return errors.Wrapf(err, "apply konnectivity role")
	}
	err = a.createResource()
	if err != nil {
		return errors.Wrapf(err, "ensure init request")
	}
	return createMeridianKubecfg([]byte(cfg))
}

func (a *kubeAuthBlock) createResource() error {
	err := crds.InitFromKubeconfig(tool.AUTH_FILE)
	if err != nil {
		return err
	}
	klog.Infof("bind to request: [%s]", a.req.Name)
	return tool.ApplyYaml(tool.PrettyYaml(a.req), fmt.Sprintf("request-%s", a.req.Name))
}

func (a *kubeAuthBlock) Purge(ctx context.Context) error {
	homecfg, err := block.HomeKubeCfg()
	if err != nil {
		return err
	}
	for _, r := range []string{
		tool.AUTH_FILE, homecfg,
	} {
		klog.Infof("remove kube auth file: %s", r)
		err = os.RemoveAll(r)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *kubeAuthBlock) CleanUp(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (a *kubeAuthBlock) Name() string {
	return fmt.Sprintf("kube auth for [%s]", a.host.NodeID())
}

func createMeridianKubecfg(cfgData []byte) error {

	sec := v1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "meridian-kubeconfig",
			Namespace: "kube-system",
		},
		Data: map[string][]byte{
			"kubeconfig": cfgData,
		},
	}
	return wait.Poll(
		2*time.Second,
		1*time.Minute,
		func() (done bool, err error) {
			err = tool.ApplyYaml(tool.PrettyYaml(sec), "meridian-kubeconfig")
			if err != nil {
				klog.Errorf("retry wait for meridian kubeconfig: %s", err.Error())
				return false, nil
			}
			return true, nil
		},
	)
}

func (a *kubeAuthBlock) fixClusterInfo() error {
	kubefile, err := block.HomeKubeCfg()
	if err != nil {
		return err
	}
	cfg, err := clientcmd.BuildConfigFromFlags("", kubefile)
	if err != nil {
		return err
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}
	cmi := client.CoreV1().ConfigMaps("kube-public")
	info, err := cmi.Get(context.TODO(), "cluster-info", metav1.GetOptions{})
	if err != nil {
		return err
	}
	data := info.Data["kubeconfig"]
	ccfg, err := clientcmd.Load([]byte(data))
	if err != nil {
		return err
	}
	access := a.req.Spec.AccessPoint
	for i, _ := range ccfg.Clusters {
		ccfg.Clusters[i].Server = fmt.Sprintf("https://%s:%s", access.APIDomain, access.APIPort)
	}
	content, err := clientcmd.Write(*ccfg)
	if err != nil {
		return err
	}
	info.Data["kubeconfig"] = string(content)
	_, err = cmi.Update(context.TODO(), info, metav1.UpdateOptions{})
	return err
}
