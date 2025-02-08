package addons

import (
	api "github.com/aoxn/meridian/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cluster-bootstrap/token/util"
	"testing"
)

func TestAddon(t *testing.T) {
	req, err := NewRequest()
	if err != nil {
		t.Fatal(err)
	}
	_, err = RenderedRequestedAddons(req)
	if err != nil {
		t.Fatal(err)
	}
}

func NewRequest() (*api.Request, error) {
	token, err := util.GenerateBootstrapToken()
	if err != nil {
		return nil, err
	}
	req := &api.Request{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Request",
			APIVersion: api.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "xdpin-001",
			Namespace: "default",
		},
		Spec: api.RequestSpec{
			Config: api.ClusterConfig{
				TLS: map[string]*api.KeyCert{},
				Etcd: api.Etcd{
					Unit: api.Unit{
						Version: "v3.4.3",
					},
				},
				Kubernetes: api.Kubernetes{
					Unit: api.Unit{
						Version: "1.31.1-aliyun.1",
					},
				},
				Namespace: "default",
				CloudType: "public",
				Network: api.NetworkCfg{
					SVCCIDR: "172.16.0.1/16",
					PodCIDR: "10.0.0.0/16",
					Domain:  "xdpin.local",
				},
				Token:    token,
				Registry: "registry.cn-hangzhou.aliyuncs.com",
			},
			AccessPoint: api.AccessPoint{
				APIDomain: "xdpin.cn",
				Intranet:  "127.0.0.1",
			},
		},
	}
	return req, nil
}
