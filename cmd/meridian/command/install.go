package command

import (
	"context"
	"fmt"
	"github.com/aoxn/meridian"
	api "github.com/aoxn/meridian/api/v1"
	user "github.com/aoxn/meridian/internal/client"
	"github.com/aoxn/meridian/internal/node/block/post/addons"
	"github.com/aoxn/meridian/internal/tool/kubeclient"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
	"strings"
)

var (
	discoverAll  bool
	discover     bool
	forVm        string
	forNodeGroup string
)

// NewCommandInstall returns a new cobra.Command for cluster creation
func NewCommandInstall() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "meridian install [addon]",
		Long:  HelpLong,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf(meridian.Logo)
			if len(args) < 1 {
				return fmt.Errorf("resource is needed for install command")
			}
			switch args[0] {
			case "addon":
				return installAddon("addon", args[1:])
			}
			return fmt.Errorf("unknown install command for resource: %s", args[0])
		},
	}
	cmd.PersistentFlags().BoolVarP(&discoverAll, "all", "a", false, "discover all available addons")
	cmd.PersistentFlags().BoolVarP(&discover, "discover", "d", false, "discover available custom addons")
	cmd.PersistentFlags().StringVarP(&forVm, "for-vm", "n", "", "for vm name ")
	cmd.PersistentFlags().StringVarP(&forNodeGroup, "for-nodegroup", "g", "", "for nodegroup ")
	return cmd
}

func installAddon(r string, args []string) error {
	if discover {
		klog.V(5).Infof("list available addons")
		fmt.Printf("%-20s %-30s%-15s\n", "NAME", "VERSION", "CATAGORY")
		for _, v := range addons.DftAllAddons {
			if !discoverAll && v.Category == "System" {
				continue
			}
			fmt.Printf("%-20s %-30s%-15s\n", v.Name, v.Version, v.Category)
		}
		return nil
	}
	if len(args) < 1 {
		return fmt.Errorf("resource name is needed")
	}
	addonName := args[0]
	if addonName == "" {
		return fmt.Errorf("addon name is needed")
	}
	var (
		err   error
		addon = api.Addon{}
	)
	if strings.Contains(addonName, "@") {
		ver := strings.Split(addonName, "@")
		if len(ver) != 2 {
			return fmt.Errorf("invalid addon: %s", addonName)
		}
		addon.Name = ver[0]
		addon.Version = ver[1]
		klog.V(5).Infof("install addon specified by user: %s/%s", addon.Name, addon.Version)
	} else {
		addon, err = addons.GetAddonByName(addonName)
		if err != nil {
			return err
		}
		klog.V(5).Infof("install addon from system default: %s/%s", addon.Name, addon.Version)
	}
	if forVm == "" {
		return fmt.Errorf("for-vm is needed")
	}
	client, err := user.Client(ListenSock)
	if err != nil {
		return errors.Wrapf(err, "service client failed")
	}
	var (
		vm = api.EmptyVM(forVm)
	)
	err = client.Get(context.TODO(), vm)
	if err != nil {
		return errors.Wrapf(err, "get vm failed")
	}
	vm.Spec.Request.Config.SetAddon(&addon)
	yml, err := addons.RenderAddon(addonName, &addons.RenderData{
		R: api.NewEmptyRequest(vm.Name, vm.Spec.Request), NodeGroup: forNodeGroup,
	})
	if err != nil {
		return errors.Wrapf(err, "get addon failed")
	}
	defer func() {
		err = saveVmConfig(vm)
		if err != nil {
			klog.Errorf("save vm config failed: %v", err)
		}
		// todo: why addon
		klog.Infof("install addon [%s] for %s complete", addonName, vm.Name)
	}()
	return kubeclient.Guest(vm).Apply(yml)
}

func saveVmConfig(vm *api.VirtualMachine) error {
	resource, err := user.Client(ListenSock)
	if err != nil {
		return errors.Wrapf(err, "service client failed")
	}
	return resource.Update(context.TODO(), vm)
}
