package meridian

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aoxn/meridian"
	v1 "github.com/aoxn/meridian/api/v1"
	user "github.com/aoxn/meridian/internal/client"
	"github.com/aoxn/meridian/internal/tool"
	"github.com/aoxn/meridian/internal/vma/model"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	u "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

func create(r string, args []string, cmdline *cmdflag) error {
	r = transformResource(r)
	resource, err := user.Client(ListenSock)
	if err != nil {
		return err
	}
	var (
		o = u.Unstructured{}
	)
	if len(args) <= 0 {
		return fmt.Errorf("vm name must be specified: eg. [meridian create vm aoxn]")
	}
	o.SetName(args[0])
	o.SetGroupVersionKind(gvk(r))
	switch r {
	case VirtualMachine:
		var (
			err             error
			loadFromHistory bool = false
			virt                 = &v1.VirtualMachine{}
		)
		if cmdline.config == "" {
			virt, err = ReadHistory(args[0])
			if err != nil {
				if !strings.Contains(err.Error(), "no such file or directory") {
					return err
				}
				klog.Infof("read history vm: %s", err.Error())
				// use default config
				virt, err = v1.LoadDft()
				if err != nil {
					return errors.Wrapf(err, "load default vm config")
				}
			} else {
				loadFromHistory = true
				klog.Infof("use default set from history vm[%s]", args[0])
			}
		} else {
			data, err := os.ReadFile(cmdline.config)
			if err != nil {
				return err
			}
			err = yaml.Unmarshal(data, virt)
			if err != nil {
				return err
			}
		}
		if len(args) > 0 {
			virt.Name = args[0]
		} else {
			virt.Name = "dft"
		}
		if virt.Name == "" {
			return fmt.Errorf("vm name is required")
		}
		if cmdline.cpus != 0 {
			virt.Spec.CPUs = cmdline.cpus
		}
		if cmdline.mems != "" {
			virt.Spec.Memory = cmdline.mems
		}
		if cmdline.withKubernetes && !loadFromHistory {
			req, err := NewRequest()
			if err != nil {
				return err
			}
			virt.Spec.Request = req.Spec
		}
		if cmdline.withNodeGroups {
			if virt.Spec.Request.Config.Features == nil {
				virt.Spec.Request.Config.Features = make(map[string]string)
			}
			virt.Spec.Request.Config.Features[v1.FeatureSupportNodeGroups] = ""
		}
		if cmdline.arch != "" {
			virt.Spec.Arch = v1.NewArch(cmdline.arch)
		} else {
			virt.Spec.Arch = v1.NewArch(runtime.GOARCH)
		}
		if cmdline.image != "" {
			f := v1.FindImage(cmdline.image)
			if f == nil {
				return fmt.Errorf("image %s not found", cmdline.image)
			}
			if f.Arch != virt.Spec.Arch {
				return fmt.Errorf("image %s arch does not match, imageArch=%s, virtArch=%s", cmdline.image, f.Arch, virt.Spec.Arch)
			}
			virt.Spec.Image.Name = f.Name
		} else {
			// get dft image
			f := v1.FindDftImageBy(string(virt.Spec.OS), string(virt.Spec.Arch))
			if f == nil {
				return fmt.Errorf("default image not found by: os=%s, arch=%s", virt.Spec.OS, virt.Spec.Arch)
			}
			virt.Spec.Image.Name = f.Name
		}
		defer SetHistory(virt.Name, virt)
		klog.Infof("create vm: [%s]", virt.Name)
		klog.V(5).Infof("create virtual machine: %s", tool.PrettyYaml(virt))
		return resource.Create(context.TODO(), virt)
	case ClusterResource:
		err = u.SetNestedField(o.Object, int64(1), "spec", "masterSpec", "replicas")
		if err != nil {
			return errors.Wrapf(err, "set gvk")
		}
	case TaskResource, KubeconfigResource, MasterSetResource:
		return fmt.Errorf("unimplemnted create: %s", toUpper(r))
	}
	return resource.Create(context.TODO(), &o)
}

// NewCommandCreate create resource
func NewCommandCreate() *cobra.Command {
	cmdline := &cmdflag{}
	cmd := &cobra.Command{
		Use:   "create",
		Short: "meridian create",
		Long:  "",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf(meridian.Logo)
			if len(args) < 1 {
				return fmt.Errorf("resource is needed for create")
			}
			return create(args[0], args[1:], cmdline)
		},
		PreRunE: checkServerHeartbeat,
	}
	cmd.PersistentFlags().BoolVarP(&cmdline.withKubernetes, "with-kubernetes", "k", true, "with kubernetes")
	cmd.PersistentFlags().StringVarP(&cmdline.config, "config", "c", "", "virtual machine config")
	cmd.PersistentFlags().IntVar(&cmdline.cpus, "cpus", 4, "cpu count")
	cmd.PersistentFlags().StringVar(&cmdline.mems, "mems", "4GiB", "memory count")
	cmd.PersistentFlags().StringVar(&cmdline.image, "image", "", "with image name")

	cmd.PersistentFlags().BoolVarP(&cmdline.withNodeGroups, "with-nodegroups", "n", true, "with nodegroups support")
	cmd.PersistentFlags().StringVar(&cmdline.arch, "arch", "", "with arch")
	return cmd
}

func SetHistory(name string, req *v1.VirtualMachine) {
	mdHome, err := model.MdHOME()
	if err != nil {
		klog.Infof("set history %s: %v", name, err)
		return
	}
	historyPath := filepath.Join(mdHome, "_history")
	_ = os.MkdirAll(historyPath, 0755)
	history := filepath.Join(historyPath, name)
	klog.Infof("write to history [%s]", history)
	_ = os.WriteFile(history, []byte(tool.PrettyJson(req)), 0755)
}

func ReadHistory(name string) (*v1.VirtualMachine, error) {
	mdHome, err := model.MdHOME()
	if err != nil {
		return nil, err
	}
	historyPath := filepath.Join(mdHome, "_history")
	_ = os.MkdirAll(historyPath, 0755)
	data, err := os.ReadFile(path.Join(historyPath, name))
	if err != nil {
		return nil, err
	}
	var virt v1.VirtualMachine
	err = json.Unmarshal(data, &virt)
	return &virt, err
}
