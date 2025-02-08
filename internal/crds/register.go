package crds

import (
	"fmt"
	v1 "github.com/aoxn/meridian/api/v1"
	ravenv1beta1 "github.com/openyurtio/openyurt/pkg/apis/raven/v1beta1"
	"github.com/pkg/errors"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog/v2"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

const (
	MasterSetKind       = "MasterSet"
	MasterSetName       = "masterset"
	MasterSetShotName   = "ms"
	MasterSetNamePlural = "mastersets"

	MasterKind       = "Master"
	MasterName       = "master"
	MasterShotName   = "m"
	MasterNamePlural = "masters"

	InfraKind       = "Infra"
	InfraName       = "infra"
	InfraShotName   = "in"
	InfraNamePlural = "infras"

	ClusterKind       = "Cluster"
	ClusterName       = "cluster"
	ClusterShotName   = "cls"
	ClusterNamePlural = "clusters"

	GatewayKind       = "Gateway"
	GatewayName       = "gateway"
	GatewayShotName   = "gtw"
	GatewayNamePlural = "gateways"

	RequestKind       = "Request"
	RequestName       = "request"
	RequestShotName   = "req"
	RequestNamePlural = "requests"

	NodeGroupKind       = "NodeGroup"
	NodeGroupName       = "nodegroup"
	NodeGroupShotName   = "ng"
	NodeGroupNamePlural = "nodegroups"

	ProviderKind       = "Provider"
	ProviderName       = "provider"
	ProviderShotName   = "pvd"
	ProviderNamePlural = "providers"
)

// InitializeCRD register crds from in cluster config file
func InitializeCRD(cfg *rest.Config) error {
	return doRegisterCRD(cfg)
}

func InitFromConfigAPI(cfg *clientcmdapi.Config) error {
	getter := func() (*clientcmdapi.Config, error) {
		return cfg, nil
	}
	restCfg, err := clientcmd.BuildConfigFromKubeconfigGetter("", getter)
	if err != nil {
		return errors.Wrap(err, "build client from clientcmdapi config")
	}
	return InitializeCRD(restCfg)
}

// InitFromKubeconfig register crds from kubeconfig file
func InitFromKubeconfig(name string) error {
	cfg, err := clientcmd.BuildConfigFromFlags("", name)
	if err != nil {
		return fmt.Errorf("register tool: build service.config, %s", err.Error())
	}
	return doRegisterCRD(cfg)
}

func doRegisterCRD(cfg *rest.Config) error {
	extc, err := apiext.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("error create incluster client: %s", err.Error())
	}
	client := NewClient(extc)
	for _, crd := range []CRD{
		NewMasterSetCRD(client),
		NewMaster(client),
		NewInfra(client),
		NewCluster(client),
		NewGateway(client),
		NewRequest(client),
		NewNodeGroup(client),
		NewProviderCRD(client),
	} {
		err := crd.Initialize()
		if err != nil {
			return fmt.Errorf("initialize tool: %s, %s", reflect.TypeOf(crd), err.Error())
		}
		klog.Infof("register tool: %s", reflect.TypeOf(crd))
	}
	return nil
}

type CRD interface {
	Initialize() error
	GetObject() runtime.Object
	GetListerWatcher() cache.ListerWatcher
}

// MasterSetCRD is the cluster tool .
type MasterSetCRD struct {
	crdi Interface
}

func NewMasterSetCRD(
	crdi Interface,
) *MasterSetCRD {
	return &MasterSetCRD{crdi: crdi}
}

func (p *MasterSetCRD) Initialize() error {
	crd := Conf{
		Kind:       MasterSetKind,
		ShortNames: []string{MasterSetShotName},
		NamePlural: MasterSetNamePlural,
		Group:      v1.GroupVersion.Group,
		Version:    v1.GroupVersion.Version,
		Scope:      apiextv1.ClusterScoped,
	}

	return p.crdi.EnsurePresent(crd)
}

// GetListerWatcher satisfies resource.tool interface (and retrieve.Retriever).
func (p *MasterSetCRD) GetListerWatcher() cache.ListerWatcher { return nil }

// GetObject satisfies resource.tool interface (and retrieve.Retriever).
func (p *MasterSetCRD) GetObject() runtime.Object { return &v1.MasterSet{} }

// Master is the cluster tool .
type Master struct {
	crdi Interface
}

func NewMaster(
	crdi Interface,
) *Master {
	return &Master{crdi: crdi}
}

func (p *Master) Initialize() error {
	crd := Conf{
		Kind:       MasterKind,
		ShortNames: []string{MasterShotName},
		NamePlural: MasterNamePlural,
		Group:      v1.GroupVersion.Group,
		Version:    v1.GroupVersion.Version,
		Scope:      apiextv1.ClusterScoped,
	}

	return p.crdi.EnsurePresent(crd)
}

// GetListerWatcher satisfies resource.tool interface (and retrieve.Retriever).
func (p *Master) GetListerWatcher() cache.ListerWatcher { return nil }

// GetObject satisfies resource.tool interface (and retrieve.Retriever).
func (p *Master) GetObject() runtime.Object { return &v1.Master{} }

// Infra is the cluster tool .
type Infra struct {
	crd Interface
}

func NewInfra(
	crdi Interface,
) *Infra {
	return &Infra{crd: crdi}
}

