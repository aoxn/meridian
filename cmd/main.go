package main

import (
	"flag"
	"fmt"
	"github.com/aoxn/meridian/cmd/meridian/guest"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/aoxn/meridian"
	v1 "github.com/aoxn/meridian/api/v1"
	m2 "github.com/aoxn/meridian/cmd/meridian"
)

// NewCommand returns a new cobra.Command implementing the root command for meridian
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "meridian",
		Short: "meridian creates and manages infrastructure agnostic Kubernetes clusters",
		Long: fmt.Sprintf("%s\n%s", meridian.Logo, "meridian creates and manages infrastructure agnostic "+
			"Kubernetes clusters and empower strong infrastructure resilience ability and easy recovery"),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return runE(cmd, args)
		},
		SilenceUsage: true,
	}
	cmd.PersistentFlags().AddGoFlagSet(NewKlogFlags())
	globalFlags(cmd)
	cmd.AddCommand(m2.NewCommandServe())
	// add all top level subcommands
	cmd.AddCommand(m2.NewCommandGet())
	cmd.AddCommand(m2.NewCommandVersion())
	cmd.AddCommand(m2.NewCommandCreate())
	cmd.AddCommand(m2.NewCommandUpdate())
	cmd.AddCommand(m2.NewCommandDelete())
	cmd.AddCommand(m2.NewCommandInit())
	cmd.AddCommand(m2.NewCommandNew())
	cmd.AddCommand(m2.NewCommandDestroy())
	cmd.AddCommand(m2.NewCommandVM())
	cmd.AddCommand(m2.NewCommandPull())
	cmd.AddCommand(guest.NewCommandGuest())
	cmd.AddCommand(m2.NewCommandInstall())
	cmd.AddCommand(m2.NewCommandJoin())
	return cmd
}

func globalFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&v1.G.Resource, "resource", "r", "cluster", "resource eg. cluster")
	cmd.PersistentFlags().StringVarP(&v1.G.OutPut, "output", "o", "", "yaml|json")
	cmd.Flags().BoolVarP(&v1.G.Cache, "cache", "c", true, "use cached file, default: true")
}

func NewKlogFlags() *flag.FlagSet {
	cmdline := flag.NewFlagSet("", flag.ExitOnError)
	klog.InitFlags(cmdline)
	return cmdline
}

func runE(cmd *cobra.Command, args []string) error { return nil }

// Run runs the `meridian` root command
func Run() error {
	return NewCommand().Execute()
}

// main wraps Run and sets the log formatter
func main() {
	ctrl.SetLogger(klogr.New())
	if err := Run(); err != nil {
		os.Exit(1)
	}
}
