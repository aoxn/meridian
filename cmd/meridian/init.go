package meridian

import (
	"fmt"
	"github.com/aoxn/meridian"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/node"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"os"
)

// NewCommandInit create resource
func NewCommandInit() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "init",
		Hidden: true,
		Short:  "meridian init",
		Long:   "",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf(meridian.Logo)
			if len(args) < 1 {
				return fmt.Errorf("config is needed for init")
			}
			data, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			req := &v1.Request{}
			err = yaml.Unmarshal(data, req)
			if err != nil {
				return err
			}
			md, err := node.NewMeridianNode(v1.ActionInit, v1.NodeRoleMaster, "", "", req)
			if err != nil {
				return errors.Wrapf(err, "meridian init")
			}
			return md.EnsureNode()
		},
	}
	return cmd
}
