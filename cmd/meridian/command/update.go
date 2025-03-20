package command

import (
	"context"
	"fmt"
	"github.com/aoxn/meridian"
	v1 "github.com/aoxn/meridian/api/v1"
	user "github.com/aoxn/meridian/internal/client"
	"github.com/spf13/cobra"
	u "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

var scheme = runtime.NewScheme()

func init() {
	err := v1.AddToScheme(scheme)
	if err != nil {
		panic(fmt.Sprintf("adding to scheme: %s", err.Error()))
	}
}

func updater(r string, args []string) error {
	if len(args) <= 0 {
		return fmt.Errorf("id must be provided")
	}
	r = transformResource(r)
	resource, err := user.Client(ListenSock)
	if err != nil {
		return err
	}
	var (
		o = u.Unstructured{}
	)

	o.SetName(args[0])
	o.SetGroupVersionKind(gvk(r))
	switch r {
	case VirtualMachine:
		return resource.Update(context.TODO(), &o)
	default:
	}
	return fmt.Errorf("unimplemented resource: %s", r)
}

// NewCommandUpdate update resource
func NewCommandUpdate() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "update",
		Short: "meridian update resource",
		Long:  "",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf(meridian.Logo)
			if len(args) < 1 {
				return fmt.Errorf("resource is needed for delete")
			}
			return updater(args[0], args[1:])
		},
	}
	return cmd
}
