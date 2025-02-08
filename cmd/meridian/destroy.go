package meridian

import (
	"fmt"
	"github.com/aoxn/meridian"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/node"
	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
	"os"
)

// NewCommandDestroy create resource
func NewCommandDestroy() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "destroy",
		Hidden: true,
		Short:  "meridian destroy",
		Long:   "",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf(meridian.Logo)
			if len(args) < 1 {
				return fmt.Errorf("resource is needed. [req]|[request]")
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
			md, err := node.NewMeridianNode("init", v1.NodeRoleMaster, "", "", req)
			if err != nil {
				return err
			}
			return md.DestroyNode()
		},
	}
	return cmd
}
