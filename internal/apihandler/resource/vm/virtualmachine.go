package vm

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/aoxn/meridian/client"
	"github.com/aoxn/meridian/internal/node/block/post/addons"
	"github.com/aoxn/meridian/internal/tool/cmd"
	"github.com/aoxn/meridian/internal/tool/sign"
	"github.com/c-robinson/iplib"
	"github.com/pkg/errors"
	"k8s.io/client-go/tools/clientcmd"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	v1 "github.com/aoxn/meridian/api/v1"
	w "github.com/aoxn/meridian/internal/meridian/worker"
	"github.com/aoxn/meridian/internal/server/service"
	"github.com/aoxn/meridian/internal/server/service/universal"
	"github.com/aoxn/meridian/internal/tool"
	"github.com/aoxn/meridian/internal/vmm/model"
	"github.com/samber/lo"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func NewVirtualMachinePvd(opt *service.Options) service.Provider {
	return &virtualMachinePvd{options: opt}
}

type virtualMachinePvd struct {
	options *service.Options
}

func (m *virtualMachinePvd) NewAPIGroup(ctx context.Context) (service.Grouped, error) {
	grp := service.Grouped{}
	t := m.addV1(grp, m.options)
	return grp, m.initPrvd(ctx, t)
}

func (m *virtualMachinePvd) addV1(grp service.Grouped, options *service.Options) *virtualMachine {
	univ := &virtualMachine{
		Store:   universal.NewUniversal(options),
		scheme:  options.Scheme,
		freezer: w.NewFreeze(),
		// allowedResource
		allowedResource: sets.New[string](),
	}
	grp.AddOrDie(univ)
	return univ
}

func (m *virtualMachinePvd) initPrvd(ctx context.Context, t *virtualMachine) error {
	work, err := w.NewWorkerMgr(ctx, "meridian", t.HandleTask, t.freezer)
	if err != nil {
		return err
	}
	t.work = work

	klog.Infof("virtualMachine worker started")
	go t.healthPolling(ctx)
	return t.InitTask()
}

type virtualMachine struct {
	service.Store
	work            *w.WorkerMgr
	freezer         *w.Action
	scheme          *runtime.Scheme
	allowedResource sets.Set[string]
}

func (m *virtualMachine) GVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    v1.GroupVersion.Group,
		Version:  v1.GroupVersion.Version,
		Resource: "virtualmachines",
	}
}

func (m *virtualMachine) isAllowed(r string) bool {
	r = strings.ToLower(r)
	if m.allowedResource.Len() == 0 {
		return true
	}
	return m.allowedResource.Has(r)
}

func (m *virtualMachine) Update(ctx context.Context, object runtime.Object, options *metav1.UpdateOptions) (runtime.Object, error) {
	if object == nil {
		return nil, service.NewUnknownObjectError("")
	}
	gvk := object.GetObjectKind().GroupVersionKind()
	if !m.isAllowed(gvk.Kind) {
		return nil, service.NewNotAllowedError(gvk.String())
	}
	o, ok := object.(*v1.VirtualMachine)
	if !ok {
		return o, fmt.Errorf("not a virtual machine")
	}
	klog.Infof("[%s]vm update", o.Name)
	vmo, err := m.Store.Get(ctx, o, &metav1.GetOptions{})
	if err != nil {
		return o, err
	}
	vm, ok := vmo.(*v1.VirtualMachine)
	if !ok {
		return vmo, fmt.Errorf("not a virtual machine")
	}
	err = m.SendTask(vm)
	if err != nil {
		return vmo, errors.Wrapf(err, "send update task")
	}
	return m.Store.Update(ctx, vm, options)
}

func (m *virtualMachine) List(ctx context.Context, out runtime.Object, options *metav1.ListOptions) (runtime.Object, error) {
	o, err := m.Store.List(ctx, out, options)
	if err != nil {
		return o, err
	}
	obj, ok := out.(*v1.VirtualMachineList)
	if !ok {
		return o, fmt.Errorf("not a virtual machine list")
	}
	for i := range obj.Items {
		inst, err := model.NewInstance(&obj.Items[i])
		if err != nil {
			continue
		}
		obj.Items[i].Status.Message = inst.Message
		obj.Items[i].Status.Phase = inst.Status.Phase
		klog.V(8).Infof("[%s]list vm: [%s],  %s", inst.Name, obj.Items[i].Status.Phase, obj.Items[i].Status.Message)
	}
	return obj, nil
}

