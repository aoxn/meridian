package kubeadm

import (
	"fmt"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/node/block/etcd"
	"github.com/aoxn/meridian/internal/node/host"
	"github.com/aoxn/meridian/internal/tool"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	tokenv1 "k8s.io/kubernetes/cmd/kubeadm/app/apis/bootstraptoken/v1"
	kv1 "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta3"
	"strings"
)

var (
	egressFile       = "egress-selector-configuration.yaml"
	konnectivityHost = "/etc/kubernetes/konnectivity-server"
)

func NewInitCfg(req *v1.Request, host host.Host) string {
	token := strings.Split(req.Spec.Config.Token, ".")

	icfg := kv1.InitConfiguration{
		TypeMeta: metav1.TypeMeta{
			Kind:       "InitConfiguration",
			APIVersion: "kubeadm.k8s.io/v1beta3",
		},
		BootstrapTokens: []tokenv1.BootstrapToken{
			{
				Token: &tokenv1.BootstrapTokenString{
					ID: token[0], Secret: token[1],
				}, TTL: &metav1.Duration{},
			},
		},
		NodeRegistration: kv1.NodeRegistrationOptions{
			KubeletExtraArgs: map[string]string{
				"cloud-provider": "external",
			},
			Name: host.NodeName(),
		},
	}
	getSans := func() []string {
		sans := req.Spec.Config.Sans
		if req.Spec.AccessPoint.Internet != "" {
			sans = append(sans, req.Spec.AccessPoint.Internet)
		}
		if req.Spec.AccessPoint.Intranet != "" {
			sans = append(sans, req.Spec.AccessPoint.Intranet)
		}
		if req.Spec.AccessPoint.APIDomain != "" {
			sans = append(sans, req.Spec.AccessPoint.APIDomain)
		}
		return sans
	}
	cluster := kv1.ClusterConfiguration{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterConfiguration",
			APIVersion: "kubeadm.k8s.io/v1beta3",
		},
		APIServer: kv1.APIServer{
			CertSANs: getSans(),
			ControlPlaneComponent: kv1.ControlPlaneComponent{
				ExtraArgs: map[string]string{
					"cloud-provider":              "external",
					"egress-selector-config-file": fmt.Sprintf("%s/%s", konnectivityHost, egressFile),
				},
				ExtraVolumes: []kv1.HostPathMount{
					{HostPath: "/etc/localtime", MountPath: "/etc/localtime", Name: "localtime"},
					{
						HostPath:  konnectivityHost,
						MountPath: konnectivityHost,
						Name:      "konnectivity-uds",
						ReadOnly:  false,
						PathType:  corev1.HostPathDirectoryOrCreate,
					},
				},
			},
		},
		ControllerManager: kv1.ControlPlaneComponent{
			ExtraArgs: map[string]string{
				"cloud-provider":         "external",
				"flex-volume-plugin-dir": "/var/lib/kubelet/kubelet-plugins/volume/exec",
			},
			ExtraVolumes: []kv1.HostPathMount{
				{HostPath: "/etc/localtime", MountPath: "/etc/localtime", Name: "localtime"},
			},
		},
		Scheduler: kv1.ControlPlaneComponent{
			ExtraVolumes: []kv1.HostPathMount{
				{HostPath: "/etc/localtime", MountPath: "/etc/localtime", Name: "localtime"},
			},
		},
		Etcd: kv1.Etcd{
			External: &kv1.ExternalEtcd{
				CAFile:    fmt.Sprintf("%s/cert/server-ca.crt", etcd.EtcdHome()),
				CertFile:  fmt.Sprintf("%s/cert/client.crt", etcd.EtcdHome()),
				KeyFile:   fmt.Sprintf("%s/cert/client.key", etcd.EtcdHome()),
				Endpoints: []string{fmt.Sprintf("https://%s:2379", host.NodeIP())},
			},
		},
		Networking: kv1.Networking{
			DNSDomain:     req.Spec.Config.Network.Domain,
			PodSubnet:     req.Spec.Config.Network.PodCIDR,
			ServiceSubnet: req.Spec.Config.Network.SVCCIDR,
		},
		//ControlPlaneEndpoint: req.Spec.AccessPoint.Intranet,
		ImageRepository:   fmt.Sprintf("%s/acs", req.Spec.Config.Registry),
		KubernetesVersion: req.Spec.Config.Kubernetes.Version,
		ClusterName:       req.Name,
	}
	// https://kubernetes.io/zh-cn/docs/reference/config-api/kubeadm-config.v1beta3/
	scfg := tool.PrettyYaml(icfg)
	scluster := tool.PrettyYaml(cluster)
	return fmt.Sprintf("%s\n---\n%s", scfg, scluster)
}
