package svc

import (
	"context"
	"fmt"
	"k8s.io/klog/v2"
	"net"
	"os"
	"path"
	"strings"
	"time"

	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/server/service"
	"github.com/aoxn/meridian/internal/server/service/backend/local"
	"github.com/aoxn/meridian/internal/server/service/generic"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/clientcmd"
)

func NewGuestInfoPvd(opt *service.Options) service.Provider {
	return &guestInfoPvd{options: opt}
}

type guestInfoPvd struct {
	options *service.Options
}

func (v *guestInfoPvd) NewAPIGroup(ctx context.Context) (service.Grouped, error) {
	grp := service.Grouped{}
	v.addV1(grp, v.options)
	return grp, nil
}

func (v *guestInfoPvd) addV1(grp service.Grouped, options *service.Options) {
	univ := NewGuestInfo(options)
	grp.AddOrDie(univ)
}

func NewGuestInfo(options *service.Options) service.Store {
	var store service.Store
	switch options.Provider.Type {
	case "Local":
		store = &local.Local{
			Standard: &generic.Store{
				Scheme: options.Scheme,
			},
		}
	default:
		panic(fmt.Sprintf("unimplemented provider type: [%s]", options.Provider.Type))
	}
	univ := &guestInfo{
		Store:           store,
		scheme:          options.Scheme,
		allowedResource: sets.New[string](),
	}
	return univ
}

type guestInfo struct {
	service.Store
	scheme          *runtime.Scheme
	allowedResource sets.Set[string]
}

func (u *guestInfo) GVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    v1.GroupVersion.Group,
		Version:  v1.GroupVersion.Version,
		Resource: "guestinfos",
	}
}

func (u *guestInfo) Get(ctx context.Context, object runtime.Object, options *metav1.GetOptions) (runtime.Object, error) {
	addrs, err := GetLocalIP()
	if err != nil {
		return nil, err
	}
	gi, ok := object.(*v1.GuestInfo)
	if !ok {
		return nil, fmt.Errorf("object is not a v1.GuestInfo")
	}
	gi.Spec.Address = addrs
	req := v1.NewEmptyRequest(gi.Name, v1.RequestSpec{})
	_, err = u.Store.Get(ctx, req, &metav1.GetOptions{})
	if err != nil {
		klog.Infof("requested vm %s: %s", gi.Name, err.Error())
	}
	klog.Infof("get guest-vm address: %s", addrs)
	gi.Status = v1.GuestInfoStatus{
		Phase:      v1.Running,
		Conditions: buildCondition(req),
	}
	return gi, nil
}

const (
	KubernetesType = "Kubernetes"
)

func buildCondition(req *v1.Request) []metav1.Condition {
	var condition []metav1.Condition
	kcfg, err := configFile()
	if err != nil {
		// True  已安装，且正常
		// False 已安装，且异常
		// Unknown 未安装
		if os.IsNotExist(err) {
			return []metav1.Condition{{
				Type:   KubernetesType,
				Reason: "NotInstalled",
				Status: metav1.ConditionUnknown,
			}}
		}
		return []metav1.Condition{{
			Type:   KubernetesType,
			Reason: err.Error(),
			Status: metav1.ConditionUnknown,
		}}
	}
	client, err := newClient(kcfg)
	if err != nil {
		return []metav1.Condition{{
			Type:   KubernetesType,
			Reason: err.Error(),
			Status: metav1.ConditionFalse,
		}}
	}
	_, err = Healthy(client)
	if err != nil {
		return []metav1.Condition{{
			Type:   KubernetesType,
			Reason: err.Error(),
			Status: metav1.ConditionFalse,
		}}
	}
	for _, v := range req.Spec.Config.Addons {
		condition = append(condition, metav1.Condition{
			Type:   v.Name,
			Reason: v.Name,
			Status: metav1.ConditionTrue,
		})
	}

	condition = append(condition, metav1.Condition{
		Type:   KubernetesType,
		Status: metav1.ConditionTrue,
	})
	return condition
}

func configFile() (string, error) {
	home := os.Getenv("HOME")
	if home == "" {
		return "", fmt.Errorf("empty HOME env")
	}
	kcfg := path.Join(home, ".kube", "config")
	_, err := os.Stat(kcfg)
	return kcfg, err
}

func newClient(kcfg string) (clientset.Interface, error) {
	cfg, err := clientcmd.BuildConfigFromFlags("", kcfg)
	if err != nil {
		return nil, err
	}
	return clientset.NewForConfig(cfg)
}

func Healthy(client clientset.Interface) (bool, error) {
	version := ""
	healthy := func(ctx context.Context) (done bool, err error) {
		info, err := client.Discovery().ServerVersion()
		if err != nil {
			klog.Errorf("wait for apiserver: %s", err.Error())
			return false, err
		}
		version = info.String()
		klog.Infof("kube-apiserver version: %+v", version)
		return true, nil
	}
	err := wait.PollUntilContextTimeout(context.TODO(), 3*time.Second, 10*time.Minute, true, healthy)
	return err == nil, err
}

func LocalAddr() ([]string, error) {
	var address []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return address, err
	}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			return address, err
		}
		// handle err
		for _, addr := range addrs {
			address = append(address, addr.String())
		}
	}
	return address, nil
}

func GetLocalIP() ([]string, error) {
	var addresses []string
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return addresses, err
	}
	for _, address := range addrs {
		ipnet, ok := address.(*net.IPNet)
		if ok && !ipnet.IP.IsLoopback() {
			addr := ipnet.IP.String()
			if addr != "" && !strings.HasPrefix(addr, "169.254") {
				addresses = append(addresses, ipnet.IP.String())
			}
		}
	}
	return addresses, nil
}
