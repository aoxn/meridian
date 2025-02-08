package runtime

import (
	"context"
	"fmt"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/node/block"
	"github.com/aoxn/meridian/internal/node/block/file"
	"github.com/aoxn/meridian/internal/node/host"
	"github.com/aoxn/meridian/internal/tool/cmd"
	apt "github.com/arduino/go-apt-client"
	"github.com/pkg/errors"
	"io/ioutil"
	"k8s.io/klog/v2"
	"os"
)

type containerdBlock struct {
	req  *v1.Request
	host host.Host
	file *file.File
}

func NewContainerdBlock(req *v1.Request, host host.Host) (block.Block, error) {

	info := file.PathInfo{
		InnerAddr: false,
		Arch:      host.Arch(),
		OSRelease: host.OS(),
		Region:    host.Region(),
	}
	err := info.Validate()
	if err != nil {
		return nil, err
	}
	return &containerdBlock{
		req:  req,
		host: host,
		file: &file.File{
			Path:    info,
			Pkg:     file.PKG_CONTAINERD,
			Ftype:   file.FILE_PKG,
			Version: req.Spec.Config.Runtime.Version,
		},
	}, nil
}

// Ensure runs the action
func (a *containerdBlock) Ensure(ctx context.Context) error {
	err := os.MkdirAll("/etc/docker", 0755)
	if err != nil {
		return fmt.Errorf("make /etc/docker dir: %s", err.Error())
	}
	err = os.MkdirAll("/etc/containerd", 0755)
	if err != nil {
		return fmt.Errorf("make /etc/containerd dir: %s", err.Error())
	}
	if err = a.file.Ensure(ctx); err != nil {
		return errors.Wrapf(err, "install containerd runtime: %s", a.req.Name)
	}

	err = os.WriteFile("/etc/containerd/config.toml", containerdCfg(a.req.Spec.Config.Registry), 0755)
	if err != nil {
		return fmt.Errorf("write docker config: %s", err.Error())
	}

	for f, v := range cfgs {
		err = ioutil.WriteFile(f, []byte(v), 0755)
		if err != nil {
			return fmt.Errorf("write docker config: %s", err.Error())
		}
	}
	klog.Infof("add docker group...")
	sta := <-cmd.NewCmd("groupadd", "-r", "docker").Start()
	if err := cmd.CmdError(sta); err != nil {
		klog.Errorf("add docker group error: %s", err.Error())
	}

	err = os.WriteFile(containerdServiceFile, containerService(), 0755)
	if err != nil {
		return fmt.Errorf("write docker config: %s", err.Error())
	}

	err = a.host.Service().Enable("docker")
	if err != nil {
		return fmt.Errorf("systecmctl enable docker error,%s ", err.Error())
	}
	err = a.host.Service().Restart("docker")
	if err != nil {
		return fmt.Errorf("systecmctl start docker error,%s ", err.Error())
	}
	err = a.host.Service().Enable("containerd")
	if err != nil {
		return fmt.Errorf("systecmctl enable containerd error,%s ", err.Error())
	}
	return a.host.Service().Restart("containerd")
}

func (a *containerdBlock) Purge(ctx context.Context) error {
	err := a.host.Service().Disable("containerd")
	if err != nil {
		return errors.Wrapf(err, "disable containerd")
	}
	err = a.host.Service().Stop("containerd")
	if err != nil {
		return errors.Wrapf(err, "stop containerd")
	}
	switch a.file.Path.OSRelease {
	case host.Ubuntu:
		var pkg []*apt.Package
		for _, i := range []string{
			"cri-tools",
			"docker-ce",
			"docker-ce-cli",
			"containerd.io",
		} {
			found, err := apt.Search(i)
			if err != nil {
				klog.Warningf("find package[%s] with error: %s", i, err.Error())
				continue
			}
			if len(found) <= 0 {
				continue
			}
			for _, p := range found {
				klog.Infof("meridian found package: [name=%s], [version=%s]", p.Name, p.Version)
			}
			pkg = append(pkg, &apt.Package{Name: i})
		}
		if len(pkg) > 0 {
			data, err := apt.Remove(pkg...)
			if err != nil {
				return errors.Wrapf(err, "remove package: %s", toPkg(pkg))
			}
			klog.Infof("do remove pkg: %s, %s", toPkg(pkg), data)
		}
	default:
		return fmt.Errorf("unimplemented os [%s] for uninstall pkg", a.file.Path.OSRelease)
	}

	files := []string{
		"/etc/docker",
		"/etc/containerd/config.toml",
		containerdServiceFile,
		"/etc/containerd/",
	}
	files = append(files, keys(cfgs)...)

	for _, r := range files {
		klog.Infof("remove containerd config file: [%s]", r)
		err := os.RemoveAll(r)
		if err != nil {
			return err
		}
	}
	return nil
}

