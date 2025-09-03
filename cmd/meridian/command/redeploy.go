package command

import (
	"context"
	"fmt"
	"github.com/aoxn/meridian"
	user "github.com/aoxn/meridian/client"
	"github.com/aoxn/meridian/internal/vmm/meta"
	"github.com/spf13/cobra"
)

func redeploy(r string, args []string) error {
	if len(args) <= 0 {
		return fmt.Errorf("id must be provided")
	}
	switch r {
	case DockerResource:
		if len(args) < 2 {
			return fmt.Errorf("id must be provided")
		}
		return redeployDocker(args[1])
	case KubernetesResource, KubernetesResourceShot:
		if len(args) < 2 {
			return fmt.Errorf("id must be provided")
		}
		return redeployK8s(args[1])
	default:
	}
	return fmt.Errorf("unknown resource %s", r)
}
func redeployDocker(name string) error {
	resource, err := user.Client(ListenSock)
	if err != nil {
		return err
	}

	return resource.Update(context.TODO(), "docker/redeploy", name, &meta.Docker{Name: name})
}

func redeployK8s(name string) error {
	resource, err := user.Client(ListenSock)
	if err != nil {
		return err
	}

	return resource.Update(context.TODO(), "k8s/redeploy", name, &meta.Kubernetes{Name: name})
}

// NewCommandRedeploy delete resource
func NewCommandRedeploy() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "redeploy",
		Short: "meridian redeploy",
		Long:  "",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf(meridian.Logo)
			if len(args) < 1 {
				return fmt.Errorf("resource is needed for delete")
			}
			return redeploy(args[0], args)
		},
	}
	return cmd
}