func (m *virtualMachine) Get(ctx context.Context, object runtime.Object, options *metav1.GetOptions) (runtime.Object, error) {
	if object == nil {
		return nil, service.NewUnknownObjectError("")
	}
	gvk := object.GetObjectKind().GroupVersionKind()
	if !m.isAllowed(gvk.Kind) {
		return nil, service.NewNotAllowedError(gvk.String())
	}
	o, ok := object.(*v1.VirtualMachine)
	if !ok {
		return o, fmt.Errorf("not a virtual machine")
	}
	klog.Infof("[%s]vm get", o.Name)
	vmo, err := m.Store.Get(ctx, o, &metav1.GetOptions{})
	if err != nil {
		return o, err
	}
	vm, ok := vmo.(*v1.VirtualMachine)
	if !ok {
		return vmo, fmt.Errorf("not a virtual machine")
	}
	inst, err := model.NewInstance(vm)
	if err != nil {
		return vmo, err
	}
	vm.Status.Message = inst.Message
	vm.Status.Phase = inst.Status.Phase
	return inst, nil
}

func (m *virtualMachine) Create(ctx context.Context, object runtime.Object, options *metav1.CreateOptions) (runtime.Object, error) {
	if object == nil {
		return nil, service.NewUnknownObjectError("")
	}
	gvk := object.GetObjectKind().GroupVersionKind()
	if !m.isAllowed(gvk.Kind) {
		return nil, service.NewNotAllowedError(gvk.String())
	}
	vm, ok := object.(*v1.VirtualMachine)
	if !ok {
		return object, nil
	}
	_, err := m.Store.Get(ctx, object, nil)
	if err == nil {
		return nil, fmt.Errorf("vm[%s] already exists", vm.Name)
	}
	// set macAddr
	for i, _ := range vm.Spec.Networks {
		network := vm.Spec.Networks[i]
		if network.MACAddress == "" {
			network.MACAddress = v1.GenMAC()
			klog.V(5).Infof("set mac address [%s] for %s", network.MACAddress, network.Interface)
		}
		vm.Spec.Networks[i] = network
		klog.V(5).Infof("mac address is: %s for %s", network.MACAddress, network.Interface)
	}

	err = setVmDefault(vm)
	if err != nil {
		return object, err
	}
	err = m.allocateAddress(ctx, vm)
	if err != nil {
		return object, err
	}
	ret, err := m.Store.Create(ctx, object, options)
	if err != nil {
		return nil, err
	}
	return ret, m.SendTask(vm)
}

func setVmDefault(vm *v1.VirtualMachine) error {

	home, err := model.MdHOME()
	if err != nil {
		return err
	}
	vm.Spec.SetPortForward(v1.PortForward{
		Proto:     "unix",
		VSockPort: 10443,
		Source:    fmt.Sprintf("/tmp/%s.sock", vm.Name),
	})
	vm.Spec.SetPortForward(v1.PortForward{
		Proto:       "unix",
		VSockPort:   10240,
		Destination: "/var/run/docker.sock",
		Source:      filepath.Join(home, vm.Name, "docker.sock"),
	})
	port, err := strconv.Atoi(vm.Spec.Request.AccessPoint.APIPort)
	if err != nil {
		return err
	}
	vm.Spec.SetPortForward(v1.PortForward{
		Proto:       "tcp",
		VSockPort:   40443,
		Destination: "apiserver",
		Source:      fmt.Sprintf("0.0.0.0:%d", port),
	})
	vm.Spec.SetMounts(v1.Mount{
		Writable:   true,
		Location:   fmt.Sprintf("~/mdata/%s", vm.Name),
		MountPoint: "/mnt/disk0",
	})
	addons.SetDftClusterAddons(&vm.Spec.Request)
	return nil
}

var (
	defaultGateway = "192.168.64.1"
	defaultCIDR    = "192.168.64.1/24"
)

