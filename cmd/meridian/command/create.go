package command

import (
	"context"
	"fmt"
	"github.com/aoxn/meridian"
	v1 "github.com/aoxn/meridian/api/v1"
	user "github.com/aoxn/meridian/client"
	"github.com/aoxn/meridian/internal/vmm/meta"
	"github.com/ghodss/yaml"
	gerrors "github.com/pkg/errors"
	"github.com/spf13/cobra"
	"os"
	"runtime"
	"strings"
)

func createNew(flags *createflag, args []string) error {
	r := args[0]
	switch r {
	case VirtualMachine, VirtualMachineShot:
		return createVm(flags, args)
	case DockerResource:
		return createDocker(flags, args)
	case KubernetesResourceShot, KubernetesResource:
		return createK8s(flags, args)
	}
	return fmt.Errorf("unexpected resource: %s", r)
}
func createDocker(flags *createflag, args []string) error {
	client, err := user.Client(ListenSock)
	if err != nil {
		return err
	}
	var ctx = context.TODO()
	var name = flags.in
	if name == "" {
		return fmt.Errorf("vm name is required by --in=xxx ")
	}
	var spec = meta.Docker{Name: name}
	return client.Create(ctx, "docker", name, &spec)
}

func createK8s(flags *createflag, args []string) error {
	client, err := user.Client(ListenSock)
	if err != nil {
		return err
	}
	var ctx = context.TODO()
	var name = flags.in
	if name == "" {
		return fmt.Errorf("vm name is required by --in=xxx ")
	}
	var spec = meta.Kubernetes{Name: name, Version: flags.version, VmName: name}
	return client.Create(ctx, "k8s", name, &spec)
}

func createVm(flags *createflag, args []string) error {
	client, err := user.Client(ListenSock)
	if err != nil {
		return err
	}

	if len(args) < 2 {
		return fmt.Errorf("vm name must be specified: eg. [meridi create vm aoxn]")
	}
	var ctx = context.TODO()
	name := args[1]
	spec, err := newMachine(name, flags)
	if err != nil {
		return gerrors.Wrapf(err, "create vm")
	}
	return client.Create(ctx, "vm", name, &spec)
}

func newMachine(name string, flags *createflag) (*v1.VirtualMachineSpec, error) {
	var spec v1.VirtualMachineSpec
	if flags.config != "" {
		data, err := os.ReadFile(flags.config)
		if err != nil {
			return nil, err
		}
		err = yaml.Unmarshal(data, spec)
		return &spec, err
	}

	if flags.mems != "" {
		spec.Memory = flags.mems
	}
	if flags.cpus != 0 {
		spec.CPUs = flags.cpus
	}
	if flags.arch != "" {
		spec.Arch = v1.NewArch(flags.arch)
	}
	if flags.image != "" {
		var f = v1.FindImage(flags.image)
		if f == nil {
			return nil, fmt.Errorf("image %s not found", flags.image)
		}
		if f.Labels == nil {
			f.Labels = make(map[string]string)
		}
		gui := strings.ToLower(f.Labels["gui"])
		switch gui {
		case "false":
			spec.GUI = false
		case "true":
			spec.GUI = true
			spec.Video.Display = "default"
		}
		spec.OS = v1.OS(f.OS)
		if string(f.Arch) != myArch() {
			return nil, fmt.Errorf("local arch %s does not match image arch %s", runtime.GOARCH, f.Arch)
		}
		spec.Arch = f.Arch
		spec.Image = v1.ImageLocation{Name: flags.image}
	}
	return &spec, nil
}

func myArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	default:
	}
	return "x86_64"
}

// NewCommandCreate create resource
func NewCommandCreate() *cobra.Command {
	cmdline := &createflag{}
	cmd := &cobra.Command{
		Use:   "create",
		Short: "meridian create",
		Long:  "",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf(meridian.Logo)
			if len(args) < 1 {
				return fmt.Errorf("resource is needed for create")
			}
			return createNew(cmdline, args)
		},
		PreRunE: checkServerHeartbeat,
	}
	cmd.PersistentFlags().BoolVarP(&cmdline.withKubernetes, "with-kubernetes", "k", true, "with kubernetes")
	cmd.PersistentFlags().StringVarP(&cmdline.config, "config", "f", "", "virtual machine config")
	cmd.PersistentFlags().IntVar(&cmdline.cpus, "cpus", 4, "cpu count")
	cmd.PersistentFlags().StringVar(&cmdline.mems, "mems", "4GiB", "memory count")
	cmd.PersistentFlags().StringVar(&cmdline.image, "image", "", "with image name")
	cmd.PersistentFlags().StringVar(&cmdline.in, "in", "", "in which vm")

	cmd.PersistentFlags().BoolVarP(&cmdline.withNodeGroups, "with-nodegroups", "n", true, "with nodegroups support")
	cmd.PersistentFlags().StringVar(&cmdline.arch, "arch", "", "with arch")
	return cmd
}
