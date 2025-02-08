package vm

import (
	"context"
	"fmt"
	"github.com/aoxn/meridian"
	v1 "github.com/aoxn/meridian/api/v1"
	hostagent "github.com/aoxn/meridian/internal/vma/host"
	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
	"io"
	"k8s.io/klog/v2"
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

// NewCommandStart returns a new cobra.Command for cluster creation
func NewCommandStart() *cobra.Command {
	cfgfile := ""
	cmd := &cobra.Command{
		Use:   "start",
		Short: "meridian start vm, running guest vm",
		Long:  "meridian start vm, running guest vm",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf(meridian.Logo)
			return Start(args, cfgfile)
		},
	}
	cmd.PersistentFlags().StringVarP(&cfgfile, "config", "c", "", "vm config file")
	return cmd
}

func Start(args []string, cfgfile string) error {
	if len(args) <= 0 {
		return fmt.Errorf("resource type is required")
	}
	resource := args[0]
	switch resource {
	case "vm":
	default:
		return fmt.Errorf("unsupported resource type: %s", resource)
	}
	var (
		data []byte
		err  error
		vm   = &v1.VirtualMachine{}
	)
	klog.Infof("start vm with cfg: [%s]", cfgfile)
	if cfgfile != "" {
		data, err = os.ReadFile(cfgfile)
	} else {
		data, err = io.ReadAll(os.Stdin)
	}
	if err != nil {
		return err
	}
	klog.Infof("start vm with data: [%s]", string(data))
	err = yaml.Unmarshal(data, vm)
	if err != nil {
		return err
	}
	if vm.Name == "" {
		return fmt.Errorf("vm name is required")
	}
	ctx, cancel := context.WithCancel(context.Background())
	signalFunc := func() {
		sigchan := make(chan os.Signal, 10)
		signal.Notify(sigchan, os.Interrupt, os.Kill, syscall.SIGTERM)
		for {
			klog.Infof("waiting for signal")
			select {
			case sig := <-sigchan:
				cancel()
				klog.Infof("received signal: %s", sig.String())
				return
			}
		}
	}
	go signalFunc()
	if vm.Spec.GUI {
		// Without this the call to vz.RunGUI fails. Adding it here, as this has to be called before the vz cgo loads.
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
	}
	host, err := hostagent.New(vm, os.Stdout)
	if err != nil {
		return err
	}
	return host.Run(ctx)
}