func (m *virtualMachine) allocateAddress(ctx context.Context, vm *v1.VirtualMachine) error {
	_, needAllocate := lo.Find(vm.Spec.Networks, func(item v1.Network) bool {
		return item.Address == ""
	})
	if !needAllocate {
		return nil
	}
	var vms = v1.VirtualMachineList{}
	_, err := m.List(ctx, &vms, &metav1.ListOptions{})
	if err != nil {
		return err
	}

	networks := lo.FlatMap(vms.Items, func(item v1.VirtualMachine, index int) []v1.Network {
		if item.Name == vm.Name {
			return nil
		}
		return item.Spec.Networks
	})

	var allocated map[string]string
	allocated = lo.FilterSliceToMap(networks, func(item v1.Network) (string, string, bool) {
		if item.Address == "" {
			return "", "", false
		}
		return item.Address, item.Address, true
	})

	klog.Infof("address has been allocated: %s", allocated)
	ip, _, err := net.ParseCIDR(defaultCIDR)
	if err != nil {
		return errors.Wrapf(err, "error parsing default cidr %s", defaultCIDR)
	}
	n := iplib.NewNet4(ip, 24)
	for index, _ := range vm.Spec.Networks {
		succeed := false
		for i := 0; i < 255; i++ {
			ip, err = n.NextIP(ip)
			if err != nil {
				return err
			}
			klog.V(6).Infof("search for vm ip: %s", ip)
			if ip.String() == "192.168.64.1" || ip.String() == "192.168.64.0" {
				continue
			}
			_, ok := allocated[n.String()]
			if ok {
				continue
			}
			succeed = true
			vm.Spec.Networks[index].IpGateway = defaultGateway
			vm.Spec.Networks[index].Address = fmt.Sprintf("%s/24", ip.String())
			break
		}
		if !succeed {
			return fmt.Errorf("no available ip address")
		}
	}
	return nil
}

func (m *virtualMachine) Delete(ctx context.Context, object runtime.Object, options *metav1.DeleteOptions) (runtime.Object, error) {
	if object == nil {
		return nil, service.NewUnknownObjectError("")
	}
	gvk := object.GetObjectKind().GroupVersionKind()
	if !m.isAllowed(gvk.Kind) {
		return nil, service.NewNotAllowedError(gvk.String())
	}
	obj, err := m.Store.Get(ctx, object, nil)
	if err != nil {
		return nil, err
	}
	vm, ok := obj.(*v1.VirtualMachine)
	if !ok {
		return m.Store.Delete(ctx, object, options)
	}
	inst, err := model.NewInstance(vm)
	if err != nil {
		return obj, err
	}
	err = inst.Destroy()
	if err != nil {
		return obj, err
	}
	m.work.CancelBy(vm.Name)
	err = m.DeleteDockerContext(vm)
	if err != nil {
		return obj, err
	}
	err = m.CleanUpKubernetesContext(vm)
	if err != nil {
		return obj, err
	}
	return m.Store.Delete(ctx, object, options)
}

func (m *virtualMachine) DeleteCollection(ctx context.Context, options *metav1.DeleteOptions) (runtime.Object, error) {

	return m.Store.DeleteCollection(ctx, options)
}

func (m *virtualMachine) InitTask() error {
	gvr := m.GVR()
	o := m.NewList(&gvr)
	_, err := m.List(context.TODO(), o, nil)
	if err != nil {
		return err
	}
	err = meta.EachListItem(o, func(item runtime.Object) error {
		virt, ok := item.(*v1.VirtualMachine)
		if !ok {
			klog.Warningf("unexpected item type: %T", item)
			return nil
		}
		switch virt.Status.Phase {
		case v1.TaskFail, v1.TaskSuccess, v1.Running:
			klog.Infof("skip [%s] virtualMachine[%s]", virt.Status.Phase, virt.Name)
			return nil
		default:
		}
		klog.Infof("send init virtualMachine: %+v", virt.Name)
		return m.SendTask(virt)
	})
	klog.Infof("init task list: [%d]", meta.LenList(o))
	return err
}

func (m *virtualMachine) SendTask(task *v1.VirtualMachine) error {
	klog.V(5).Infof("[%s]send task", task.Name)
	m.work.Enqueue(task.Name)
	return nil
}

func (m *virtualMachine) HandleTask(ctx context.Context, req *w.Request, rep *w.Response) error {

	var (
		err error
		vm  = v1.EmptyVM(req.Key)
	)
	_, err = m.Store.Get(ctx, vm, nil)
	if err != nil {
		return err
	}
	klog.V(5).Infof("virtualMachine handle task: [%s]", tool.PrettyYaml(vm))
	return m.handleVm(ctx, vm)
}

type counter struct {
	count         int
	notRetryUntil time.Time
}

var mu sync.Mutex

type readiness map[string]*counter

var cnt readiness = make(map[string]*counter)

func (n readiness) set(name string) {
	mu.Lock()
	defer mu.Unlock()
	abc, ok := n[name]
	if !ok {
		n[name] = &counter{count: 0}
		return
	}
	abc.count++
}

