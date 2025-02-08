package meridian

import (
	"context"
	"fmt"
	"github.com/aoxn/meridian"
	"github.com/aoxn/meridian/internal/apihandler"
	"github.com/spf13/cobra"
)

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
