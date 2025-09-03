package command

import (
	"context"
	"fmt"
	"github.com/aoxn/meridian"
	user "github.com/aoxn/meridian/client"
	"github.com/aoxn/meridian/client/rest"
	"github.com/aoxn/meridian/internal/vmm/meta"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"os"
	"os/exec"
	"strings"
	"time"

	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/tool"
	"github.com/pkg/errors"
)

var ListenSock = "/tmp/meridian.sock"

//
//func Get(r string, args []string, discover bool) error {
//	r = transformResource(r)
//	var (
//		id  = ""
//		ctx = context.TODO()
//	)
//	if len(args) > 0 {
//		id = args[0]
//	}
//	resource, err := user.Client(ListenSock)
//	if err != nil {
//		return errors.Wrapf(err, "service client failed")
//	}
//	var (
//		o      = u.Unstructured{}
//		header = func() {}
//		item   = func(obj client.Object) {}
//	)
//	o.SetGroupVersionKind(gvk(r))
//	switch r {
//	case ClusterResource:
//		header = func() {
//			fmt.Printf("%-40s%-20s%-40s\n", "NAME", "REPLICAS", "PAUSED")
//		}
//		item = func(obj client.Object) {
//			mo := &v1.Cluster{}
//			err := toObject(obj, mo)
//			if err != nil {
//				klog.Errorf("to object failed: %s", err.Error())
//				return
//			}
//			fmt.Printf("%-40s%-20d%-40t\n", mo.Name, *mo.Spec.MasterSpec.Replicas, mo.Spec.MasterSpec.Paused)
//		}
//	case VirtualMachine:
//		header = func() {
//			fmt.Printf("%-20s%-40s%-10s%-40s\n", "NAME", "ADDRESS", "STATE", "DOCKER_ENDPOINT")
//		}
//		item = func(obj client.Object) {
//			mo := &v1.VirtualMachine{}
//			err := toObject(obj, mo)
//			if err != nil {
//				klog.Errorf("to object failed: %s", err.Error())
//				return
//			}
//			fmt.Printf("%-20s%-40s%-10s%-40s\n", mo.Name, strings.Join(mo.Status.Address, ","),
//				mo.Status.Phase, fmt.Sprintf("[docker context use %s]", mo.Name))
//		}
//	case ImageResource:
//
//	case AddonResource:
//		header = func() {
//			fmt.Printf("%-30s%-30s%-15s%-25s%-20s\n", "NAME", "VERSION", "CATEGORY", "TEMPLATE_VERSION", "VM_CLUSTER")
//		}
//		item = func(obj client.Object) {
//			mo := &v1.VirtualMachine{}
//			err := toObject(obj, mo)
//			if err != nil {
//				klog.Errorf("to object failed: %s", err.Error())
//				return
//			}
//			addons := mo.Spec.Request.Config.Addons
//			sort.SliceStable(addons, func(i, j int) bool {
//				return addons[i].Category > addons[j].Category
//			})
//			for _, v := range addons {
//				fmt.Printf("%-30s%-30s%-15s%-25s%-20s\n", v.Name, v.Version, v.Category, v.TemplateVersion, mo.Name)
//			}
//		}
//	case KubeconfigResource:
//		vm := v1.EmptyVM(id)
//		err = resource.Get(ctx, vm)
//		if err != nil {
//			return errors.Wrapf(err, "get kubeconfig failed")
//		}
//		tls := vm.Spec.Request.Config.TLS
//		root, ok := tls["root"]
//		if !ok {
//			return fmt.Errorf("no root certificate")
//		}
//		key, crt, err := sign.SignKubernetesClient(root.Cert, root.Key, []string{})
//		if err != nil {
//			return fmt.Errorf("sign kubeconfig cert: %s", err.Error())
//		}
//		address := ""
//		for _, addr := range vm.Status.Address {
//			if strings.HasPrefix(addr, "192.168") {
//				address = addr
//				break
//			}
//		}
//		cfg, err := tool.RenderConfig(
//			"kubeconfig",
//			tool.KubeConfigTpl,
//			tool.RenderParam{
//				AuthCA:      base64.StdEncoding.EncodeToString(root.Cert),
//				Address:     address,
//				Port:        "6443",
//				ClusterName: v1.MeridianClusterName(vm.Name),
//				UserName:    v1.MeridianUserName(vm.Name),
//				ClientCRT:   base64.StdEncoding.EncodeToString(crt),
//				ClientKey:   base64.StdEncoding.EncodeToString(key),
//			},
//		)
//		if err != nil {
//			return fmt.Errorf("render kubeconfig error: %s", err.Error())
//		}
//		homecfg, err := block.HomeKubeCfg()
//		if err != nil {
//			return err
//		}
//		target := fmt.Sprintf("%s.%s", homecfg, vm.Name)
//		klog.Infof("write kubeconfig to %s", target)
//		return os.WriteFile(target, []byte(cfg), 0755)
//	default:
//		header = func() {
//			fmt.Printf("%-40s\n", "NAME")
//		}
//		item = func(o client.Object) {
//			fmt.Printf("%-40s\n", o.GetName())
//		}
//	}
//	switch r {
//	case ImageResource:
//
//	case AddonResource:
//		if discover {
//			if id != "" {
//				var (
//					name    = id
//					version = ""
//				)
//				sver := strings.Split(id, "@")
//				if len(sver) == 2 {
//					name = sver[0]
//					version = sver[1]
//				}
//				atpl := addons.GetAddonTplBy(name, version)
//				if atpl == nil {
//					return fmt.Errorf("addon not found: %s", id)
//				}
//				klog.Infof(tool.PrettyYaml(atpl))
//				return nil
//
//			}
//			sort.SliceStable(addons.DftAllAddons, func(i, j int) bool {
//				if addons.DftAllAddons[i].Version > addons.DftAllAddons[j].Version {
//					return true
//				}
//				return false
//			})
//
//			klog.V(5).Infof("list available addons")
//			fmt.Printf("%-20s %-30s%-15s\n", "NAME", "VERSION", "CATAGORY")
//			for _, v := range addons.DftAllAddons {
//				fmt.Printf("%-20s %-30s%-15s\n", v.Name, v.Version, v.Category)
//			}
//			return nil
//		}
//		if id == "" {
//			return fmt.Errorf("no vm id specified")
//		}
//		vm := v1.EmptyVM(id)
//		err = resource.Get(ctx, vm)
//		if err != nil {
//			return errors.Wrapf(err, "get addons failed")
//		}
//		data, _ := json.Marshal(vm)
//		err = json.Unmarshal(data, &o)
//		if err != nil {
//			return errors.Wrapf(err, "json unmarshal failed")
//		}
//	default:
//		switch id {
//		case "":
//			err = resource.List(ctx, &o)
//			if err != nil {
//				return errors.Wrapf(err, "request resource list")
//			}
//		default:
//			o.SetName(id)
//			err := resource.Get(ctx, &o)
//			if err != nil {
//				if strings.Contains(err.Error(), "NoSuchKey") {
//					return fmt.Errorf("no such cluster: %s", id)
//				}
//				return errors.Wrapf(err, "request masterset: %s", id)
//			}
//		}
//	}
//	return formatOut(&o, header, item)
//}