func (n readiness) reset(name string, next time.Time) {
	mu.Lock()
	defer mu.Unlock()
	n.clear(name, next)
}

func (n readiness) clear(name string, next time.Time) {
	n[name] = &counter{count: 0, notRetryUntil: next}
}

func (n readiness) send(name string) bool {
	mu.Lock()
	defer mu.Unlock()
	abc, ok := n[name]
	if ok {
		if abc.count < 6 {
			return false
		}
		if abc.notRetryUntil.IsZero() ||
			abc.notRetryUntil.After(time.Now()) {
			n.clear(name, time.Now().Add(5*time.Minute))
			return true
		}
		return false
	}
	return false
}

func (m *virtualMachine) healthPolling(ctx context.Context) {

	tick := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-tick.C:
			klog.V(8).Infof("health tick check for vms...")
			vmList := &v1.VirtualMachineList{
				TypeMeta: metav1.TypeMeta{
					APIVersion: v1.GroupVersion.String(),
					Kind:       "VirtualMachineList",
				},
			}
			_, err := m.List(ctx, vmList, nil)
			if err != nil {
				klog.Infof("list all vm failed: %s", err.Error())
				continue
			}

			wg := sync.WaitGroup{}
			check := func(vm *v1.VirtualMachine) {
				defer wg.Done()

				endpoint := fmt.Sprintf("/tmp/guest-%s.sock", vm.Name)
				us, err := client.Client(endpoint)
				if err != nil {
					klog.Errorf("new guest client: %s", err.Error())
					return
				}
				healthy := v1.Healthy{}
				err = us.Raw().Get().PathPrefix("/").Resource("health").Do(&healthy)
				if err == nil {
					cnt.reset(vm.Name, time.Time{})
					return
				}
				cnt.set(vm.Name)
				if cnt.send(vm.Name) {
					klog.Infof("[%-10s] vm tick expired, trigger reconcile", vm.Name)
					err = m.SendTask(vm)
					klog.Infof("[%-10s] vm tick expired, reconcile finished: [%v]", vm.Name, err)
					return
				}
				klog.Infof("vm [%s] healthy check failed: %s", vm.Name, err.Error())
			}
			for _, vm := range vmList.Items {
				wg.Add(1)
				go check(&vm)
			}
			wg.Wait()
			time.Sleep(5 * time.Second)
		case <-ctx.Done():
			klog.Infof("health polling returned")
			return
		}
	}
}

func (m *virtualMachine) handleVm(ctx context.Context, vm *v1.VirtualMachine) error {

	inst, err := model.NewInstance(vm)
	if err != nil {
		return err
	}
	begin := time.Now() // used for logrus propagation

	pid, err := inst.LoadPID()
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		// not exist
		klog.Infof("[%-10s]pid file not exist: [%s], %s", vm.Name, pid.PID, err.Error())
	} else {
		klog.Infof("[%-10s]got existing pid [%s]", vm.Name, pid.PID)
	}
	if pid.PID == "" || time.Now().After(pid.Stamp.Add(30*time.Second)) {
		klog.Infof("[%-10s]pid not found or expired: [%s]", vm.Name, pid)
		_ = os.MkdirAll(inst.Dir, 0o700)
		vmBin, err := vmBinaryPath()
		if err != nil {
			return err
		}
		klog.Infof("boot vm from: %s", vmBin)
		haStdoutPath := filepath.Join(inst.Dir, v1.HostAgentStdoutLog)
		haStderrPath := filepath.Join(inst.Dir, v1.HostAgentStderrLog)
		if err := os.RemoveAll(haStdoutPath); err != nil {
			return err
		}
		if err := os.RemoveAll(haStderrPath); err != nil {
			return err
		}
		haStdoutW, err := os.Create(haStdoutPath)
		if err != nil {
			return err
		}
		// no defer haStdoutW.Close()
		haStderrW, err := os.Create(haStderrPath)
		if err != nil {
			return err
		}
		// no defer haStderrW.Close()
		var args = []string{"start"}
		haCmd := exec.CommandContext(ctx, vmBin, args...)

		haCmd.Stdin = strings.NewReader(tool.PrettyYaml(vm))
		haCmd.Stdout = haStdoutW
		haCmd.Stderr = haStderrW
		haCmd.SysProcAttr = &syscall.SysProcAttr{
			Setsid: true,
		}

		if err := haCmd.Start(); err != nil {
			return err
		}
		klog.Infof("[%-10s]vm started: [%s], %s", vm.Name, pid.PID, strings.Join(append([]string{vmBin}, args...), " "))
	} else {
		klog.Infof("[%-10s]virtual machine already started: [%s]", vm.Name, pid.PID)
	}
	klog.Infof("[%-10s]vm started in %f(s)", vm.Name, time.Now().Sub(begin).Seconds())
	merdiand, err := client.Client(endpoint(vm.Name))
	if err != nil {
		return err
	}
	if err := m.WaitHostAgentStart(ctx, merdiand, vm); err != nil {
		return errors.Wrapf(err, "wait host agent: %s", vm.Name)
	}
	_, err = m.Store.Update(ctx, vm, &metav1.UpdateOptions{})

	if err = m.EnsureKubernetes(ctx, merdiand, vm); err != nil {
		return errors.Wrapf(err, "ensure kubernetes: %s", vm.Name)
	}

	if err = m.EnsureDockerContext(vm); err != nil {
		return errors.Wrapf(err, "ensure docker context: %s", vm.Name)
	}
	_, err = m.Store.Update(ctx, vm, &metav1.UpdateOptions{})
	return err
}

