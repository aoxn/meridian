//go:build windows
// +build windows

package tool

import "fmt"

var AUTH_FILE = "/etc/kubernetes/admin.local"

// ApplyYaml run
// kubectl --kubeconfig /etc/kubernetes/admin.local apply -f /tmp/cfg.yaml
// NOT concurrentable
func ApplyYaml(data string) error {
	return ApplyYamlCommon(data, AUTH_FILE, "/tmp/cfg.yaml")
}

func ApplyYamlCommon(data, authf, tmp string) error {

	return nil
}

func Kubectl(args ...string) ([]string, error) {
	return []string{}, fmt.Errorf("not implemented")
}
