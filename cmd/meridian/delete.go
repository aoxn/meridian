package meridian

import (
	"context"
	"fmt"
	"github.com/aoxn/meridian"
	user "github.com/aoxn/meridian/internal/client"
	"github.com/aoxn/meridian/internal/vma/model"
	"github.com/spf13/cobra"
	u "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"os"
	"path/filepath"
)

func deleter(r string, args []string) error {
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
	case ImageResource:
		imgDir, err := model.MdImagesDir()
		if err != nil {
			return err
		}
		return os.RemoveAll(filepath.Join(imgDir, args[0]))
	default:
	}
	return resource.Delete(context.TODO(), &o)
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
			return deleter(args[0], args[1:])
		},
	}
	return cmd
}
