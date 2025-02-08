//go:build linux || darwin
// +build linux darwin

package post

import (
	"bytes"
	"context"
	"fmt"
	"github.com/aoxn/meridian"
	api "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/node/block"
	"github.com/aoxn/meridian/internal/node/host"
	"github.com/aoxn/meridian/internal/tool"
	"github.com/aoxn/meridian/internal/tool/sign"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"html/template"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"strings"
	"time"
)

type postBlock struct {
	req  *api.Request
	host host.Host
}

// NewPostBlock returns a new postBlock for post kubernetes install
func NewPostBlock(req *api.Request, host host.Host) (block.Block, error) {
	return &postBlock{host: host, req: req}, nil
}

// Ensure runs the postBlock
func (a *postBlock) Ensure(ctx context.Context) error {

	klog.Infof("on waiting for kube-apiserver ok")
	err := waitBootstrap(a.req)
	if err != nil {
		return err
	}
	klog.Infof("on sign meridian operator tls")
	err = signAndCreate(a.req)
	if err != nil {
		return err
	}
	klog.Infof("on creating meridian operator")
	return doRunWdrip(a.req)
}

func waitBootstrap(req *api.Request) error {
	err := wait.Poll(
		3*time.Second,
		10*time.Minute,
		func() (done bool, err error) {

			_, err = tool.Kubectl("--kubeconfig", tool.AUTH_FILE,
				"-l", "node-role.kubernetes.io/control-plane", "get", "no",
			)
			if err != nil {
				klog.Infof("wait for bootstrap master: %s", err.Error())
				return false, nil
			}
			return true, nil
		},
	)
	if err != nil {
		return fmt.Errorf("wait bootstrap: %s", err.Error())
	}
	return err
	//return post.RunWdrip(ctx.BootCFG)
}

func (a *postBlock) Name() string {
	return fmt.Sprintf("meridian operator: [%s]", a.host.NodeID())
}

func (a *postBlock) Purge(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (a *postBlock) CleanUp(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func signAndCreate(req *api.Request) error {
	crt, ok := req.Spec.Config.TLS["root"]
	if !ok {
		return fmt.Errorf("empty root ca-key in request: %s", req.Name)
	}
	sans := []string{
		"meridian-operator",
		"meridian-operator.svc",
		"meridian-operator.default.svc",
		"meridian-operator.kube-system.svc",
		"meridian-webhook",
	}
	key, cbyte, err := sign.SignCertWithDNS(crt.Cert, crt.Key, sans)
	if err != nil {
		return err
	}
	cfg := api.Config{
		Kind:       "Config",
		APIVersion: "v1",
		AuthInfos: map[string]*api.AuthInfo{
			req.Spec.Provider.Type: &req.Spec.Provider,
		},
		CurrentContext: req.Spec.Provider.Type,
		Server: api.ServerConfig{
			WebhookCA:      crt.Cert,
			WebhookTLSCert: cbyte,
			WebhookTLSKey:  key,
		},
	}
	sec := v1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "meridian.cfg",
			Namespace: "kube-system",
		},
		Data: map[string][]byte{
			"meridian.cfg": []byte(tool.PrettyYaml(cfg)),
		},
	}
	return wait.Poll(
		2*time.Second,
		1*time.Minute,
		func() (done bool, err error) {
			err = tool.ApplyYaml(tool.PrettyYaml(sec), "meridian.cfg")
			if err != nil {
				klog.Errorf("retry wait for meridian operator: %s", err.Error())
				return false, nil
			}
			return true, nil
		},
	)
}

func doRunWdrip(ctx *api.Request) error {
	render := func(spec *api.Request) (string, error) {
		t, err := template.New("meridian_operator").Parse(meridianTpl)
		if err != nil {
			return "", errors.Wrap(err, "failed to parse config template")
		}
		reg := strings.Split(ctx.Spec.Config.Registry, "/")
		// execute the template
		var buff bytes.Buffer
		err = t.Execute(
			&buff,
			struct {
				Version  string
				Registry string
				UUID     string
			}{
				Version:  meridian.Version,
				Registry: fmt.Sprintf("%s/aoxn", reg[0]),
				UUID:     uuid.New().String(),
			},
		)
		return buff.String(), err
	}
	cfg, err := render(ctx)
	if err != nil {
		return err
	}
	return wait.Poll(
		2*time.Second,
		1*time.Minute,
		func() (done bool, err error) {

			err = tool.ApplyYaml(cfg, "meridian")
			if err != nil {
				klog.Errorf("retry wait for meridian operator: %s", err.Error())
				return false, nil
			}
			return true, nil
		},
	)
}

