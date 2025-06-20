package node

import (
	"context"
	"fmt"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/node/block"
	"github.com/aoxn/meridian/internal/node/block/etcd"
	"github.com/aoxn/meridian/internal/node/block/kubeadm"
	"github.com/aoxn/meridian/internal/node/block/post"
	"github.com/aoxn/meridian/internal/node/block/runtime"
	"github.com/aoxn/meridian/internal/node/host"
	"github.com/aoxn/meridian/internal/node/host/meta/alibaba"
	"github.com/aoxn/meridian/internal/node/host/meta/local"
	"github.com/aoxn/meridian/internal/tool"
	"github.com/pkg/errors"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	clientgov1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/klog/v2"
	"os"
	"path"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

func splitHost(apiserver string) (addr, port string, err error) {
	pair := strings.Split(apiserver, ":")
	if len(pair) != 2 {
		return "", "", errors.New("invalid apiserver")
	}
	return pair[0], pair[1], nil
}

func withSchema(api string) string {
	return fmt.Sprintf("https://%s", api)
}

// 1. meridian host init 清理etcd、节点配置，然后重新初始化etcd和master节点。
// 2. meridian host recover

func InitNode(action string, role v1.NodeRole, apiserver, token, ng, cloud string, labels []string) (*Meridian, error) {
	switch role {
	case v1.NodeRoleWorker, v1.NodeRoleMaster:
	default:
		return nil, fmt.Errorf("invalid node role: %s", role)
	}
	switch action {
	case v1.ActionInit, v1.ActionJoin:
	default:
		return nil, fmt.Errorf("invalid node action: %s", action)
	}
	addr, _, err := splitHost(apiserver)
	if err != nil {
		return nil, err
	}
	err = tool.AddHostResolve(v1.APIServerDomain, addr)
	if err != nil {
		return nil, errors.Wrapf(err, "add resolve host")
	}
	var md Meridian
	cfg, err := GetCSR(apiserver, token, "xdpin")
	if err != nil {
		return &md, errors.Wrapf(err, "request kubeconfig failed")
	}
	bcfg, err := newClientCfg(*cfg)
	if err != nil {
		return &md, errors.Wrapf(err, "new kubeconfig failed")
	}
	cp, err := newControllerClient(bcfg)
	if err != nil {
		return &md, errors.Wrap(err, "failed to create controller client")
	}
	ctx := context.Background()
	var req = v1.Request{}
	err = cp.Get(ctx, client.ObjectKey{Name: ClusterName}, &req)
	if err != nil {
		return &md, errors.Wrap(err, "failed to get request")
	}
	return NewMeridianNode(action, role, ng, cloud, &req, labels)
}

func newControllerClient(cfg *rest.Config) (client.Client, error) {
	scheme := kruntime.NewScheme()
	utilruntime.Must(v1.AddToScheme(scheme))
	utilruntime.Must(clientgov1.AddToScheme(scheme))
	return client.New(cfg, client.Options{
		Scheme: scheme,
	})
}

const (
	KubeconfigTmp = "/tmp/kubeconfig"
	ClusterName   = "kubernetes"
)

func NewMeridianNode(
	action string,
	role v1.NodeRole,
	ng string,
	cloud string,
	req *v1.Request,
	labels []string,
) (*Meridian, error) {
	return &Meridian{
		request:   req,
		cloud:     cloud,
		role:      role,
		nodeGroup: ng,
		labels:    labels,
		action:    action,
	}, nil
}

type Meridian struct {
	nodeGroup string
	cloud     string
	labels    []string
	// role of node
	action  string // be one of [init|join]
	role    v1.NodeRole
	request *v1.Request
}

func (m *Meridian) EnsureNode() error {
	m.request.Name = ClusterName
	if err := m.request.Validate(); err != nil {
		return errors.Wrap(err, "validating init request")
	}

	local, err := NewLocal(m.cloud)
	if err != nil {
		return errors.Wrap(err, "new local host when")
	}

	if len(m.request.Spec.Config.Sans) == 0 {
		sans := []string{
			local.NodeIP(),
			v1.APIServerDomain,
		}
		m.request.Spec.Config.Sans = sans
	}
	klog.Infof("local host(ensure): %v", local)
	blocks, err := m.buildActionBlocks(local)
	if err != nil {
		return errors.Wrap(err, "building action blocks")
	}
	err = block.RunBlocks(blocks)
	if err != nil {
		return err
	}
	return m.saveRequest()
}

func (m *Meridian) saveRequest() error {
	mpath := "/etc/meridian/"
	if err := os.MkdirAll(mpath, 0644); err != nil {
		return err
	}

	return os.WriteFile(path.Join(mpath, "request.cfg"), []byte(tool.PrettyYaml(m.request)), 0755)
}

func (m *Meridian) DestroyNode() error {
	local, err := NewLocal(m.cloud)
	if err != nil {
		return errors.Wrap(err, "new local host when")
	}

	if len(m.request.Spec.Config.Sans) == 0 {
		m.request.Spec.Config.Sans = []string{local.NodeIP()}
	}
	klog.Infof("local host(purge): %v", local)
	etcdBlock, err := etcd.NewBlock(m.request, local, m.action)
	if err != nil {
		return errors.Wrap(err, "new etcd block while")
	}
	runtimeBlock, err := runtime.NewContainerdBlock(m.request, local)
	if err != nil {
		return errors.Wrap(err, "new runtime block while")
	}

	nvidiaBlock, err := runtime.NewNvidiaBlock(m.request, local)
	if err != nil {
		return errors.Wrap(err, "new runtime block while")
	}
	kubeletBlock, err := kubeadm.NewKubeletBlock(m.request, local, m.role, m.nodeGroup, m.labels)
	if err != nil {
		return errors.Wrap(err, "new kubelet block while")
	}

	kubeAuthBlock, err := kubeadm.NewKubeAuthBlock(m.request, local)
	if err != nil {
		return errors.Wrap(err, "new kube auth block while")
	}

	blocks := []block.Block{
		kubeAuthBlock, kubeletBlock, nvidiaBlock, runtimeBlock, etcdBlock,
	}
	ctx := context.TODO()
	for _, b := range blocks {
		err = b.Purge(ctx)
		if err != nil {
			return errors.Wrapf(err, "purge %s", b.Name())
		}
	}
	return nil
}

func (m *Meridian) buildActionBlocks(local host.Host) ([]block.Block, error) {
	etcdBlock, err := etcd.NewBlock(m.request, local, m.action)
	if err != nil {
		return nil, errors.Wrap(err, "new etcd block while")
	}
	runtimeBlock, err := runtime.NewContainerdBlock(m.request, local)
	if err != nil {
		return nil, errors.Wrap(err, "new containerd block while")
	}

	kubeletBlock, err := kubeadm.NewKubeletBlock(m.request, local, m.role, m.nodeGroup, m.labels)
	if err != nil {
		return nil, errors.Wrap(err, "new kubelet block while")
	}
	initBlock, err := kubeadm.NewInitBlock(m.request, local)
	if err != nil {
		return nil, errors.Wrap(err, "new init block while")
	}
	ccmBlock, err := kubeadm.NewCCMBlock(m.request, local)
	if err != nil {
		return nil, errors.Wrap(err, "new ccm block while")
	}
	kubeAuthBlock, err := kubeadm.NewKubeAuthBlock(m.request, local)
	if err != nil {
		return nil, errors.Wrap(err, "new kube auth block while")
	}
	kubejoinBlock, err := kubeadm.NewJoinBlock(m.request, local)
	if err != nil {
		return nil, errors.Wrap(err, "new kube join block while")
	}
	nvidiaBlock, err := runtime.NewNvidiaBlock(m.request, local)
	if err != nil {
		return nil, errors.Wrap(err, "new nvidia block while")
	}
	//postBlock, err := post.NewPostBlock(m.request, local)
	//if err != nil {
	//	return nil, errors.Wrap(err, "new post block while")
	//}
	postAddon, err := post.NewPostAddon(m.request, local)
	if err != nil {
		return nil, errors.Wrap(err, "new post addon")
	}
	var blocks []block.Block
	switch m.role {
	case v1.NodeRoleMaster:
		base := []block.Block{
			etcdBlock,
			runtimeBlock,
			kubeletBlock,
		}

		addon := []block.Block{
			ccmBlock,
			postAddon,
		}
		blocks = append(blocks, block.NewConcurrentBlock(base), nvidiaBlock, initBlock, kubeAuthBlock, block.NewConcurrentBlock(addon))
	case v1.NodeRoleWorker:
		base := []block.Block{
			runtimeBlock,
			kubeletBlock,
		}

		blocks = append(blocks, block.NewConcurrentBlock(base), nvidiaBlock, kubejoinBlock)
	}
	klog.Infof("building blocks generated for [%s] initialize", m.role)
	return blocks, nil
}

func NewLocal(pvd string) (host.Host, error) {
	info := local.NewMetaData(&local.Config{
		VpcID:     "xxx",
		VswitchID: "vvvv",
		Region:    "cn-hangzhou",
		ZoneID:    "cn-hangzhou-a",
	})
	if pvd != "" {
		info = alibaba.NewMetaDataAlibaba(nil)
	}
	return host.NewLocalHost(info)
}