func (p *Infra) Initialize() error {
	crd := Conf{
		Kind:       InfraKind,
		ShortNames: []string{InfraShotName},
		NamePlural: InfraNamePlural,
		Group:      v1.GroupVersion.Group,
		Version:    v1.GroupVersion.Version,
		Scope:      apiextv1.ClusterScoped,
	}

	return p.crd.EnsurePresent(crd)
}

// GetListerWatcher satisfies resource.tool interface (and retrieve.Retriever).
func (p *Infra) GetListerWatcher() cache.ListerWatcher { return nil }

// GetObject satisfies resource.tool interface (and retrieve.Retriever).
func (p *Infra) GetObject() runtime.Object { return &v1.Infra{} }

// Cluster is the cluster tool .
type Cluster struct {
	crdi Interface
}

func NewCluster(
	crdi Interface,
) *Cluster {
	return &Cluster{crdi: crdi}
}

func (p *Cluster) Initialize() error {
	crd := Conf{
		Kind:       ClusterKind,
		ShortNames: []string{ClusterShotName},
		NamePlural: ClusterNamePlural,
		Group:      v1.GroupVersion.Group,
		Version:    v1.GroupVersion.Version,
		Scope:      apiextv1.ClusterScoped,
	}

	return p.crdi.EnsurePresent(crd)
}

// GetListerWatcher satisfies resource.tool interface (and retrieve.Retriever).
func (p *Cluster) GetListerWatcher() cache.ListerWatcher { return nil }

// GetObject satisfies resource.tool interface (and retrieve.Retriever).
func (p *Cluster) GetObject() runtime.Object { return &v1.Cluster{} }

// Gateway is the gateway tool .
type Gateway struct {
	crdi Interface
}

func NewGateway(
	crdi Interface,
) *Gateway {
	return &Gateway{crdi: crdi}
}

func (p *Gateway) Initialize() error {
	crd := Conf{
		Kind:       GatewayKind,
		ShortNames: []string{GatewayShotName},
		NamePlural: GatewayNamePlural,
		Group:      ravenv1beta1.GroupVersion.Group,
		Version:    ravenv1beta1.GroupVersion.Version,
		Scope:      apiextv1.ClusterScoped,
	}

	return p.crdi.EnsurePresent(crd)
}

// GetListerWatcher satisfies resource.tool interface (and retrieve.Retriever).
func (p *Gateway) GetListerWatcher() cache.ListerWatcher { return nil }

// GetObject satisfies resource.tool interface (and retrieve.Retriever).
func (p *Gateway) GetObject() runtime.Object { return &ravenv1beta1.Gateway{} }

// Request is the gateway tool .
type Request struct {
	crdi Interface
}

func NewRequest(
	crdi Interface,
) *Request {
	return &Request{crdi: crdi}
}

func (p *Request) Initialize() error {
	crd := Conf{
		Kind:       RequestKind,
		ShortNames: []string{RequestShotName},
		NamePlural: RequestNamePlural,
		Group:      v1.GroupVersion.Group,
		Version:    v1.GroupVersion.Version,
		Scope:      apiextv1.ClusterScoped,
	}

	return p.crdi.EnsurePresent(crd)
}

// GetListerWatcher satisfies resource.tool interface (and retrieve.Retriever).
func (p *Request) GetListerWatcher() cache.ListerWatcher { return nil }

// GetObject satisfies resource.tool interface (and retrieve.Retriever).
func (p *Request) GetObject() runtime.Object { return &v1.Request{} }

// NodeGroup is the gateway tool .
type NodeGroup struct {
	crdi Interface
}

func NewNodeGroup(
	crdi Interface,
) *NodeGroup {
	return &NodeGroup{crdi: crdi}
}

func (p *NodeGroup) Initialize() error {
	crd := Conf{
		Kind:       NodeGroupKind,
		ShortNames: []string{NodeGroupShotName},
		NamePlural: NodeGroupNamePlural,
		Group:      v1.GroupVersion.Group,
		Version:    v1.GroupVersion.Version,
		Scope:      apiextv1.ClusterScoped,
	}

	return p.crdi.EnsurePresent(crd)
}

// GetListerWatcher satisfies resource.tool interface (and retrieve.Retriever).
func (p *NodeGroup) GetListerWatcher() cache.ListerWatcher { return nil }

// GetObject satisfies resource.tool interface (and retrieve.Retriever).
func (p *NodeGroup) GetObject() runtime.Object { return &v1.NodeGroup{} }

// Provider is the gateway tool .
type Provider struct {
	crdi Interface
}

func NewProviderCRD(
	crdi Interface,
) *Provider {
	return &Provider{crdi: crdi}
}

func (p *Provider) Initialize() error {
	crd := Conf{
		Kind:       ProviderKind,
		ShortNames: []string{ProviderShotName},
		NamePlural: ProviderNamePlural,
		Group:      v1.GroupVersion.Group,
		Version:    v1.GroupVersion.Version,
		Scope:      apiextv1.ClusterScoped,
	}

	return p.crdi.EnsurePresent(crd)
}

// GetListerWatcher satisfies resource.tool interface (and retrieve.Retriever).
func (p *Provider) GetListerWatcher() cache.ListerWatcher { return nil }

// GetObject satisfies resource.tool interface (and retrieve.Retriever).
func (p *Provider) GetObject() runtime.Object { return &v1.Provider{} }