func vmBinaryPath() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	link, err := filepath.EvalSymlinks(self)
	if err != nil {
		return "", err
	}
	return path.Join(path.Dir(link), "meridian-vm"), nil
}

func endpoint(name string) string {
	return fmt.Sprintf("/tmp/%s.sock", name)
}

func (m *virtualMachine) EnsureDockerContext(vm *v1.VirtualMachine) error {
	docker := "/usr/local/bin/docker"
	content := []string{"context", "inspect", vm.Name}
	r := <-cmd.NewCmd(docker, content...).Start()
	err := cmd.CmdError(r)
	if err != nil {
		content = []string{
			"context",
			"create",
			vm.Name,
			"--docker", fmt.Sprintf("host=unix://%s", vm.GetDockerEndpoint()),
			"--description", "meridian docker endpoint",
		}
		r = <-cmd.NewCmd(docker, content...).Start()
		return cmd.CmdError(r)
	}
	content = []string{
		"context",
		"update",
		vm.Name,
		"--docker", fmt.Sprintf("host=unix://%s", vm.GetDockerEndpoint()),
	}
	r = <-cmd.NewCmd(docker, content...).Start()
	return cmd.CmdError(r)

}

func getAddress(addrs []string) string {
	for _, addr := range addrs {
		if strings.HasPrefix(addr, "192.168") {
			return addr
		}
	}
	return ""
}

