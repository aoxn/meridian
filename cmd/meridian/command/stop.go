package command

import (
	"context"
	"fmt"
	"github.com/aoxn/meridian"
	user "github.com/aoxn/meridian/client"
	"github.com/aoxn/meridian/internal/vmm/meta"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func stop(flags *commandFlags, args []string) error {
	r := args[0]
	switch r {
	case VirtualMachine, VirtualMachineShot:
		return stopVm(flags, args[1:])
	default:
	}
	return fmt.Errorf("unknown resource [%s], available %s", r, expectedResource)
}

func stopVm(flags *commandFlags, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("vm name is required")
	}
	var name = args[0]
	client, err := user.Client(ListenSock)
	if err != nil {
		return errors.Wrap(err, "get client failed")
	}
	var vm = meta.Machine{}
	return client.Update(context.TODO(), "vm/stop", name, &vm)
}

// NewCommandStop returns a new cobra.Command for cluster creation
func NewCommandStop() *cobra.Command {
	flags := &commandFlags{}
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "meridian stop vm",
		Long:  HelpLong,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf(meridian.Logo)
			if len(args) < 1 {
				return fmt.Errorf("resource is needed for get")
			}
			return stop(flags, args)
		},
	}
	return cmd
}
