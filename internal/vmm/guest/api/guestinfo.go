package api

import (
	"context"
	"fmt"
	"github.com/aoxn/meridian/internal/tool/server"
	"k8s.io/klog/v2"
	"net"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	v1 "github.com/aoxn/meridian/api/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/clientcmd"
)

func GetGI(r *http.Request, w http.ResponseWriter) int {
	addrs, err := GetLocalIP()
	if err != nil {
		return server.HttpJson(w, err)
	}
	gi := v1.EmptyGI("guest")
	gi.Spec.Address = addrs
	req := v1.NewEmptyRequest(gi.Name, v1.RequestSpec{})
	gi.Status = v1.GuestInfoStatus{
		Phase:      v1.Running,
		Conditions: buildCondition(req),
	}
	return server.HttpJson(w, gi)
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
