package command

import (
	"encoding/json"
	"fmt"
	"github.com/aoxn/meridian"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime/schema"
	gschema "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"unicode"
)

const HelpLong = `
## Create a kubernetes cluster with ROS provider
## Get cluster list
## Get cluster specification
## Watch the cluster creation process
## Delete cluster created by wdrip with ROS provider
`

func NewCommandVersion() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "version",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf(meridian.Logo)
			fmt.Printf(meridian.Version)
			return nil
		},
	}
	return cmd
}

type cmdflag struct {
	config string
	cpus   int
	mems   string
	image  string
	arch   string

	withNodeGroups bool
	withKubernetes bool
}

const (
	DefaultEndpoint    = ""
	VirtualMachineShot = "vm"
	VirtualMachine     = "virtualmachine"
	ClusterResource    = "cluster"
	MasterSetResource  = "masterset"
	TaskResource       = "task"
	ImageResource      = "image"
	AddonResource      = "addon"
	KubeconfigResource = "kubeconfig"
	RequestResource    = "request"
)

func transformResource(resource string) string {
	switch resource {
	case VirtualMachineShot:
		return VirtualMachine
	}
	return resource
}

func toUpper(r string) string {
	if r == "" {
		return ""
	}
	k := []rune(r)
	k[0] = unicode.ToUpper(k[0])
	return string(k)
}

func gvk(r string) schema.GroupVersionKind {
	r = toUpper(r)

	for _, v := range scheme.KnownTypes(v1.GroupVersion) {
		name := strings.ToLower(v.Name())
		if name == strings.ToLower(r) {
			r = v.Name()
		}
	}
	klog.V(5).Infof("transform to kind: %s", r)
	return schema.GroupVersionKind{
		Group:   v1.GroupVersion.Group,
		Version: v1.GroupVersion.Version,
		Kind:    r,
	}
}

func toObject(src client.Object, dst client.Object) error {
	data, err := json.Marshal(src)
	if err != nil {
		return err
	}
	decoder := gschema.Codecs.UniversalDecoder()
	_, _, err = decoder.Decode(data, nil, dst)
	return err
}
