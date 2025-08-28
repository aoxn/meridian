package command

import (
	"fmt"
	"github.com/aoxn/meridian"
	api "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/vmm/meta"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

// NewCommandPull returns a new cobra.Command for cluster creation
func NewCommandPull() *cobra.Command {
	var discover bool
	cmd := &cobra.Command{
		Use:   "pull",
		Short: "meridian pull image",
		Long:  HelpLong,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf(meridian.Logo)
			if discover {
				klog.V(5).Infof("list available images")
				fmt.Printf("%-20s %-10s %-10s %s\n", "NAME", "OS", "ARCH", "DESCRIPTION")
				for _, v := range api.DftImages() {
					fmt.Printf("%-20s %-10s %-10s %s\n", v.Name, v.OS, v.Arch, "no description")
				}
				return nil
			}
			if len(args) < 2 {
				return fmt.Errorf("image name is needed")
			}
			if args[0] != "image" {
				return fmt.Errorf("only support [image]")
			}
			return PullImage(args[1])
		},
	}
	cmd.PersistentFlags().BoolVarP(&discover, "discover", "d", false, "discover available images")
	return cmd
}

func PullImage(name string) error {

	f := api.FindImage(name)
	if f == nil {
		return fmt.Errorf("unexpected image name: [%s], use[ m get image -d ] obtain available images", name)
	}

	backend, err := meta.NewLocal()
	if err != nil {
		return errors.Wrapf(err, "failed to load local image repo")
	}
	img := meta.Image{
		Name:     name,
		Arch:     string(f.Arch),
		OS:       f.OS,
		Labels:   f.Labels,
		Location: f.Location,
	}
	err = backend.Image().Pull(&img)
	if err != nil {
		return errors.Wrapf(err, "failed to pull image")
	}
	return backend.Image().Create(&img)
}

func getImage() ([]*meta.Image, error) {
	backend, err := meta.NewLocal()
	if err != nil {
		return nil, err
	}
	return backend.Image().List()
}