var (
	expectedResource = []string{
		ImageResource, ImagesResource,
		VirtualMachine,
		VirtualMachineShot,
		ClusterResource,
		AddonResource,
		KubeconfigResource,
		DockerResource,
	}
)

func GetNew(flags *commandFlags, args []string) error {
	r := args[0]
	switch r {
	case ImageResource, ImagesResource:
		return showImages(flags)
	case VirtualMachine, VirtualMachineShot:
		return showVms(flags)
	case DockerResource:
		return showDocker(flags)
	case KubernetesResource, KubernetesResourceShot:
		return showK8s(flags)
	default:
	}
	return fmt.Errorf("unknown resource [%s], available %s", r, expectedResource)
}

func showImages(flags *commandFlags) error {
	var (
		err  error
		imgs []*meta.Image
	)
	switch flags.discover {
	case true:
		for _, v := range v1.DftImages() {
			imgs = append(imgs, &meta.Image{
				Name:     v.Name,
				OS:       v.OS,
				Labels:   v.Labels,
				Arch:     string(v.Arch),
				Version:  v.Version,
				Location: v.Location,
			})
		}
	default:
		imgs, err = getImage()
		if err != nil {
			return errors.Wrapf(err, "get image failed")
		}
	}
	switch flags.output {
	case "json":
		fmt.Println(tool.PrettyJson(imgs))
	case "yaml", "yml":
		fmt.Println(tool.PrettyYaml(imgs))
	default:
		fmt.Printf("%-30s%-10s%-10s%-15s%-15s\n", "NAME", "OS", "ARCH", "VERSION", "LABELS")
		for _, img := range imgs {
			l := lo.MapToSlice(img.Labels, func(k string, v string) string {
				return fmt.Sprintf("%s:%s", k, v)
			})
			fmt.Printf("%-30s%-10s%-10s%-15s%-15s\n", img.Name, img.OS, img.Arch, img.Version, strings.Join(l, ","))
		}
	}
	return nil
}

func showVms(flags *commandFlags) error {
	var mchs []*meta.Machine
	client, err := user.Client(ListenSock)
	if err != nil {
		return errors.Wrap(err, "get client failed")
	}
	err = client.List(context.TODO(), "vm", &mchs)
	if err != nil {
		return errors.Wrap(err, "get vms failed")
	}

	switch flags.output {
	case "json":
		fmt.Println(tool.PrettyJson(mchs))
	case "yaml", "yml":
		fmt.Println(tool.PrettyYaml(mchs))
	default:
		fmt.Printf("%-15s%-10s%-8s%-8s%-8s%-10s%-20s\n",
			"NAME", "OS", "ARCH", "CPUs", "MEMs", "STATE", "ADDRESS")
		for _, mch := range mchs {
			addr := lo.Map(mch.Spec.Networks, func(item v1.Network, index int) string {
				return item.Address
			})
			fmt.Printf("%-15s%-10s%-8s%-8d%-8s%-10s%-20s\n",
				mch.Name, mch.Spec.OS, mch.Spec.Arch, mch.Spec.CPUs, mch.Spec.Memory,
				mch.State, strings.Join(addr, ","))
		}
	}
	return nil
}