func keys(m map[string]string) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

func (a *containerdBlock) CleanUp(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (a *containerdBlock) Name() string {
	return fmt.Sprintf("containerd init [%s]", a.host.NodeID())
}

func containerdCfg(registry string) []byte {
	var tpl = `
version = 2
root = "/var/lib/containerd"
state = "/run/containerd"
disabled_plugins = []
required_plugins = ["io.containerd.grpc.v1.cri"]
oom_score = -999

[grpc]
  address = "/run/containerd/containerd.sock"
  max_recv_message_size = 16777216
  max_send_message_size = 16777216

[debug]
  address = "/run/containerd/debug.sock"
  level = "info"

[timeouts]
  "io.containerd.timeout.shim.cleanup" = "5s"
  "io.containerd.timeout.shim.load" = "5s"
  "io.containerd.timeout.shim.shutdown" = "3s"
  "io.containerd.timeout.task.state" = "2s"

[plugins]
  [plugins."io.containerd.gc.v1.scheduler"]
    pause_threshold = 0.02
    deletion_threshold = 0
    mutation_threshold = 100
    schedule_delay = "0s"
    startup_delay = "100ms"

  [plugins."io.containerd.grpc.v1.cri"]
    sandbox_image = "%s/pause:3.5"
    ignore_image_defined_volumes = true
    [plugins."io.containerd.grpc.v1.cri".containerd]
      snapshotter = "overlayfs"
      default_runtime_name = "runc"
      disable_snapshot_annotations = true
      discard_unpacked_layers = false

      [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
          runtime_type = "io.containerd.runc.v2"
          privileged_without_host_devices = false
          [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
            NoPivotRoot = false
            NoNewKeyring = false
            SystemdCgroup = true
    [plugins."io.containerd.grpc.v1.cri".cni]
      bin_dir = "/opt/cni/bin"
      conf_dir = "/etc/cni/net.d"
      max_conf_num = 1
    [plugins."io.containerd.grpc.v1.cri".registry]
      config_path = "/etc/containerd/cert.d"

  [plugins."io.containerd.internal.v1.opt"]
    path = "/opt/containerd"

  [plugins."io.containerd.internal.v1.restart"]
    interval = "10s"

  [plugins."io.containerd.metadata.v1.bolt"]
    content_sharing_policy = "shared"
`

	return []byte(fmt.Sprintf(tpl, fmt.Sprintf("%s/acs", registry)))
}

var containerdServiceFile = "/usr/lib/systemd/system/containerd.service"

func containerService() []byte {
	var spec = `
[Unit]
Description=containerd container runtime
Documentation=https://containerd.io
After=network.target local-fs.target

[Service]
ExecStartPre=-/sbin/modprobe overlay
ExecStart=/usr/bin/containerd

Type=notify
Delegate=yes
KillMode=process
Restart=always
RestartSec=5
# Having non-zero Limit*s causes performance problems due to accounting overhead
# in the kernel. We recommend using cgroups to do container-local accounting.
LimitNPROC=infinity
LimitCORE=infinity
LimitNOFILE=1048576
# Comment TasksMax if your systemd version does not supports it.
# Only systemd 226 and above support this version.
TasksMax=infinity
OOMScoreAdjust=-999

[Install]
WantedBy=multi-user.target
	`
	return []byte(spec)
}

func toPkg(pkg []*apt.Package) []string {
	var name []string
	for _, v := range pkg {
		name = append(name, v.Name)
	}
	return name
}
