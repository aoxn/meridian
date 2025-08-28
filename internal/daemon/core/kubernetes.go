package core

import (
	"strings"
)

func getAddress(addrs []string) string {
	for _, addr := range addrs {
		if strings.HasPrefix(addr, "192.168") {
			return addr
		}
	}
	return ""
}

//
//func (mgr *LocalDockerMgr) setKubernetesContext(vm *v1.VirtualMachine) error {
//	root := vm.Spec.Request.Config.TLS["root"]
//	if root == nil {
//		klog.Warningf("unexpected root tls config: vm.Spec.Request.Config.TLS")
//		return nil
//	}
//	addr := getAddress(vm.Status.Address)
//	if addr == "" {
//		klog.Warningf("unexpected empty address: vm.Status.Address[%s]", vm.Status.Address)
//		return nil
//	}
//	key, crt, err := sign.SignKubernetesClient(root.Cert, root.Key, []string{})
//	if err != nil {
//		return fmt.Errorf("sign kubernetes client crt for %s: %s", vm.Name, err.Error())
//	}
//
//	data, err := tool.RenderConfig(
//		fmt.Sprintf("%s@%s", v1.MeridianUserName(vm.Name), v1.MeridianClusterName(vm.Name)),
//		tool.KubeConfigTpl,
//		tool.RenderParam{
//			AuthCA:      base64.StdEncoding.EncodeToString(root.Cert),
//			Address:     addr,
//			Port:        "6443",
//			ClusterName: v1.MeridianClusterName(vm.Name),
//			UserName:    v1.MeridianUserName(vm.Name),
//			ClientCRT:   base64.StdEncoding.EncodeToString(crt),
//			ClientKey:   base64.StdEncoding.EncodeToString(key),
//		},
//	)
//	if err != nil {
//		return fmt.Errorf("render kube config error: %s", err.Error())
//	}
//	gencfg, err := clientcmd.Load([]byte(data))
//	if err != nil {
//		return fmt.Errorf("load kube config error: %s", err.Error())
//	}
//	home, err := os.UserHomeDir()
//	if err != nil {
//		klog.Warningf("ensure kubeconfig context: %s", err.Error())
//		return nil
//	}
//	kcfg := filepath.Join(home, ".kube", "config")
//	_, err = os.Stat(kcfg)
//	if err != nil {
//		if os.IsNotExist(err) {
//			err = os.MkdirAll(filepath.Join(home, ".kube"), 0755)
//			if err != nil {
//				return err
//			}
//			return clientcmd.WriteToFile(*gencfg, kcfg)
//		}
//		return err
//	}
//	cfg, err := clientcmd.LoadFromFile(kcfg)
//	if err != nil {
//		klog.Warningf("ensure kubeconfig context: %s", err.Error())
//		return nil
//	}
//	var (
//		userName    = v1.MeridianUserName(vm.Name)
//		clusterName = v1.MeridianClusterName(vm.Name)
//	)
//	if cfg.Clusters == nil {
//		cfg.Clusters = make(map[string]*clientcmdapi.Cluster)
//	}
//	if cfg.AuthInfos == nil {
//		cfg.AuthInfos = make(map[string]*clientcmdapi.AuthInfo)
//	}
//	if cfg.Contexts == nil {
//		cfg.Contexts = make(map[string]*clientcmdapi.Context)
//	}
//	cfg.Clusters[clusterName] = gencfg.Clusters[clusterName]
//	cfg.AuthInfos[userName] = gencfg.AuthInfos[userName]
//	ctx := fmt.Sprintf("%s@%s", userName, clusterName)
//	cfg.Contexts[ctx] = gencfg.Contexts[ctx]
//	return clientcmd.WriteToFile(*cfg, kcfg)
//}
//
//func (m *virtualMachine) CleanUpKubernetesContext(vm *v1.VirtualMachine) error {
//
//	home, err := os.UserHomeDir()
//	if err != nil {
//		klog.Warningf("ensure kubeconfig context: %s", err.Error())
//		return nil
//	}
//	cfg, err := clientcmd.LoadFromFile(filepath.Join(home, ".kube", "config"))
//	if err != nil {
//		klog.Warningf("ensure kubeconfig context: %s", err.Error())
//		return nil
//	}
//	var (
//		write       = false
//		userName    = v1.MeridianUserName(vm.Name)
//		clusterName = v1.MeridianClusterName(vm.Name)
//	)
//	_, ok := cfg.Clusters[clusterName]
//	if ok {
//		write = true
//		delete(cfg.Clusters, clusterName)
//	}
//	_, ok = cfg.AuthInfos[userName]
//	if ok {
//		write = true
//		delete(cfg.AuthInfos, userName)
//	}
//	ctx := fmt.Sprintf("%s@%s", userName, clusterName)
//	_, ok = cfg.Contexts[ctx]
//	if ok {
//		write = true
//		delete(cfg.Contexts, ctx)
//	}
//	if !write {
//		return nil
//	}
//	return clientcmd.WriteToFile(*cfg, filepath.Join(home, ".kube", "config"))
//}
//

