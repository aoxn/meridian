package command

import (
	"context"
	"fmt"
	"github.com/aoxn/meridian"
	user "github.com/aoxn/meridian/client"
	"github.com/aoxn/meridian/internal/vmm/meta"
	"github.com/spf13/cobra"
)

func deleter(r string, args []string) error {
	if len(args) <= 0 {
		return fmt.Errorf("id must be provided")
	}
	switch r {
	case ImageResource, ImagesResource:
		if len(args) < 2 {
			return fmt.Errorf("id must be provided")
		}
		return deleteImage(args[1])
	case VirtualMachineShot, VirtualMachine:
		if len(args) < 2 {
			return fmt.Errorf("id must be provided")
		}
		return deleteVm(args[1])
	case DockerResource:
		if len(args) < 2 {
			return fmt.Errorf("id must be provided")
		}
		return deleteDocker(args[1])
	case KubernetesResource, KubernetesResourceShot:
		if len(args) < 2 {
			return fmt.Errorf("id must be provided")
		}
		return deleteK8s(args[1])
	default:
	}
	return fmt.Errorf("unknown resource %s", r)
}
func deleteDocker(name string) error {
	resource, err := user.Client(ListenSock)
	if err != nil {
		return err
	}

	return resource.Delete(context.TODO(), "docker", name, &meta.Docker{})
}

func deleteK8s(name string) error {
	resource, err := user.Client(ListenSock)
	if err != nil {
		return err
	}

	return resource.Delete(context.TODO(), "k8s", name, &meta.Kubernetes{})
}

func deleteVm(name string) error {
	resource, err := user.Client(ListenSock)
	if err != nil {
		return err
	}

	return resource.Delete(context.TODO(), "vm", name, &meta.Machine{})
}

func deleteImage(name string) error {
	resource, err := user.Client(ListenSock)
	if err != nil {
		return err
	}

	return resource.Delete(context.TODO(), "image", name, &meta.Image{})
}

// NewCommandDelete delete resource
func NewCommandDelete() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "meridian delete cluster",
		Long:  "",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf(meridian.Logo)
			if len(args) < 1 {
				return fmt.Errorf("resource is needed for delete")
			}
			return deleter(args[0], args)
		},
	}
	return cmd
}
