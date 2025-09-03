package core

import (
	"context"
	"encoding/base64"
	"fmt"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/tool"
	"github.com/aoxn/meridian/internal/tool/sign"
	"github.com/aoxn/meridian/internal/vmm/meta"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/cluster-bootstrap/token/util"
	"k8s.io/klog/v2"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

func NewK8sMgr(stateMgr *vmStateMgr) (*LocalK8sMgr, error) {
	var err error
	mgr := &LocalK8sMgr{
		vmStateMgr: stateMgr,
	}
	mgr.stateStore, err = mgr.newK8sStateStore(stateMgr.meta)
	return mgr, err
}

type LocalK8sMgr struct {
	tskMgr     *taskMgr
	vmStateMgr *vmStateMgr
	stateStore *k8sStateStore
}

func (mgr *LocalK8sMgr) Create(ctx context.Context, k8s *meta.Kubernetes) error {
	vm := mgr.vmStateMgr.Get(k8s.VmName)
	if vm == nil || vm.machine == nil {
		return fmt.Errorf("the vm %s not found", k8s.VmName)
	}
	l := mgr.stateStore.Get(k8s.Name)
	if l != nil {
		return fmt.Errorf("k8s %s already exists", k8s.Name)
	}
	kstate, err := mgr.stateStore.Create(&meta.Kubernetes{
		Name: k8s.Name, Spec: k8s.Spec, State: "Created", VmName: k8s.VmName,
	})
	if err != nil {
		return errors.Wrap(err, "create kubernetes error")
	}
	if kstate.tryLock() {
		go func() {
			err = kstate.deploy(context.TODO())
			if err != nil {
				klog.Errorf("deploy kubernetes %s: %s", k8s.Name, err.Error())
			}
		}()
		return nil
	}

	return fmt.Errorf("unexpected another k8s in creating")
}

func (mgr *LocalK8sMgr) Redeploy(ctx context.Context, k8s *meta.Kubernetes) error {
	kstate := mgr.stateStore.Get(k8s.Name)
	if kstate == nil {
		return fmt.Errorf("k8s %s does not exists", k8s.Name)
	}

	vm := mgr.vmStateMgr.Get(kstate.k8s.VmName)
	if vm == nil || vm.machine == nil {
		return fmt.Errorf("correspond vm %s not found", k8s.VmName)
	}
	if kstate.tryLock() {
		go func() {
			err := kstate.deploy(context.TODO())
			if err != nil {
				klog.Errorf("deploy kubernetes %s: %s", k8s.Name, err.Error())
			}
		}()
		return nil
	}
	return fmt.Errorf("another deploying is in progress: %s, wait for timeout", k8s.Name)
}

func (mgr *LocalK8sMgr) Destroy(ctx context.Context, at string) error {

	kstate := mgr.stateStore.Get(at)
	if kstate == nil || kstate.k8s == nil {
		return fmt.Errorf("k8s %s not found", at)
	}
	vm := mgr.vmStateMgr.Get(kstate.k8s.VmName)
	if vm.machine == nil {
		klog.Errorf("correspond vm %s not found", kstate.k8s.VmName)
		return mgr.stateStore.Delete(at)
	}

	return mgr.stateStore.Delete(at)
}

func (mgr *LocalK8sMgr) newK8sStateStore(bk meta.Backend) (*k8sStateStore, error) {
	machines, err := bk.K8S().List()
	if err != nil {
		return nil, errors.Wrap(err, "read k8s config error")
	}
	var vms = make(map[string]*k8sState)

	for _, k8s := range machines {
		vm := mgr.vmStateMgr.Get(k8s.VmName)
		vms[k8s.Name] = &k8sState{
			name:    k8s.Name,
			k8s:     k8s,
			meta:    bk,
			vmState: vm,
			mu:      &sync.RWMutex{},
		}
	}
	return &k8sStateStore{mu: &sync.RWMutex{}, vmStateMgr: mgr.vmStateMgr, k8s: vms, meta: bk}, nil
}

type k8sStateStore struct {
	mu         *sync.RWMutex
	meta       meta.Backend
	k8s        map[string]*k8sState
	vmStateMgr *vmStateMgr
}

func (mgr *k8sStateStore) Get(name string) *k8sState {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	return mgr.k8s[name]
}

func (mgr *k8sStateStore) List() []*meta.Kubernetes {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	transFn := func(key string, value *k8sState) *meta.Kubernetes {
		return value.k8s
	}
	return lo.MapToSlice(mgr.k8s, transFn)
}

func (mgr *k8sStateStore) Delete(name string) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	kstate, ok := mgr.k8s[name]
	if !ok {
		return fmt.Errorf("k8s %s not found", name)
	}
	err := kstate.destroy()
	if err != nil {
		return errors.Wrapf(err, "destroy k8s failed")
	}
	delete(mgr.k8s, name)
	return mgr.meta.K8S().Remove(&meta.Kubernetes{Name: name})
}

