package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/aoxn/meridian"
	v1 "github.com/aoxn/meridian/api/v1"
	hostagent "github.com/aoxn/meridian/internal/vma/host"
	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
	"io"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	ctrl "sigs.k8s.io/controller-runtime"
)

func Start(args []string, cfgfile string) error {
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

type Flags struct {
	LogLevel string
}

// NewCommandStart returns a new cobra.Command for cluster creation
func NewCommandStart() *cobra.Command {
	cfgfile := ""
	cmd := &cobra.Command{
		Use:   "start",
		Short: "meridian-vm start , running guest vm",
		Long:  "meridian-vm start , running guest vm",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf(meridian.Logo)
			return Start(args, cfgfile)
		},
	}
	cmd.PersistentFlags().StringVarP(&cfgfile, "config", "c", "", "vm config file")
	return cmd
}

// NewCommand returns a new cobra.Command implementing the root command for meridian
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "meridian-vm",
		Short: "meridian-vm creates vm",
		Long:  fmt.Sprintf("%s\n%s", meridian.Logo, "meridian-vm creates vm"),
	}
	cmd.PersistentFlags().AddGoFlagSet(NewKlogFlags())
	cmd.AddCommand(NewCommandStart())
	cmd.AddCommand(NewCommandVersion())
	return cmd
}
func NewCommandVersion() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "version",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf(meridian.Logo)
			fmt.Printf(meridian.Version)
			return nil
		},
	}
	return cmd
}

func NewKlogFlags() *flag.FlagSet {
	cmdline := flag.NewFlagSet("", flag.ExitOnError)
	klog.InitFlags(cmdline)
	return cmdline
}

// Run runs the `meridian` root command
func Run() error {
	return NewCommand().Execute()
}

// main wraps Run and sets the log formatter
func main() {
	ctrl.SetLogger(klogr.New())
	if err := Run(); err != nil {
		klog.Errorf("run error: %s", err.Error())
		os.Exit(1)
	}
}