func (m *virtualMachine) EnsureKubernetesContext(vm *v1.VirtualMachine) error {
	root := vm.Spec.Request.Config.TLS["root"]
	if root == nil {
		klog.Warningf("unexpected root tls config: vm.Spec.Request.Spec.TLS")
		return nil
	}
	addr := getAddress(vm.Status.Address)
	if addr == "" {
		klog.Warningf("unexpected empty address: vm.Status.Address[%s]", vm.Status.Address)
		return nil
	}
	key, crt, err := sign.SignKubernetesClient(root.Cert, root.Key, []string{})
	if err != nil {
		return fmt.Errorf("sign kubernetes client crt for %s: %s", vm.Name, err.Error())
	}

	data, err := tool.RenderConfig(
		fmt.Sprintf("%s@%s", v1.MeridianUserName(vm.Name), v1.MeridianClusterName(vm.Name)),
		tool.KubeConfigTpl,
		tool.RenderParam{
			AuthCA:      base64.StdEncoding.EncodeToString(root.Cert),
			Address:     addr,
			Port:        "6443",
			ClusterName: v1.MeridianClusterName(vm.Name),
			UserName:    v1.MeridianUserName(vm.Name),
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
		klog.Warningf("ensure kubeconfig context: %s", err.Error())
		return nil
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
		userName    = v1.MeridianUserName(vm.Name)
		clusterName = v1.MeridianClusterName(vm.Name)
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

func (m *virtualMachine) CleanUpKubernetesContext(vm *v1.VirtualMachine) error {

	home, err := os.UserHomeDir()
	if err != nil {
		klog.Warningf("ensure kubeconfig context: %s", err.Error())
		return nil
	}
	cfg, err := clientcmd.LoadFromFile(filepath.Join(home, ".kube", "config"))
	if err != nil {
		klog.Warningf("ensure kubeconfig context: %s", err.Error())
		return nil
	}
	var (
		write       = false
		userName    = v1.MeridianUserName(vm.Name)
		clusterName = v1.MeridianClusterName(vm.Name)
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

func (m *virtualMachine) DeleteDockerContext(vm *v1.VirtualMachine) error {
	content := []string{"context", "rm", "-f", vm.Name}
	r := <-cmd.NewCmd("/usr/local/bin/docker", content...).Start()
	err := cmd.CmdError(r)
	if err != nil {
		klog.Warningf("[%-10s]docker context rm %s: %s", vm.Name, vm.Name, err.Error())
	}
	return nil
}

func (m *virtualMachine) EnsureKubernetes(ctx context.Context, client client.Interface, vm *v1.VirtualMachine) error {
	gi := v1.EmptyGI(vm.Name)
	err := client.Get(ctx, gi)
	if err != nil {
		return err
	}
	klog.Infof("[%-10s] guest vm status: %+v", vm.Name, gi.Status.Conditions)
	cond := meta.FindStatusCondition(gi.Status.Conditions, "Kubernetes")
	if cond == nil ||
		cond.Status == metav1.ConditionUnknown || cond.Reason == "NotInstalled" {
		klog.Infof("[%-10s] unknown Kubernetes condition, create k8s", vm.Name)
		req := v1.NewEmptyRequest(vm.Name, vm.Spec.Request)
		err = client.Create(ctx, req)
		if err != nil {
			return errors.Wrapf(err, "create k8s request")
		}
		klog.Infof("[%-10s]k8s request created, wait response", vm.Name)
	}
	// todo repair node
	pollFunc := func(ctx context.Context) (bool, error) {
		km := v1.EmptyVM(vm.Name)
		_, err := m.Get(ctx, km, nil)
		if err != nil {
			if IsNotFound(err) {
				klog.Errorf("[%-10s] vm not found: [%s]", vm.Name, km.Name)
				return true, err
			}
			klog.Infof("find vm [%s] error: %v", vm.Name, err.Error())
			return false, nil
		}
		err = client.Get(ctx, gi)
		if err != nil {
			klog.Infof("[%-10s] ensure kubernetes, wait host agent start: %v", vm.Name, err)
			return false, nil
		}
		if len(vm.Status.Address) == 0 {
			klog.Infof("[%-10s]ensure kubernetes wait guest address: %s", vm.Name, vm.Status.Phase)
			return false, nil
		}
		klog.Infof("[%-10s] ensure kubernetes, wait guest responed with: %+v", vm.Name, gi.Status)
		cond = meta.FindStatusCondition(gi.Status.Conditions, "Kubernetes")
		if cond == nil {
			klog.Infof("[%-10s]kubernetes condition not found yet", vm.Name)
			return false, nil
		}
		if cond.Status == metav1.ConditionTrue &&
			gi.Status.Phase == v1.Running {
			return true, nil
		}
		klog.Infof("[%-10s]kubernetes condition found, Status=%s, %s, %s", vm.Name, cond.Status, gi.Status.Phase, cond.Reason)
		return cond.Status == metav1.ConditionTrue && gi.Status.Phase == v1.Running, nil
	}
	err = wait.PollUntilContextTimeout(ctx, 3*time.Second, 8*time.Minute, false, pollFunc)
	if err != nil {
		return err
	}
	return m.EnsureKubernetesContext(vm)
}

func (m *virtualMachine) WaitHostAgentStart(ctx context.Context, client client.Interface, vm *v1.VirtualMachine) error {

	gi := v1.EmptyGI(vm.Name)
	pollFunc := func(ctx context.Context) (bool, error) {
		err := client.Get(ctx, gi)
		if err != nil {
			klog.Infof("[%-10s]wait host agent start: %v", vm.Name, err)
			return false, nil
		}
		vm.Status.Phase = gi.Status.Phase
		vm.Status.Address = gi.Spec.Address
		klog.Infof("[%-10s]guest agent responed with: %+v", vm.Name, vm.Status)
		if len(vm.Status.Address) == 0 {
			klog.Infof("[%-10s]wait guest address: [%s]", vm.Name, vm.Status)
			return false, nil
		}
		return gi.Status.Phase == v1.Running, nil
	}
	return wait.PollUntilContextTimeout(ctx, 3*time.Second, 2*time.Minute, false, pollFunc)
}
func newUnixSocketDialer(sock string) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, proto, addr string) (conn net.Conn, err error) {
		return net.Dial("unix", sock)
	}
}

func IsNotFound(err error) bool {
	return strings.Contains(err.Error(), "NotFound")
}