func (mgr *k8sStateStore) Create(k8s *meta.Kubernetes) (*k8sState, error) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	_, ok := mgr.k8s[k8s.Name]
	if ok {
		return nil, fmt.Errorf("k8s %s already exists", k8s.Name)
	}
	vm := mgr.vmStateMgr.Get(k8s.VmName)
	if vm == nil {
		return nil, fmt.Errorf("vm %s does not exist", k8s.Name)
	}
	k8s.State, k8s.Message = Created, fmt.Sprintf("machine %s created", k8s.Name)
	state := &k8sState{
		name:    k8s.Name,
		k8s:     k8s,
		meta:    mgr.meta,
		vmState: vm,
		mu:      &sync.RWMutex{},
	}
	mgr.k8s[k8s.Name] = state
	err := mgr.meta.K8S().Create(k8s)
	return state, err
}

type k8sState struct {
	name      string
	deploying bool
	mu        *sync.RWMutex
	k8s       *meta.Kubernetes
	meta      meta.Backend
	vmState   *vmState
	cancelFn  context.CancelFunc
}

func (st *k8sState) setState(state string, msg ...any) {
	st.k8s.State, st.k8s.Message = state, fmtMessage(msg...)
	err := st.meta.K8S().Update(st.k8s)
	if err != nil {
		klog.Errorf("update k8s %s state failed: %v", st.k8s.Name, err)
	}
}

func (st *k8sState) destroy() error {
	err := st.cleanUpK8sContext(st.name)
	if err != nil {
		klog.Errorf("failed to clean up k8s context: %s", err)
	}
	out, err := st.vmState.SSH().RunCommand(
		context.TODO(), st.vmState.name, getK8sCmd(ActionDestroy, st.k8s))
	if err != nil {
		klog.Infof("destroy command result: %s", out)
		return errors.Wrap(err, "run destroy k8s command")
	}
	return nil
}

func (st *k8sState) tryLock() bool {
	st.mu.Lock()
	defer st.mu.Unlock()
	if !st.deploying {
		st.deploying = true
		return st.deploying
	}
	return false
}

func (st *k8sState) deploy(ctx context.Context) error {
	free := func() {
		st.mu.Lock()
		defer st.mu.Unlock()
		st.deploying = false
	}
	defer free()
	st.setState(Deploying, "k8s is in deploying")
	out, err := st.vmState.SSH().RunCommand(ctx, st.k8s.VmName, getK8sCmd(ActionInstall, st.k8s))
	if err != nil {
		st.setState(Error, "run command: %v", err.Error())
		klog.Errorf("run deploy command result: %s, %s", err.Error(), out)
		return errors.Wrap(err, "run install k8s command")
	}
	klog.Infof("install command result: %s", string(out))
	addr := lo.FirstOr(st.vmState.machine.Spec.Networks, v1.Network{})
	if addr.Address == "" {
		st.setState(Error, "vm address not found: %v", addr)
		return fmt.Errorf("unexpected empty address: %s", st.k8s.Name)
	}

	err = st.setKubernetesContext(st.k8s, strings.Split(addr.Address, "/")[0])
	if err == nil {
		st.setState(Running, "k8s is running")
		return nil
	}
	st.setState(Error, "deploy k8s context failed: %s", err.Error())
	return errors.Wrap(err, "set k8s context")
}

func (st *k8sState) setKubernetesContext(k8s *meta.Kubernetes, addr string) error {
	root := k8s.Spec.Config.TLS["root"]
	if root == nil {
		return fmt.Errorf("unexpect empty root certificate: %s", k8s.Name)
	}
	key, crt, err := sign.SignKubernetesClient(root.Cert, root.Key, []string{})
	if err != nil {
		return fmt.Errorf("sign kubernetes client crt for %s: %s", k8s.Name, err.Error())
	}

	data, err := tool.RenderConfig(
		fmt.Sprintf("%s@%s", v1.MeridianUserName(k8s.Name), v1.MeridianClusterName(k8s.Name)),
		tool.KubeConfigTpl,
		tool.RenderParam{
			AuthCA:      base64.StdEncoding.EncodeToString(root.Cert),
			Address:     addr,
			Port:        "6443",
			ClusterName: v1.MeridianClusterName(k8s.Name),
			UserName:    v1.MeridianUserName(k8s.Name),
			ClientCRT:   base64.StdEncoding.EncodeToString(crt),
			ClientKey:   base64.StdEncoding.EncodeToString(key),
		},
	)
	if err != nil {
		return fmt.Errorf("render kube config error: %s", err.Error())
	}
	gencfg, err := clientcmd.Load([]byte(data))
	if err != nil {
		return fmt.Errorf("load kube config error: %s", err.Error())
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("unexpected home dir: %s", err.Error())
	}
	kcfg := filepath.Join(home, ".kube", "config")
	_, err = os.Stat(kcfg)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(filepath.Join(home, ".kube"), 0755)
			if err != nil {
				return err
			}
			return clientcmd.WriteToFile(*gencfg, kcfg)
		}
		return err
	}
	cfg, err := clientcmd.LoadFromFile(kcfg)
	if err != nil {
		klog.Warningf("ensure kubeconfig context: %s", err.Error())
		return nil
	}
	var (
		userName    = v1.MeridianUserName(k8s.Name)
		clusterName = v1.MeridianClusterName(k8s.Name)
	)
	if cfg.Clusters == nil {
		cfg.Clusters = make(map[string]*clientcmdapi.Cluster)
	}
	if cfg.AuthInfos == nil {
		cfg.AuthInfos = make(map[string]*clientcmdapi.AuthInfo)
	}
	if cfg.Contexts == nil {
		cfg.Contexts = make(map[string]*clientcmdapi.Context)
	}
	cfg.Clusters[clusterName] = gencfg.Clusters[clusterName]
	cfg.AuthInfos[userName] = gencfg.AuthInfos[userName]
	ctx := fmt.Sprintf("%s@%s", userName, clusterName)
	cfg.Contexts[ctx] = gencfg.Contexts[ctx]
	return clientcmd.WriteToFile(*cfg, kcfg)
}