var meridianTpl = `
apiVersion: v1
kind: Service
metadata:
  name: knode-operator
  namespace: "kube-system"
  labels:
    app: knode-operator
spec:
  type: ClusterIP
  ipFamilyPolicy: SingleStack
  clusterIP: None
  ports:
    - port: 8443
      targetPort: 8443
      protocol: TCP
      name: tcp
  selector:
    control-plane: meridian-operator

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: meridian-operator
  namespace: kube-system
  labels:
    random.uuid: "{{ .UUID }}"
    control-plane: meridian-operator
    app.kubernetes.io/name: deployment
    app.kubernetes.io/instance: meridian-operator
    app.kubernetes.io/component: manager
    app.kubernetes.io/created-by: meridian
    app.kubernetes.io/part-of: meridian
spec:
  selector:
    matchLabels:
      control-plane: meridian-operator
  replicas: 1
  template:
    metadata:
      annotations:
        kubectl.kubernetes.io/default-container: manager
      labels:
        control-plane: meridian-operator
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                - key: kubernetes.io/arch
                  operator: In
                  values:
                    - amd64
                    - arm64
                    - ppc64le
                    - s390x
                - key: kubernetes.io/os
                  operator: In
                  values:
                    - linux
      securityContext:
        runAsNonRoot: true
        # TODO(user): For common cases that do not require escalating privileges
        # it is recommended to ensure that all your Pods/Containers are restrictive.
        # More info: https://kubernetes.io/docs/concepts/security/pod-security-standards/#restricted
        # Please uncomment the following code if your project does NOT have to work on old Kubernetes
        # versions < 1.19 or on vendors versions which do NOT support this field by default (i.e. Openshift < 4.11 ).
        # seccompProfile:
        #   type: RuntimeDefault
      containers:
      - command:
        - /manager
        - --kubeconfig=/etc/kubernetes/config/meridian-kubeconfig
        image: "{{ .Registry }}/meridian:{{ .Version }}"
        name: manager
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop:
              - "ALL"
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8081
          initialDelaySeconds: 15
          periodSeconds: 20
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8081
          initialDelaySeconds: 5
          periodSeconds: 10
        # TODO(user): Configure the resources accordingly based on the project requirements.
        # More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
        resources:
          limits:
            cpu: 1000m
            memory: 2Gi
          requests:
            cpu: 500m
            memory: 500Mi
        volumeMounts:
          - mountPath: /etc/kubernetes/config
            name: meridian-kubeconfig
            readOnly: true
          - mountPath: /etc/localtime
            name: localtime
            readOnly: true
          - name: meridian
            mountPath: /etc/meridian/
            readOnly: true
      hostNetwork: true
      terminationGracePeriodSeconds: 10
      nodeSelector:
        node-role.kubernetes.io/control-plane: ""
      tolerations:
        - effect: NoSchedule
          key: node.kubernetes.io/not-ready
          operator: Exists
        - effect: NoSchedule
          key: node.kubernetes.io/unreachable
          operator: Exists
        - effect: NoSchedule
          operator: Exists
          key: node-role.kubernetes.io/control-plane
        - effect: NoSchedule
          operator: Exists
          key: node-role.kubernetes.io/control-plane
        - effect: NoSchedule
          operator: Exists
          key: node.cloudprovider.kubernetes.io/uninitialized
      volumes:
        - hostPath:
            path: /usr/share/zoneinfo/Asia/Shanghai
            type: ""
          name: localtime
        - name: meridian-kubeconfig
          projected:
            defaultMode: 420
            sources:
              - secret:
                  items:
                    - key: kubeconfig
                      path: meridian-kubeconfig
                  name: meridian-kubeconfig
        - name: meridian
          projected:
            defaultMode: 420
            sources:
              - secret:
                  items:
                    - key: meridian.cfg
                      path: config 
                  name: meridian.cfg

`
