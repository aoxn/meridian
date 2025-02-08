//go:build linux || darwin
// +build linux darwin

package cmd

import (
	"fmt"
	gcmd "github.com/go-cmd/cmd"
	"k8s.io/klog/v2"
	"strings"
)

func NewCmd(name string, args ...string) *gcmd.Cmd {
	klog.Infof("debug run command: %s %s", name, strings.Join(args, " "))
	return gcmd.NewCmd(name, args...)
}

func CmdError(sta gcmd.Status) error {
	if len(sta.Stderr) != 0 {
		klog.Infof("stand error NotEmpty[%d]: %s", sta.Exit, sta.Stderr)
	}
	if sta.Exit == 0 && sta.Error == nil {
		return nil
	}
	return fmt.Errorf("exit=%d, error: %v", sta.Exit, sta.Error)
}

func CmdResult(sta gcmd.Status) (string, error) {
	var result []string
	if len(sta.Stderr) != 0 {
		result = append(sta.Stdout, "====================================================")
		result = append(result, sta.Stderr...)
		result = append(result, fmt.Sprintf("ExitCode=[%d]", sta.Exit))
	}
	if sta.Exit == 0 && sta.Error == nil {
		return strings.Join(result, "\n"), nil
	}
	return strings.Join(result, "\n"), fmt.Errorf("command exit with: code=[%d], %s", sta.Exit, sta.Error)
}

func Systemctl(ops []string) error {
	cm := NewCmd(
		"systemctl", ops...,
	)
	result := <-cm.Start()
	return result.Error
}
