package tpl

// ConfigTemplateBetaV1 is the kubadm config template for API version v1beta1
const ConfigTemplateBetaV1 = `# config generated by kind
apiVersion: kubeadm.k8s.io/v1beta1
kind: ClusterConfiguration
metadata:
  name: config
kubernetesVersion: {{.cfg.Kubernetes.Version}}
clusterName: "{{.cfg.Kubernetes.Key}}"
{{ if .cfg.Endpoint -}}
controlPlaneEndpoint: {{ .cfg.Endpoint }}
{{- end }}
# on runtime for mac we have to expose the api server via port forward,
# so we need to ensure the cert is valid for localhost so we can talk
# to the cluster after rewriting the kubeconfig to point to localhost
apiServer:
  certSANs:
{{ range $_, $v := .cfg.Sans }}  - {{$v}} {{ end }}
controllerManager:
  extraArgs:
    enable-hostpath-provisioner: "true"
networking:
  podSubnet: "{{ .cfg.Network.PodCIDR }}"
---
apiVersion: kubeadm.k8s.io/v1beta1
kind: InitConfiguration
metadata:
  name: config
# we use a well know token for TLS bootstrap
bootstrapTokens:
- token: "{{ .cfg.Token }}"
# we use a well know port for making the API server discoverable inside runtime network. 
# from the host machine such port will be accessible via a random local port instead.
localAPIEndpoint:
  bindPort: 6443
nodeRegistration:
  criSocket: "/run/containerd/containerd.sock"
---
# no-op entry that exists solely so it can be patched
apiVersion: kubeadm.k8s.io/v1beta1
kind: JoinConfiguration
metadata:
  name: config
nodeRegistration:
  criSocket: "/run/containerd/containerd.sock"
---
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
metadata:
  name: config
# disable disk resource management by default
# kubelet will see the host disk that the inner container runtime
# is ultimately backed by and attempt to recover disk space. we don't want that.
imageGCHighThresholdPercent: 100
evictionHard:
  nodefs.available: "0%"
  nodefs.inodesFree: "0%"
  imagefs.available: "0%"
---
# no-op entry that exists solely so it can be patched
apiVersion: kubeproxy.config.k8s.io/v1alpha1
kind: KubeProxyConfiguration
metadata:
  name: config
`
