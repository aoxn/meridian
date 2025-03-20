package command

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aoxn/meridian"
	api "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/vma/download"
	"github.com/aoxn/meridian/internal/vma/model"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"os"
	"path/filepath"
	"runtime"
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
			if len(args) < 1 {
				return fmt.Errorf("image name is needed")
			}
			return PullImage(args[0])
		},
	}
	cmd.PersistentFlags().BoolVarP(&discover, "discover", "d", false, "discover available images")
	return cmd
}

func PullImage(name string) error {
	f := api.FindImage(name)
	if f == nil {
		return fmt.Errorf("image %s not found", name)
	}
	imgDir, err := model.MdImagesDir()
	if err != nil {
		return err
	}
	ipath := filepath.Join(imgDir, name)
	exist, err := api.Exist(ipath)
	if err != nil {
		return err
	}
	if exist {
		return fmt.Errorf("image %s already exists", name)
	}
	klog.Infof("debug go runtime arch: %q", runtime.GOARCH)
	_, err = download.DownloadFile(context.TODO(), "", *f, true, "download os images", f.Arch)
	if err != nil {
		return err
	}
	content, _ := json.MarshalIndent(f, "", "  ")
	err = os.MkdirAll(ipath, os.ModePerm)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(ipath, "data"), content, os.ModePerm)
}

func getImage() (*api.ImageList, error) {
	imageDir, err := model.MdImagesDir()
	if err != nil {
		return nil, err
	}
	var images = &api.ImageList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ImageList",
			APIVersion: api.GroupVersion.String(),
		},
		ListMeta: metav1.ListMeta{},
	}
	archs, err := os.Stat(imageDir)
	if err != nil {
		if os.IsNotExist(err) {
			return images, nil
		}
		return nil, err
	}
	if !archs.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", imageDir)
	}

	imageRepo, err := os.ReadDir(imageDir)
	if err != nil {
		return nil, err
	}
	for _, img := range imageRepo {
		if !img.IsDir() {
			continue
		}

		f := filepath.Join(
			imageDir,
			img.Name(),
			"data")
		data, err := os.ReadFile(f)
		if err != nil {
			klog.Infof("failed to read data from %s, %s", f, err)
			continue
		}
		m := &api.File{}
		err = json.Unmarshal(data, m)
		if err != nil {
			klog.Infof("failed to unmarshal data from %s, %s", f, err)
			continue
		}
		image := api.Image{
			ObjectMeta: metav1.ObjectMeta{
				Name: img.Name(),
			},
			TypeMeta: metav1.TypeMeta{
				Kind:       "Image",
				APIVersion: api.GroupVersion.String(),
			},
			Spec: api.ImageSpec{
				Name: m.Name,
				Arch: string(m.Arch),
				OS:   m.OS,
			},
		}
		images.Items = append(images.Items, image)
	}
	return images, nil
}
