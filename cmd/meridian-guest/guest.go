package main

import (
	"fmt"
	"github.com/aoxn/meridian"
	"github.com/aoxn/meridian/internal/vma/guest"
	"github.com/spf13/cobra"
	"os"
)

// NewCommandGuest returns a new cobra.Command for cluster creation
func NewCommandGuest() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "meridian-guest",
		Short: "meridian-guest app",
		Long:  "meridian-guest app",
	}
	cmd.AddCommand(NewCommandGuestServe())
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

// NewCommandGuestServe returns a new cobra.Command for cluster creation
func NewCommandGuestServe() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "serve",
		Aliases: []string{"s"},
		Short:   "meridian guest serve, running apiserver in guest vm",
		Long:    "meridian guest serve, running apiserver in guest vm",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf(meridian.Logo)
			return Guest(args)
		},
	}
	return cmd
}

func Guest(args []string) error {
	return guest.RunDaemonAPI()
}

func main() {
	err := NewCommandGuest().Execute()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