func (st *k8sState) cleanUpK8sContext(name string) error {

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("unexpected home dir: %s", err.Error())
	}
	cfg, err := clientcmd.LoadFromFile(filepath.Join(home, ".kube", "config"))
	if err != nil {
		return fmt.Errorf("load kube config error: %s", err.Error())
	}
	var (
		write       = false
		userName    = v1.MeridianUserName(name)
		clusterName = v1.MeridianClusterName(name)
	)
	_, ok := cfg.Clusters[clusterName]
	if ok {
		write = true
		delete(cfg.Clusters, clusterName)
	}
	_, ok = cfg.AuthInfos[userName]
	if ok {
		write = true
		delete(cfg.AuthInfos, userName)
	}
	ctx := fmt.Sprintf("%s@%s", userName, clusterName)
	_, ok = cfg.Contexts[ctx]
	if ok {
		write = true
		delete(cfg.Contexts, ctx)
	}
	if !write {
		return nil
	}
	return clientcmd.WriteToFile(*cfg, filepath.Join(home, ".kube", "config"))
}

func getK8sCmd(action string, k *meta.Kubernetes) string {
	req := v1.Request{
		ObjectMeta: metav1.ObjectMeta{
			Name: k.Name,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Request",
			APIVersion: "xdpin.cn/v1",
		},
		Spec: k.Spec,
	}
	var tpl = `
cat >config.yml << EOF
%s
EOF
`
	var command []string
	switch action {
	case ActionInstall:
		command = []string{
			base,
			fmt.Sprintf(tpl, tool.PrettyYaml(req)),
			"sudo rm -rf /etc/kubernetes/super-admin.conf",
			fmt.Sprintf("sudo /usr/local/bin/meridian-node init config.yml"),
		}
	case ActionDestroy:
		command = []string{
			base,
			fmt.Sprintf(tpl, tool.PrettyYaml(req)),
			"sudo /usr/local/bin/meridian-node destroy config.yml",
		}
	}
	return strings.Join(command, "\n")
}

func DftRequest() (*v1.RequestSpec, error) {

	token, err := util.GenerateBootstrapToken()
	if err != nil {
		return nil, err
	}
	randPort := rand.Intn(1000)
	req := v1.RequestSpec{
		Config: v1.ClusterConfig{
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
	}
	err = setRootCA(&req)
	if err != nil {
		return nil, errors.Wrap(err, "set root CA")
	}
	req.SetDefault()
	return &req, nil
}

func setRootCA(req *v1.RequestSpec) error {
	newRootCA := func() (*v1.KeyCert, error) {
		key, crt, err := sign.SelfSignedPair()
		if err != nil {
			return nil, err
		}
		return &v1.KeyCert{Key: key, Cert: crt}, nil
	}

	newCA4SA := func() (*v1.KeyCert, error) {
		key, crt, err := sign.SelfSignedPairSA()
		if err != nil {
			return nil, err
		}
		return &v1.KeyCert{Key: key, Cert: crt}, nil
	}
	root, err := newRootCA()
	if err != nil {
		return err
	}
	frontProxy, err := newRootCA()
	if err != nil {
		return err
	}
	svc, err := newCA4SA()
	if err != nil {
		return err
	}
	etcdPeer, err := newRootCA()
	if err != nil {
		return err
	}
	etcdServer, err := newRootCA()
	if err != nil {
		return err
	}
	req.Config.TLS = map[string]*v1.KeyCert{
		"root":        root,
		"svc":         svc,
		"front-proxy": frontProxy,
		"etcd-peer":   etcdPeer,
		"etcd-server": etcdServer,
	}
	return nil
}