//
//func (m *virtualMachine) EnsureKubernetes(ctx context.Context, client client.Interface, vm *v1.VirtualMachine) error {
//	gi := v1.EmptyGI(vm.Name)
//	err := client.Get(ctx, gi)
//	if err != nil {
//		return err
//	}
//	klog.Infof("[%-10s] guest vm status: %+v", vm.Name, gi.Status.Conditions)
//	cond := meta.FindStatusCondition(gi.Status.Conditions, "Kubernetes")
//	if cond == nil ||
//		cond.Status == metav1.ConditionUnknown || cond.Reason == "NotInstalled" {
//		klog.Infof("[%-10s] unknown Kubernetes condition, create k8s", vm.Name)
//		req := v1.NewEmptyRequest(vm.Name, vm.Spec.Request)
//		err = client.Create(ctx, req)
//		if err != nil {
//			return errors.Wrapf(err, "create k8s request")
//		}
//		klog.Infof("[%-10s]k8s request created, wait response", vm.Name)
//	}
//	// todo repair node
//	pollFunc := func(ctx context.Context) (bool, error) {
//		km := v1.EmptyVM(vm.Name)
//		_, err := m.Get(ctx, km, nil)
//		if err != nil {
//			if IsNotFound(err) {
//				klog.Errorf("[%-10s] vm not found: [%s]", vm.Name, km.Name)
//				return true, err
//			}
//			klog.Infof("find vm [%s] error: %v", vm.Name, err.Error())
//			return false, nil
//		}
//		err = client.Get(ctx, gi)
//		if err != nil {
//			klog.Infof("[%-10s] ensure kubernetes, wait host agent start: %v", vm.Name, err)
//			return false, nil
//		}
//		if len(vm.Status.Address) == 0 {
//			klog.Infof("[%-10s]ensure kubernetes wait guest address: %s", vm.Name, vm.Status.Phase)
//			return false, nil
//		}
//		klog.Infof("[%-10s] ensure kubernetes, wait guest responed with: %+v", vm.Name, gi.Status)
//		cond = meta.FindStatusCondition(gi.Status.Conditions, "Kubernetes")
//		if cond == nil {
//			klog.Infof("[%-10s]kubernetes condition not found yet", vm.Name)
//			return false, nil
//		}
//		if cond.Status == metav1.ConditionTrue &&
//			gi.Status.Phase == v1.Running {
//			return true, nil
//		}
//		klog.Infof("[%-10s]kubernetes condition found, Status=%s, %s, %s", vm.Name, cond.Status, gi.Status.Phase, cond.Reason)
//		return cond.Status == metav1.ConditionTrue && gi.Status.Phase == v1.Running, nil
//	}
//	err = wait.PollUntilContextTimeout(ctx, 3*time.Second, 8*time.Minute, false, pollFunc)
//	if err != nil {
//		return err
//	}
//	return m.EnsureKubernetesContext(vm)
//}
