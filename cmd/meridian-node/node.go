package main

import (
	"context"
	"fmt"
	"github.com/aoxn/meridian"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/cmd/common"
	"github.com/aoxn/meridian/cmd/meridian/command"
	"github.com/aoxn/meridian/internal/node"
	"github.com/aoxn/meridian/internal/node/block/kubeadm"
	"github.com/aoxn/meridian/internal/tool"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
	"os"
	"path"
)

// NewCommandHost returns a new cobra.Command for cluster creation
func NewCommandHost() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "meridian-node",
		Short: "meridian-node",
		Long:  "meridian-node",
	}
	cmd.AddCommand(NewCommandInit())
	cmd.AddCommand(NewCommandJoin())
	cmd.AddCommand(NewCommandNew())
	cmd.AddCommand(NewCommandVersion())
	cmd.AddCommand(NewCommandDestroy())
	cmd.AddCommand(NewCommandCreate())
	cmd.AddCommand(command.NewCommandInstall())
	return cmd
}
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

func NewJoinRequest(req *v1.Request) *v1.Request {
	req.Spec.Config.TLS["root"].Key = []byte{}
	delete(req.Spec.Config.TLS, "svc")
	delete(req.Spec.Config.TLS, "front-proxy")
	delete(req.Spec.Config.TLS, "etcd-peer")
	delete(req.Spec.Config.TLS, "etcd-server")
	return req
}

// NewCommandDestroy create resource
func NewCommandDestroy() *cobra.Command {
	forceDestroy := false
	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "meridian destroy /etc/meridian/request.yaml",
		Long:  "meridian destroy /etc/meridian/request.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf(meridian.Logo)
			if len(args) < 1 {
				return fmt.Errorf("resource file is needed. eg. [/etc/meridian/request.yaml]")
			}
			r := args[0]
			switch r {
			case "docker":
				md, err := node.NewMeridianNode(
					"destroy", v1.NodeRoleMaster, "", "", nil, []string{})
				if err != nil {
					return err
				}
				return md.DestroyDocker(context.TODO())
			default:
				data, err := os.ReadFile(args[0])
				if err != nil {
					return err
				}
				req := &v1.Request{}
				err = yaml.Unmarshal(data, req)
				if err != nil {
					return err
				}
				md, err := node.NewMeridianNode("init", v1.NodeRoleMaster, "", "", req, []string{})
				if err != nil {
					return err
				}
				return md.DestroyNode(forceDestroy)
			}
		},
	}
	cmd.PersistentFlags().BoolVar(&forceDestroy, "force", false, "force destroy")
	return cmd
}

// NewCommandInit create resource
func NewCommandInit() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "meridian init",
		Long:  "",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf(meridian.Logo)
			if len(args) < 1 {
				return fmt.Errorf("config is needed for init")
			}
			data, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			req := &v1.Request{}
			err = yaml.Unmarshal(data, req)
			if err != nil {
				return err
			}
			md, err := node.NewMeridianNode(v1.ActionInit, v1.NodeRoleMaster, "", "", req, []string{})
			if err != nil {
				return errors.Wrapf(err, "meridian init")
			}
			return md.EnsureNode()
		},
	}
	return cmd
}

// NewCommandCreate create resource
func NewCommandCreate() *cobra.Command {
	var version string
	var registry string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "meridian create",
		Long:  "",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf(meridian.Logo)
			if len(args) < 1 {
				return fmt.Errorf("resource is needed for create")
			}
			r := args[0]
			switch r {
			case "docker":
				if version == "" || registry == "" {
					return fmt.Errorf("version or registry is needed for init")
				}
				md, err := node.NewMeridianNode(
					v1.ActionInit, v1.NodeRoleMaster, "", "", nil, []string{})
				if err != nil {
					return errors.Wrapf(err, "meridian init")
				}
				return md.CreateDocker(context.Background(), version, registry)
			default:
				return fmt.Errorf("unknown resource: %s", r)
			}
		},
	}
	cmd.Flags().StringVar(&version, "version", "", "docker version")
	cmd.Flags().StringVar(&registry, "registry", "", "registry version")
	return cmd
}

// NewCommandJoin create resource
func NewCommandJoin() *cobra.Command {
	var (
		role      = ""
		endpoint  = ""
		token     = ""
		nodeGroup = ""
		cloud     = ""
		labels    []string
	)
	cmd := &cobra.Command{
		Use:   "join",
		Short: "meridian join",
		Long:  "",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf(meridian.Logo)
			if role == "" || endpoint == "" || token == "" {
				return fmt.Errorf("role and endpoint and token required")
			}
			switch role {
			case string(v1.NodeRoleMaster), string(v1.NodeRoleWorker):
			default:
				return fmt.Errorf("invalid role: %s", role)
			}
			md, err := node.InitNode(v1.ActionJoin, v1.NodeRole(role), endpoint, token, nodeGroup, cloud, labels)
			if err != nil {
				return errors.Wrapf(err, "init meridian node")
			}
			return md.EnsureNode()
		},
	}
	cmd.PersistentFlags().StringVarP(&role, "role", "r", string(v1.NodeRoleWorker), "node role, one of Master|Worker")
	cmd.PersistentFlags().StringVarP(&endpoint, "api-server", "s", "", "meridian apiserver endpoint. eg. 192.168.1.1:6443")
	cmd.PersistentFlags().StringVarP(&token, "token", "t", "", "meridian kubeadm join token")
	cmd.PersistentFlags().StringVarP(&nodeGroup, "group", "g", "", "meridian node group")
	cmd.PersistentFlags().StringSliceVarP(&labels, "label", "l", nil, "register node labels")
	cmd.PersistentFlags().StringVarP(&cloud, "cloud", "c", "", "cloud type")
	return cmd
}

// NewCommandNew create resource
func NewCommandNew() *cobra.Command {
	var (
		join  = false
		write = ""
	)
	cmd := &cobra.Command{
		Use:   "new",
		Short: "meridian new",
		Long:  "",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf(meridian.Logo)
			if len(args) < 1 {
				return fmt.Errorf("resource is needed. [req]|[request]")
			}
			var (
				err error
				req = &v1.Request{}
			)
			if join {
				r, err := os.ReadFile(path.Join(kubeadm.KUBEADM_CONFIG_DIR, "request.yml"))
				if err != nil {
					return err
				}
				err = yaml.Unmarshal(r, req)
				if err != nil {
					return err
				}
				req = NewJoinRequest(req)
			} else {
				req, err = common.NewRequest()
				if err != nil {
					return errors.Wrapf(err, "build request")
				}
			}
			data := tool.PrettyYaml(req)
			if err != nil {
				return fmt.Errorf("new request template: %s", err.Error())
			}
			if write != "" {
				if !path.IsAbs(write) {
					dir, err := os.Getwd()
					if err != nil {
						klog.Infof("can not get current working directory")
						fmt.Println(data)
						return nil
					}
					write = path.Join(dir, write)
				}

				return os.WriteFile(write, []byte(data), 0755)
			} else {
				fmt.Printf("%s", data)
			}
			return nil
		},
	}
	cmd.PersistentFlags().StringVarP(&write, "write", "w", "", "write to file: request.yml in current dir")
	cmd.PersistentFlags().BoolVarP(&join, "join", "j", false, "generate join file: request-join.yml in current dir from request.yml")
	return cmd
}

func main() {
	err := NewCommandHost().Execute()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
