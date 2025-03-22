package common

import (
	"fmt"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/tool"
	"github.com/aoxn/meridian/internal/tool/sign"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cluster-bootstrap/token/util"
	"math/rand"
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
					InitToken: tool.RandomID(12),
				},
				Kubernetes: v1.Kubernetes{
					Unit: v1.Unit{
						Version: "1.31.1-aliyun.1",
					},
				},
				Runtime: v1.Runtime{
					Version:              "1.6.28",
					NvidiaToolKitVersion: "1.17.5",
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
