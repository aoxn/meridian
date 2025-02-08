package meridian

import (
	"fmt"
	"github.com/aoxn/meridian"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/node/block/kubeadm"
	"github.com/aoxn/meridian/internal/tool"
	"github.com/aoxn/meridian/internal/tool/sign"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cluster-bootstrap/token/util"
	"k8s.io/klog/v2"
	"math/rand"
	"os"
	"path"
)

func newCA() (*v1.KeyCert, error) {
	key, crt, err := sign.SelfSignedPair()
	if err != nil {
		return nil, err
	}
	return &v1.KeyCert{Key: key, Cert: crt}, nil
}

func newCA4SA() (*v1.KeyCert, error) {
	key, crt, err := sign.SelfSignedPairSA()
	if err != nil {
		return nil, err
	}
	return &v1.KeyCert{Key: key, Cert: crt}, nil
}

func NewJoinRequest(req *v1.Request) *v1.Request {
	req.Spec.Config.TLS["root"].Key = []byte{}
	delete(req.Spec.Config.TLS, "svc")
	delete(req.Spec.Config.TLS, "front-proxy")
	delete(req.Spec.Config.TLS, "etcd-peer")
	delete(req.Spec.Config.TLS, "etcd-server")
	return req
}

func NewRequest() (*v1.Request, error) {
	root, err := newCA()
	if err != nil {
		return nil, err
	}
	frontProxy, err := newCA()
	if err != nil {
		return nil, err
	}
	svc, err := newCA4SA()
	if err != nil {
		return nil, err
	}
	etcdPeer, err := newCA()
	if err != nil {
		return nil, err
	}
	etcdServer, err := newCA()
	if err != nil {
		return nil, err
	}
	token, err := util.GenerateBootstrapToken()
	if err != nil {
		return nil, err
	}
	randPort := rand.Intn(1000)
	req := &v1.Request{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Request",
			APIVersion: v1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "xdpin-001",
			Namespace: "default",
		},
		Spec: v1.RequestSpec{
			Config: v1.ClusterConfig{
				TLS: map[string]*v1.KeyCert{
					"root":        root,
					"svc":         svc,
					"front-proxy": frontProxy,
					"etcd-peer":   etcdPeer,
					"etcd-server": etcdServer,
				},
				Etcd: v1.Etcd{
					Unit: v1.Unit{
						Version: "v3.4.3",
					},
				},
				Kubernetes: v1.Kubernetes{
					Unit: v1.Unit{
						Version: "1.31.1-aliyun.1",
					},
				},
				Namespace: "default",
				CloudType: "public",
				Network: v1.NetworkCfg{
					SVCCIDR: "172.16.0.1/16",
					PodCIDR: "10.0.0.0/16",
					Domain:  "xdpin.local",
				},
				Token:    token,
				Registry: "registry.cn-hangzhou.aliyuncs.com",
			},
			AccessPoint: v1.AccessPoint{
				APIDomain:  v1.APIServerDomain,
				Intranet:   "127.0.0.1",
				APIPort:    fmt.Sprintf("%d", 40000+randPort),
				TunnelPort: fmt.Sprintf("%d", 42000+randPort),
			},
		},
	}
	return req, nil
}

// NewCommandNew create resource
func NewCommandNew() *cobra.Command {
	var (
		join  = false
		write = ""
	)
	cmd := &cobra.Command{
		Use:    "new",
		Hidden: true,
		Short:  "meridian new",
		Long:   "",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf(meridian.Logo)
			if len(args) < 1 {
				return fmt.Errorf("resource is needed. [req]|[request]")
			}
			var (
				err error
				req = &v1.Request{}
			)
			if join {
				r, err := os.ReadFile(path.Join(kubeadm.KUBEADM_CONFIG_DIR, "request.yml"))
				if err != nil {
					return err
				}
				err = yaml.Unmarshal(r, req)
				if err != nil {
					return err
				}
				req = NewJoinRequest(req)
			} else {
				req, err = NewRequest()
				if err != nil {
					return errors.Wrapf(err, "build request")
				}
			}
			data := tool.PrettyYaml(req)
			if err != nil {
				return fmt.Errorf("new request template: %s", err.Error())
			}
			if write != "" {
				if !path.IsAbs(write) {
					dir, err := os.Getwd()
					if err != nil {
						klog.Infof("can not get current working directory")
						fmt.Println(data)
						return nil
					}
					write = path.Join(dir, write)
				}

				return os.WriteFile(write, []byte(data), 0755)
			} else {
				fmt.Printf("%s", data)
			}
			return nil
		},
	}
	cmd.PersistentFlags().StringVarP(&write, "write", "w", "", "write to file: request.yml in current dir")
	cmd.PersistentFlags().BoolVarP(&join, "join", "j", false, "generate join file: request-join.yml in current dir from request.yml")
	return cmd
}
