package xdpin

import (
	"fmt"
	"github.com/aoxn/meridian/internal/tool/mapping"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"os"
	"path"
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
	UpnpPortMapping   []mapping.Item `json:"portMapping"`
	XdpDomain         Domain         `json:"domainForXdpin,omitempty"`
	LbACL             AclGroup       `json:"SlbACL,omitempty"`
	SSHSecrurityGroup SecurityGroup  `json:"SSHSecurityGroup"`
}

type Domain struct {
	Auth       `json:"auth"`
	DomainName string `json:"domainName"`
	DomainRR   string `json:"domainRR"`
	Provider   string `json:"provider"`
}

type SecurityGroup struct {
	Auth            `json:"auth"`
	RuleIdentity    string `json:"ruleIdentity"`
	Provider        string `json:"provider"`
	SecurityGroupID string `json:"securityGroupID"`
}

type Auth struct {
	RefName         string `json:"refName"`
	AccessKeyID     string `json:"accessKeyID"`
	AccessKeySecret string `json:"accessKeySecret"`
	Region          string `json:"region"`
}

type AclGroup struct {
	Auth     `json:"auth"`
	AclID    string `json:"aclID"`
	Provider string `json:"provider"`
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
