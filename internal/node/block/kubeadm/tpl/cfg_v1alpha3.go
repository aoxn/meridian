package tpl

// ConfigTemplateAlphaV3 is the kubadm config template for API version v1alpha3
const ConfigTemplateAlphaV3 = `
apiVersion: kubeadm.k8s.io/v1alpha3
kind: ClusterConfiguration
metadata:
  name: config
kubernetesVersion: v{{ .Kubernetes.Version }}
clusterName: "{{ .ClusterID }}"
{{ if .Endpoint.Intranet -}}
controlPlaneEndpoint: {{ .Endpoint.Intranet }}
{{- end }}
# we need nsswitch.conf so we use /etc/hosts
# https://github.com/kubernetes/kubernetes/issues/69195
apiServerExtraVolumes:
- name: nsswitch
  mountPath: /etc/nsswitch.conf
  hostPath: /etc/nsswitch.conf
  writeable: false
  pathType: FileOrCreate
- hostPath: /etc/localtime
  mountPath: /etc/localtime
  name: localtime
controllerManagerExtraVolumes:
- hostPath: /etc/localtime
  mountPath: /etc/localtime
  name: localtime
schedulerExtraVolumes:
- hostPath: /etc/localtime
  mountPath: /etc/localtime
  name: localtime
# on runtime for mac we have to expose the api server via port forward,
# so we need to ensure the cert is valid for localhost so we can talk
# to the cluster after rewriting the kubeconfig to point to localhost
apiServerCertSANs: 
- 127.0.0.1
{{ range $_, $v := .Sans }}
- {{$v}} 
{{ end }}
controllerManagerExtraArgs:
  enable-hostpath-provisioner: "true"
  cloud-provider: external
  horizontal-pod-autoscaler-use-service-clients: "true"
  host-cidr-mask-size: "{{ .Network.NetMask }}"
apiServerExtraArgs:
  cloud-provider: external
  feature-gates: VolumeSnapshotDataSource=true,CSINodeInfo=true,CSIDriverRegistry=true
networking:
  podSubnet: "{{ .Network.PodCIDR }}"
  dnsDomain: {{ .Network.Domain }}
  serviceSubnet: {{ .Network.SVCCIDR }}
etcd:
  external:
    caFile: /var/lib/etcd/cert/server-ca.crt
    certFile: /var/lib/etcd/cert/client.crt
    keyFile: /var/lib/etcd/cert/client.key
    endpoints:
{{ range $_, $v := .EtcdEndpoints }}
    - {{$v}} 
{{ end }}
imageRepository: {{ .Registry }}
---
apiVersion: kubeadm.k8s.io/v1alpha3
kind: InitConfiguration
metadata:
  name: config
# we use a well know token for TLS bootstrap
bootstrapTokens:
- token: "{{ .Kubernetes.KubeadmToken }}"
  ttl: 0s
# we use a well know port for making the API server discoverable inside runtime network. 
# from the host machine such port will be accessible via a random local port instead.
apiEndpoint:
  bindPort: 6443
nodeRegistration:
  kubeletExtraArgs:
    cloud-provider: external
  name: {{ .NodeName }}
  criSocket: "/run/containerd/containerd.sock"
---
# no-op entry that exists solely so it can be patched
apiVersion: kubeadm.k8s.io/v1alpha3
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
proxyMode: {{ .Network.Mode }}
`
