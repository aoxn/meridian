package xdpin

import (
	"context"
	"fmt"
	api "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/tool/mapping"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"os"
	"path"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sync"
)

var (
	defaultPath = "/xdpin"
	statePath   = path.Join(defaultPath, "data")
)

func configLocation() string {
	return path.Join(defaultPath, "config")
}

func stateLocation() string { return path.Join(statePath, "state") }

type Config struct {
	UpnpPortMapping   []mapping.Item `json:"mapping,omitempty"`
	XdpDomain         Domain         `json:"dns,omitempty"`
	LbACL             AclGroup       `json:"acl,omitempty"`
	SSHSecrurityGroup SecurityGroup  `json:"securityGroup,omitempty"`
}

type Domain struct {
	Region     string `json:"region,omitempty"`
	DomainName string `json:"domainName,omitempty"`
	DomainRR   string `json:"domainRR,omitempty"`
	Provider   string `json:"authProvider,omitempty"`
}

type SecurityGroup struct {
	Region          string `json:"region,omitempty"`
	RuleIdentity    string `json:"ruleIdentity,omitempty"`
	Provider        string `json:"authProvider,omitempty"`
	SecurityGroupID string `json:"securityGroupID,omitempty"`
}

type Auth struct {
	RefName         string `json:"refName,omitempty"`
	AccessKeyID     string `json:"accessKeyID,omitempty"`
	AccessKeySecret string `json:"accessKeySecret,omitempty"`
	Region          string `json:"region,omitempty"`
}

type AclGroup struct {
	Region   string `json:"region,omitempty"`
	AclID    string `json:"aclID,omitempty"`
	Provider string `json:"authProvider,omitempty"`
}

func GetAuth(cli client.Client, provider string) (api.Provider, error) {
	auth := api.Provider{}
	err := cli.Get(context.TODO(),
		client.ObjectKey{Name: provider}, &auth)
	return auth, err
}

func Load(cli client.Client, from client.ObjectKey) (Config, error) {
	var cm = v1.ConfigMap{}
	err := cli.Get(context.Background(), from, &cm)
	if err != nil {
		return Config{}, errors.Wrapf(err, "read configmap[%s] from k8s", from.String())
	}
	data := cm.Data["config"]
	if data == "" {
		return Config{}, errors.New("no [config] data found")
	}
	var cfg = Config{}
	err = yaml.Unmarshal([]byte(data), &cfg)
	return cfg, err
}

func LoadCfg() (Config, error) {
	cfg := configLocation()
	klog.Infof("load config from: %s", cfg)
	_, err := os.Stat(cfg)
	if err != nil {
		if os.IsNotExist(err) {
			_, err := os.Stat(stateLocation())
			if err != nil {
				return Config{}, errors.Wrapf(err, "fallback to state file")
			}
			err = LoadState()
			if err != nil {
				return Config{}, err
			}
			klog.Infof("restore config from state")
			return stateful.Config, nil
		}
		return stateful.Config, err
	}
	data, err := os.ReadFile(cfg)
	if err != nil {
		return Config{}, err
	}
	config := Config{}
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return Config{}, err
	}
	stateful.Config = config
	klog.Infof("replace state config with content from [%s]", cfg)
	return stateful.Config, nil
}

var (
	mu       = &sync.RWMutex{}
	stateful = &Stateful{Data: make(map[string]*v1.ConfigMap)}
)

func key(cm *v1.ConfigMap) string {
	return fmt.Sprintf("%s/%s", cm.Namespace, cm.Name)
}

func Set(cm *v1.ConfigMap) {
	mu.Lock()
	defer mu.Unlock()
	stateful.Data[key(cm)] = cm
	err := SaveState()
	if err != nil {
		klog.Infof("save state failed: %v", err)
		return
	}
	klog.Infof("save state for [%s]", key(cm))
}

func Remove(key string) {
	mu.Lock()
	defer mu.Unlock()
	_, ok := stateful.Data[key]
	if !ok {
		return
	}
	delete(stateful.Data, key)
	err := SaveState()
	if err != nil {
		klog.Infof("save state failed: %v", err)
		return
	}
	klog.Infof("remove state for [%s]", key)
}

type Stateful struct {
	Config Config                   `json:"config"`
	Data   map[string]*v1.ConfigMap `json:"data"`
}

func LoadState() error {
	_, err := os.Stat(stateLocation())
	if err != nil {
		if os.IsNotExist(err) {
			_, err := os.Stat(configLocation())
			if err != nil {
				return errors.Wrapf(err, "fallback to config file")
			}
			_, err = LoadCfg()
			if err != nil {
				klog.Infof("load config failed: %v", err)
				return err
			}
			err = SaveState()
			if err != nil {
				klog.Infof("save state failed: %v", err)
				return err
			}
		}
		return err
	}
	data, err := os.ReadFile(stateLocation())
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, &stateful)
}

func SaveState() error {
	err := os.MkdirAll(statePath, os.ModePerm)
	if err != nil {
		return errors.Wrapf(err, "ensure data dir")
	}
	data, err := yaml.Marshal(stateful)
	if err != nil {
		return err
	}
	return os.WriteFile(stateLocation(), data, 0755)
}

const cfgExample = `
apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    xdpin.cn/mark: ""
  name: xdpin.cfg
  namespace: kube-system
data:
  config: |
    dns:
      state: enable
      authProvider: kubernetes.aoxn
      domainRR: "@,www"
      domainName: "xdpin.cn"
    mapping:
    - state: enable
      externalPort: 22
      internalPort: 22
      protocol: TCP
      description: "ssh"
    - state: enable
      externalPort: 6443
      internalPort: 6443
      protocol: TCP
      description: "kubernetes"
    - state: enable
      externalPort: 8096
      internalPort: 8096
      protocol: TCP
      description: "jellyfin"
`
