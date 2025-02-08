package meridian

import (
	"fmt"
	"github.com/aoxn/meridian"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/node"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// NewCommandJoin create resource
func NewCommandJoin() *cobra.Command {
	var (
		role      = ""
		endpoint  = ""
		token     = ""
		nodeGroup = ""
		cloud     = ""
	)
	cmd := &cobra.Command{
		Use:    "join",
		Hidden: true,
		Short:  "meridian join",
		Long:   "",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf(meridian.Logo)
			if role == "" || endpoint == "" || token == "" {
				return fmt.Errorf("role and endpoint and token required")
			}
			switch role {
			case string(v1.NodeRoleMaster), string(v1.NodeRoleWorker):
			default:
				return fmt.Errorf("invalid role: %s", role)
			}
			md, err := node.InitNode(v1.ActionJoin, v1.NodeRole(role), endpoint, token, nodeGroup, cloud)
			if err != nil {
				return errors.Wrapf(err, "init meridian node")
			}
			return md.EnsureNode()
		},
	}
	cmd.PersistentFlags().StringVarP(&role, "role", "r", string(v1.NodeRoleWorker), "node role, one of Master|Worker")
	cmd.PersistentFlags().StringVarP(&endpoint, "api-server", "s", "", "meridian apiserver endpoint. eg. 192.168.1.1:6443")
	cmd.PersistentFlags().StringVarP(&token, "token", "t", "", "meridian kubeadm join token")
	cmd.PersistentFlags().StringVarP(&nodeGroup, "group", "g", "", "meridian node group")

	cmd.PersistentFlags().StringVarP(&cloud, "cloud", "c", "", "cloud type")
	return cmd
}
