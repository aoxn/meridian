package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/aoxn/meridian"
	"github.com/aoxn/meridian/cmd/meridiand/vm"
	"github.com/aoxn/meridian/internal/apihandler"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Flags struct {
	LogLevel string
}

// NewCommand returns a new cobra.Command implementing the root command for meridian
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "meridian",
		Short: "meridian creates and manages infrastructure agnostic Kubernetes clusters",
		Long: fmt.Sprintf("%s\n%s", meridian.Logo, "meridian creates and manages infrastructure agnostic "+
			"Kubernetes clusters and empower strong infrastructure resilience ability and easy recovery"),
		SilenceUsage: true,
	}
	cmd.PersistentFlags().AddGoFlagSet(NewKlogFlags())
	cmd.AddCommand(NewCommandServe())
	cmd.AddCommand(vm.NewCommandVM())
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

// NewCommandServe returns a new cobra.Command implementing the root command for meridian
func NewCommandServe() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "meridian serve boot an apiserver",
		Long: fmt.Sprintf("%s\n%s", meridian.Logo, "meridian creates and manages infrastructure agnostic "+
			"Kubernetes clusters and empower strong infrastructure resilience ability and easy recovery"),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := apihandler.RunDaemonAPI(context.TODO())
			return err
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
