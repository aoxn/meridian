package kubeclient

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	api "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/tool"
	"github.com/aoxn/meridian/internal/tool/sign"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog/v2"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Guest(vm *api.VirtualMachine) *KubeClient {
	return &KubeClient{vm: vm, side: "guest"}
}

func Host(req *api.Request) *KubeClient {
	return &KubeClient{request: req, side: "host"}
}

type KubeClient struct {
	side    string
	request *api.Request
	auth    *clientcmdapi.Config
	client  kubernetes.Interface
	vm      *api.VirtualMachine
}

func (kc *KubeClient) initClient() error {
	if kc.client != nil {
		return nil
	}
	var (
		address = "127.0.0.1"
		request = kc.request
	)
	switch kc.side {
	case "guest":
		if len(kc.vm.Status.Address) <= 0 {
			return fmt.Errorf("unknown api server address for vm: %s", kc.vm.Name)
		}
		for _, v := range kc.vm.Status.Address {
			if strings.HasPrefix(v, "192.168") {
				address = v
				break
			}
		}
	case "host":
		klog.Infof("build local kubernetes client config")
	default:
		return fmt.Errorf("unimplenmented host kubeclient: %s", kc.vm.Name)
	}
	root := request.Spec.Config.TLS["root"]
	key, crt, err := sign.SignKubernetesClient(root.Cert, root.Key, []string{})
	if err != nil {
		return fmt.Errorf("sign kubernetes client crt: %s", err.Error())
	}
	cfgData, err := tool.RenderConfig(
		"admin.authconfig",
		tool.KubeConfigTpl,
		tool.RenderParam{
			AuthCA:      base64.StdEncoding.EncodeToString(root.Cert),
			Address:     address,
			Port:        "6443",
			ClusterName: "kubernetes.cluster",
			UserName:    "kubernetes.user",
			ClientCRT:   base64.StdEncoding.EncodeToString(crt),
			ClientKey:   base64.StdEncoding.EncodeToString(key),
		},
	)
	if err != nil {
		return fmt.Errorf("render vm kubeconfig error: %s", err.Error())
	}
	klog.V(5).Infof("debug: with kubeconfig: %s", cfgData)
	config, err := clientcmd.Load([]byte(cfgData))
	if err != nil {
		return fmt.Errorf("load kubeconfig error: %s", err.Error())
	}
	kc.auth = config
	client, err := ToClientSet(config)
	if err != nil {
		return err
	}
	kc.client = client
	return nil
}

func (kc *KubeClient) Apply(yml string) error {
	if err := kc.initClient(); err != nil {
		return err
	}
	return doApply(bytes.NewBufferString(yml), NewClientGetter(kc.auth))
}

func (kc *KubeClient) Client() (kubernetes.Interface, error) {
	if err := kc.initClient(); err != nil {
		return nil, err
	}
	return kc.client, nil
}

// ClientSetFromFile returns a ready-to-use client from a KubeConfig file
func ClientSetFromFile(path string) (*kubernetes.Clientset, error) {
	config, err := clientcmd.LoadFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load admin kubeconfig [%v]", err)
	}
	return ToClientSet(config)
}

// ToClientSet converts a KubeConfig object to a client
func ToClientSet(config *clientcmdapi.Config) (*kubernetes.Clientset, error) {
	clientConfig, err := clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create API client configuration from kubeconfig: %v", err)
	}

	client, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %v", err)
	}
	return client, nil
}

// CreateOrUpdateSecret creates a Secret if the target resource doesn't exist. If the resource exists already, this function will update the resource instead.
func (kc *KubeClient) CreateOrUpdateSecret(secret *v1.Secret) error {
	if err := kc.initClient(); err != nil {
		return err
	}
	if _, err := kc.client.CoreV1().Secrets(secret.ObjectMeta.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("unable to create secret: %v", err)
		}

		if _, err := kc.client.CoreV1().Secrets(secret.ObjectMeta.Namespace).Update(context.TODO(), secret, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("unable to update secret: %v", err)
		}
	}
	return nil
}

func (kc *KubeClient) CreateOrUpdateConfigMap(cm *v1.ConfigMap) error {
	if err := kc.initClient(); err != nil {
		return err
	}
	if _, err := kc.client.CoreV1().ConfigMaps(cm.Namespace).Create(context.TODO(), cm, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("unable to create configmap: %v", err)
		}

		if _, err := kc.client.CoreV1().ConfigMaps(cm.ObjectMeta.Namespace).Update(context.TODO(), cm, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("unable to update configmap: %v", err)
		}
	}
	return nil
}

func (kc *KubeClient) EnsureNamespace(namespace string) error {
	if err := kc.initClient(); err != nil {
		return err
	}
	// should with retry.
	if _, err := kc.client.CoreV1().
		Namespaces().
		Create(
			context.TODO(),
			&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}},
			metav1.CreateOptions{},
		); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func LoadClientFromConfig(config *clientcmdapi.Config) (kubernetes.Interface, error) {
	rest, err := clientcmd.BuildConfigFromKubeconfigGetter(
		"",
		func() (*clientcmdapi.Config, error) {
			return config, nil
		},
	)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(rest)
}

func (kc *KubeClient) FindSecret(namespace, name string) (*v1.Secret, bool, error) {
	if err := kc.initClient(); err != nil {
		return nil, false, err
	}
	secret, err := kc.client.
		CoreV1().
		Secrets(namespace).
		Get(context.TODO(), name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return secret, true, nil
}

func (kc *KubeClient) FindMasters() ([]v1.Node, bool, error) {
	if err := kc.initClient(); err != nil {
		return nil, false, err
	}
	nodes, err := kc.client.
		CoreV1().Nodes().List(
		context.TODO(),
		metav1.ListOptions{
			LabelSelector: "host-role.kubernetes.io/master",
		},
	)
	if apierrors.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return nodes.Items, true, nil
}

func (kc *KubeClient) FindConfigMap(namespace, name string) (*v1.ConfigMap, bool, error) {
	if err := kc.initClient(); err != nil {
		return nil, false, err
	}
	cm, err := kc.client.
		CoreV1().
		ConfigMaps(namespace).
		Get(context.TODO(), name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return cm, true, nil
}
