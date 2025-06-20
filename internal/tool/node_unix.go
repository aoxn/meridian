//go:build linux || darwin
// +build linux darwin

package tool

import (
	"fmt"
	"github.com/aoxn/meridian/internal/tool/cmd"
	"io/ioutil"
)

var AUTH_FILE = "/etc/kubernetes/admin.conf"

// ApplyYaml run
// kubectl --kubeconfig /etc/kubernetes/admin.local apply -f /tmp/cfg.yaml
// NOT concurrentable
func ApplyYaml(data string, name string) error {
	return ApplyYamlCommon(data, AUTH_FILE, fmt.Sprintf("/tmp/cfg.%s.yaml", name))
}

func ApplyYamlCommon(data, authf, tmp string) error {

	err := ioutil.WriteFile(tmp, []byte(data), 0755)
	if err != nil {
		return fmt.Errorf("create tmp file: %s", err.Error())
	}
	stauts := <-cmd.NewCmd(
		"/usr/local/bin/kubectl",
		"--kubeconfig",
		authf,
		"apply", "-f", tmp,
	).Start()
	return cmd.CmdError(stauts)
}

func Kubectl(args ...string) ([]string, error) {
	stauts := <-cmd.NewCmd(
		"/usr/local/bin/kubectl", args...,
	).Start()
	return stauts.Stdout, cmd.CmdError(stauts)
}
