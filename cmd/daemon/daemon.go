package main

import (
	"context"
	"fmt"
	"github.com/aoxn/meridian"
	"github.com/aoxn/meridian/cmd/meridian/guest"
	"github.com/aoxn/meridian/cmd/meridian/vm"
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
	flags := &Flags{}
	cmd := &cobra.Command{
		Use:   "meridian",
		Short: "meridian creates and manages infrastructure agnostic Kubernetes clusters",
		Long: fmt.Sprintf("%s\n%s", meridian.Logo, "meridian creates and manages infrastructure agnostic "+
			"Kubernetes clusters and empower strong infrastructure resilience ability and easy recovery"),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return runE(flags, cmd, args)
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			//err := v1.LoadConfig()
			//if err != nil {
			//	return errors.Wrap(err, "need provider config")
			//}
			err := apihandler.RunDaemonAPI(context.TODO())
			//return os.Remove("/tmp/meridian.sock")
			return err
		},
	}
	cmd.AddCommand(guest.NewCommandGuest())
	cmd.AddCommand(vm.NewCommandStart())
	// add all top level subcommands
	//cmd.AddCommand(bootstrap.NewCommand())
	return cmd
}

func runE(flags *Flags, cmd *cobra.Command, args []string) error {

	return nil
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
