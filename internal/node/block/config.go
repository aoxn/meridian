package block

import (
	"fmt"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"
	"os"
	"path"
)

func HomeKubeCfg() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return home, err
	}
	home = path.Join(home, ".kube")
	klog.Infof("kube config home dir: %s", home)
	err = os.MkdirAll(home, 0755)
	if err != nil {
		return "", errors.Wrap(err, "mkdir of home kube")
	}
	return fmt.Sprintf("%s/config", home), nil
}
