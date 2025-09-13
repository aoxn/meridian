package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/aoxn/meridian"
	v1 "github.com/aoxn/meridian/api/v1"
	hostagent "github.com/aoxn/meridian/internal/vmm/host"
	"github.com/aoxn/meridian/internal/vmm/meta"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	godaemon "github.com/sevlyar/go-daemon"
	"github.com/spf13/cobra"

	"io"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"syscall"
)

func init() {
	runtime.LockOSThread()
	klog.Infof("run gui with lock thread: init")
}

func Start(args []string, cfgfile string) error {

	var (
		data []byte
		err  error
		vm   = &meta.Machine{}
	)
	klog.Infof("start vm with cfg: [%s]", cfgfile)
	if cfgfile != "" {
		data, err = os.ReadFile(cfgfile)
	} else {
		data, err = io.ReadAll(os.Stdin)
	}
	if err != nil {
		return errors.Wrap(err, "read vm config")
	}
	err = yaml.Unmarshal(data, vm)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal vm")
	}
	cnctx := &godaemon.Context{
		PidFileName: vm.PIDFile(),
		PidFilePerm: 0644,
		LogFileName: filepath.Join(vm.Dir(), v1.HostAgentStdoutLog),
		LogFilePerm: 0644,
		Umask:       027,
		WorkDir:     vm.Dir(),
	}
	d, err := cnctx.Reborn()
	if err != nil {
		return errors.Wrapf(err, "reborn daemon")
	}
	if d != nil {
		klog.Infof("daemon is running: %d", d.Pid)
		return err
	}

	defer cnctx.Release()

	klog.Infof("[%s]daemon started: with pid [%d]", vm.Name, os.Getpid())

	klog.Infof("[%s]start vm with data: [%s]", vm.Name, string(data))
	if vm.Name == "" {
		return fmt.Errorf("vm name is required")
	}

	sigchan := make(chan os.Signal, 10)
	signal.Notify(sigchan, os.Interrupt, os.Kill, syscall.SIGTERM)
	host, err := hostagent.New(vm, sigchan)
	if err != nil {
		return errors.Wrap(err, "new host agent")
	}
	return host.Run(context.TODO())
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
