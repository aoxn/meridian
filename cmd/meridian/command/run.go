package command

import (
	"context"
	"fmt"
	"github.com/aoxn/meridian"
	user "github.com/aoxn/meridian/client"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func run(flags *createflag, args []string) error {
	r := args[0]
	switch r {
	case VirtualMachine, VirtualMachineShot:
		return runVm(flags, args[1:])
	default:
	}
	return fmt.Errorf("unknown resource [%s], available %s", r, expectedResource)
}

func runVm(flags *createflag, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("vm name is required")
	}
	var name = args[0]
	vm, err := newMachine(name, flags)
	if err != nil {
		return errors.Wrap(err, "new vm")
	}
	client, err := user.Client(ListenSock)
	if err != nil {
		return errors.Wrap(err, "get client failed")
	}
	return client.Create(context.TODO(), "vm/run", name, &vm)
}

// NewCommandRun returns a new cobra.Command for cluster creation
func NewCommandRun() *cobra.Command {
	flags := &createflag{}
	cmd := &cobra.Command{
		Use:   "run",
		Short: "meridian run vm",
		Long:  HelpLong,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf(meridian.Logo)
			if len(args) < 1 {
				return fmt.Errorf("resource is needed for get")
			}
			return run(flags, args)
		},
	}
	cmd.PersistentFlags().BoolVarP(&flags.withKubernetes, "with-kubernetes", "k", true, "with kubernetes")
	cmd.PersistentFlags().StringVarP(&flags.config, "config", "f", "", "virtual machine config")
	cmd.PersistentFlags().IntVar(&flags.cpus, "cpus", 4, "cpu count")
	cmd.PersistentFlags().StringVar(&flags.mems, "mems", "4GiB", "memory count")
	cmd.PersistentFlags().StringVar(&flags.image, "image", "", "with image name")

	cmd.PersistentFlags().BoolVarP(&flags.withNodeGroups, "with-nodegroups", "n", true, "with nodegroups support")
	cmd.PersistentFlags().StringVar(&flags.arch, "arch", "", "with arch")
	return cmd
}