func showDocker(flags *commandFlags) error {
	var mchs []*meta.Docker
	client, err := user.Client(ListenSock)
	if err != nil {
		return errors.Wrap(err, "get client failed")
	}
	err = client.List(context.TODO(), "docker", &mchs)
	if err != nil {
		return errors.Wrap(err, "get docker failed")
	}

	switch flags.output {
	case "json":
		fmt.Println(tool.PrettyJson(mchs))
	case "yaml", "yml":
		fmt.Println(tool.PrettyYaml(mchs))
	default:
		fmt.Printf("%-15s%-15s%-15s%-30s\n",
			"NAME", "VERSION", "REF_VM", "ENDPOINT")
		for _, mch := range mchs {
			fmt.Printf("%-15s%-15s%-15s%-30s\n",
				mch.Name, mch.Version, mch.VmName, fmt.Sprintf("[docker context use %s]", mch.Name))
		}
	}
	return nil
}

func showK8s(flags *commandFlags) error {
	var mchs []*meta.Kubernetes
	client, err := user.Client(ListenSock)
	if err != nil {
		return errors.Wrap(err, "get client failed")
	}
	err = client.List(context.TODO(), "k8s", &mchs)
	if err != nil {
		return errors.Wrap(err, "get k8s failed")
	}

	switch flags.output {
	case "json":
		fmt.Println(tool.PrettyJson(mchs))
	case "yaml", "yml":
		fmt.Println(tool.PrettyYaml(mchs))
	default:
		fmt.Printf("%-15s%-20s%-15s%-10s%-30s\n",
			"NAME", "VERSION", "REF_VM", "STATE", "ENDPOINT")
		for _, mch := range mchs {
			fmt.Printf("%-15s%-20s%-15s%-10s%-30s\n",
				mch.Name, mch.Spec.Config.Kubernetes.Version, mch.VmName, mch.State, fmt.Sprintf("[kubectl context use %s]", mch.Name))
		}
	}
	return nil
}

type commandFlags struct {
	output   string
	discover bool
}

// NewCommandGet returns a new cobra.Command for cluster creation
func NewCommandGet() *cobra.Command {
	flags := &commandFlags{}
	cmd := &cobra.Command{
		Use:   "get",
		Short: "meridian get cluster",
		Long:  HelpLong,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf(meridian.Logo)
			if len(args) < 1 {
				return fmt.Errorf("resource is needed for get")
			}
			//return Get(args[0], args[1:], discover)
			return GetNew(flags, args)
		},
		PreRunE: checkServerHeartbeat,
	}
	cmd.Flags().StringVarP(&flags.output, "output", "o", "", "output format: json,yaml")
	cmd.Flags().BoolVarP(&flags.discover, "discover", "d", false, "discover available addons from server")
	return cmd
}

func checkServerHeartbeat(cmd *cobra.Command, _ []string) error {
	return nil
}

func checkServerHeartbeat2(cmd *cobra.Command, _ []string) error {
	c, err := user.Client(ListenSock)
	if err != nil {
		return errors.Wrapf(err, "service client failed")
	}
	err = heartbeat(c.Raw())
	if err != nil {
		if !strings.Contains(err.Error(), " refused") {
			return err
		}
		if err := startApp(cmd.Context(), c.Raw()); err != nil {
			return errors.New("could not connect to ollama app, is it running?")
		}
	}
	return nil
}

func heartbeat(client rest.Interface) error {
	health := v1.Healthy{}
	return client.Get(context.TODO()).PathPrefix("/healthy").Do(&health)
}

func startApp(ctx context.Context, client rest.Interface) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	link, err := os.Readlink(exe)
	if err != nil {
		return err
	}
	if !strings.Contains(link, "Meridian.app") {
		return errors.New("could not find meridian app")
	}
	path := strings.Split(link, "Meridian.app")
	if err := exec.Command("/usr/bin/open", "-a", path[0]+"Meridian.app").Run(); err != nil {
		return err
	}
	// waitForever
	// wait for the server to start
	timeout := time.After(5 * time.Second)
	tick := time.Tick(500 * time.Millisecond)
	for {
		select {
		case <-timeout:
			return errors.New("timed out waiting for server to start")
		case <-tick:
			if err := heartbeat(client); err == nil {
				return nil // server has started
			}
		case <-ctx.Done():
			return fmt.Errorf("context canceled waiting for server to start")
		}
	}
}
