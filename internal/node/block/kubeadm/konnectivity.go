//go:build linux || darwin
// +build linux darwin

/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package kubeadminit implements the kubeadm init actionInit
package kubeadm

import (
	"encoding/base64"
	"fmt"
	"github.com/aoxn/meridian/internal/tool"
	"github.com/aoxn/meridian/internal/tool/sign"
	"github.com/pkg/errors"
	"os"
	"path"
)

var (
	kubernetesDir = "/etc/kubernetes/"
	manifestDir   = path.Join(kubernetesDir, "manifests")
)

func (a *actionInit) createKonnectivityPod() error {
	for _, v := range []string{konnectivityHost, manifestDir} {
		err := os.MkdirAll(v, 0755)
		if err != nil {
			return errors.Wrapf(err, "failed to create konnectivity host directory")
		}
	}
	egress := path.Join(konnectivityHost, egressFile)
	err := os.WriteFile(egress, []byte(egressSelector), 0755)
	if err != nil {
		return err
	}
	err = os.WriteFile(path.Join(manifestDir, "konnectivity-host.yaml"), []byte(konnectivityPod), 0755)
	if err != nil {
		return err
	}
	return a.createKonnectivityKubeconfig()
}

func (a *actionInit) createKonnectivityKubeconfig() error {
	var (
		addr = "127.0.0.1"
		port = "6443"
	)

	root := a.req.Spec.Config.TLS["root"]
	key, crt, err := sign.SignKonnectivity(root.Cert, root.Key, []string{})
	if err != nil {
		return fmt.Errorf("sign konnectivity client crt: %s", err.Error())
	}

	cfg, err := tool.RenderConfig(
		"konnectivity.kubeconfig",
		tool.KubeConfigTpl,
		tool.RenderParam{
			AuthCA:      base64.StdEncoding.EncodeToString(root.Cert),
			Address:     addr,
			Port:        port,
			ClusterName: "kubernetes",
			UserName:    "system:konnectivity-server",
			ClientCRT:   base64.StdEncoding.EncodeToString(crt),
			ClientKey:   base64.StdEncoding.EncodeToString(key),
		},
	)
	if err != nil {
		return fmt.Errorf("render konnectivity config error: %s", err.Error())
	}
	return os.WriteFile(path.Join(kubernetesDir, "konnectivity-server.conf"), []byte(cfg), 0755)
}

var konnectRole = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: system:konnectivity-server
  labels:
    kubernetes.io/cluster-service: "true"
    addonmanager.kubernetes.io/mode: Reconcile
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:auth-delegator
subjects:
  - apiGroup: rbac.authorization.k8s.io
    kind: User
    name: system:konnectivity-server
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: konnectivity-agent
  namespace: kube-system
  labels:
    kubernetes.io/cluster-service: "true"
    addonmanager.kubernetes.io/mode: Reconcile

`

var egressSelector = `
apiVersion: apiserver.k8s.io/v1beta1
kind: EgressSelectorConfiguration
egressSelections:
# Since we want to control the egress traffic to the cluster, we use the
# "cluster" as the name. Other supported values are "etcd", and "controlplane".
- name: cluster
  connection:
    # This controls the protocol between the API Server and the Konnectivity
    # server. Supported values are "GRPC" and "HTTPConnect". There is no
    # end user visible difference between the two modes. You need to set the
    # Konnectivity server to work in the same mode.
    proxyProtocol: GRPC
    transport:
      # This controls what transport the API Server uses to communicate with the
      # Konnectivity server. UDS is recommended if the Konnectivity server
      # locates on the same machine as the API Server. You need to configure the
      # Konnectivity server to listen on the same UDS socket.
      # The other supported transport is "tcp". You will need to set up TLS 
      # config to secure the TCP transport.
      uds:
        udsName: /etc/kubernetes/konnectivity-server/konnectivity-server.socket
- name: controlplane
  connection:
    proxyProtocol: Direct
`

var konnectivityPod = `
apiVersion: v1
kind: Pod
metadata:
  name: konnectivity-server
  namespace: kube-system
spec:
  priorityClassName: system-cluster-critical
  hostNetwork: true
  containers:
  - name: konnectivity-server-container
    image: registry.cn-hangzhou.aliyuncs.com/aoxn/proxy-server:v0.0.37
    command: ["/proxy-server"]
    args: [
            "--logtostderr=true",
            # This needs to be consistent with the value set in egressSelectorConfiguration.
            "--uds-name=/etc/kubernetes/konnectivity-server/konnectivity-server.socket",
            "--delete-existing-uds-file",
            # The following two lines assume the Konnectivity server is
            # deployed on the same machine as the apiserver, and the certs and
            # key of the API Server are at the specified location.
            "--cluster-cert=/etc/kubernetes/pki/apiserver.crt",
            "--cluster-key=/etc/kubernetes/pki/apiserver.key",
            # This needs to be consistent with the value set in egressSelectorConfiguration.
            "--mode=grpc",
            "--server-port=0",
            "--agent-port=8132",
            "--admin-port=8133",
            "--health-port=8134",
            "--agent-namespace=kube-system",
            "--agent-service-account=konnectivity-agent",
            "--kubeconfig=/etc/kubernetes/konnectivity-server.conf",
            "--authentication-audience=system:konnectivity-server"
            ]
    livenessProbe:
      httpGet:
        scheme: HTTP
        host: 127.0.0.1
        port: 8134
        path: /healthz
      initialDelaySeconds: 30
      timeoutSeconds: 60
    ports:
    - name: agentport
      containerPort: 8132
      hostPort: 8132
    - name: adminport
      containerPort: 8133
      hostPort: 8133
    - name: healthport
      containerPort: 8134
      hostPort: 8134
    volumeMounts:
    - name: k8s-certs
      mountPath: /etc/kubernetes/pki
      readOnly: true
    - name: kubeconfig
      mountPath: /etc/kubernetes/konnectivity-server.conf
      readOnly: true
    - name: konnectivity-uds
      mountPath: /etc/kubernetes/konnectivity-server
      readOnly: false
  volumes:
  - name: k8s-certs
    hostPath:
      path: /etc/kubernetes/pki
  - name: kubeconfig
    hostPath:
      path: /etc/kubernetes/konnectivity-server.conf
      type: FileOrCreate
  - name: konnectivity-uds
    hostPath:
      path: /etc/kubernetes/konnectivity-server
      type: DirectoryOrCreate

`
