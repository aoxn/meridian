package tool

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/versioning"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/clientcmd/api"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"math/rand"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	clientgov1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/klog/v2"
	"math/big"
	"net"
	"os"
	"text/template"

	tj "k8s.io/apimachinery/pkg/runtime/serializer/json"
)

var (
	CA          = "ca"
	ETCD_PEER   = "etcd.peer"
	ETCD_CLIENT = "etcd.client"
)

const (
	NODE_MASTER_LABEL   = "host-role.kubernetes.io/master"
	NODE_MASTER_LABEL_1 = "node-role.kubernetes.io/control-plane"
	NODE_PROVIDER_LABEL = "xdpin.cn/provider"
)

func GetProviderName(node *corev1.Node) string {
	labels := node.Labels
	if labels == nil {
		return ""
	}
	if v, ok := labels[NODE_PROVIDER_LABEL]; ok {
		return v
	}
	return ""
}

const (
	WorkingNamespace               = "kube-system"
	RavenGlobalConfig              = "raven-cfg"
	RavenAgentConfig               = "raven-agent-config"
	LabelCurrentGatewayEndpoints   = "raven.openyurt.io/endpoints-name"
	GatewayProxyInternalService    = "x-raven-proxy-internal-svc"
	GatewayProxyServiceNamePrefix  = "x-raven-proxy-svc"
	GatewayTunnelServiceNamePrefix = "x-raven-tunnel-svc"
	ExtraAllowedSourceCIDRs        = "raven.openyurt.io/extra-allowed-source-cidrs"

	RavenProxyNodesConfig      = "edge-tunnel-nodes"
	ProxyNodesKey              = "tunnel-nodes"
	ProxyServerSecurePortKey   = "proxy-internal-secure-addr"
	ProxyServerInsecurePortKey = "proxy-internal-insecure-addr"
	ProxyServerExposedPortKey  = "proxy-external-addr"
	VPNServerExposedPortKey    = "tunnel-bind-addr"
	RavenEnableProxy           = "enable-l7-proxy"
	RavenEnableTunnel          = "enable-l3-tunnel"
)

// GetNodeInternalIP returns internal ip of the given `node`.
func GetNodeInternalIP(node corev1.Node) string {
	var ip string
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP && net.ParseIP(addr.Address) != nil {
			ip = addr.Address
			break
		}
	}
	return ip
}

func GetNodeGroupID(node *corev1.Node) string {
	if node.Labels == nil {
		return ""
	}
	ng, ok := node.Labels[v1.MERIDIAN_NODEGROUP]
	return lo.Ternary(ok, ng, "")
}

// IsNodeReady checks if the `node` is `corev1.NodeReady`
func IsNodeReady(node corev1.Node) bool {
	_, nc := GetNodeCondition(&node.Status, corev1.NodeReady)
	// GetNodeCondition will return nil and -1 if the condition is not present
	return nc != nil && nc.Status == corev1.ConditionTrue
}

// GetNodeCondition extracts the provided condition from the given status and returns that.
// Returns nil and -1 if the condition is not present, and the index of the located condition.
func GetNodeCondition(status *corev1.NodeStatus, conditionType corev1.NodeConditionType) (int, *corev1.NodeCondition) {
	if status == nil {
		return -1, nil
	}
	for i := range status.Conditions {
		if status.Conditions[i].Type == conditionType {
			return i, &status.Conditions[i]
		}
	}
	return -1, nil
}

func HashObject(o interface{}) string {
	data, _ := json.Marshal(o)
	var a interface{}
	err := json.Unmarshal(data, &a)
	if err != nil {
		klog.Errorf("unmarshal: %s", err.Error())
	}
	return computeHash(PrettyYaml(a))
}

func computeHash(target string) string {
	hash := sha256.Sum224([]byte(target))
	return strings.ToLower(hex.EncodeToString(hash[:]))
}

type Errors []error

func (e Errors) Error() string {
	result := ""
	for _, err := range e {
		if err == nil {
			klog.Errorf("nil error")
			continue
		}
		result += err.Error() + "\n"
	}
	return result
}

func (e Errors) HasError() error {
	if len(e) != 0 {
		return e
	}
	return nil
}

func PrettyYaml(obj interface{}) string {
	bs, err := yaml.Marshal(obj)
	if err != nil {
		klog.Errorf("failed to parse yaml, %s", err.Error())
	}
	return string(bs)
}

func PrettyJson(obj interface{}) string {
	pretty := bytes.Buffer{}
	data, err := json.Marshal(obj)
	if err != nil {
		fmt.Printf("PrettyJson, mashal error: %s", err.Error())
		return ""
	}
	err = json.Indent(&pretty, data, "", "    ")

	if err != nil {
		fmt.Printf("PrettyJson, indent error: %s", err.Error())
		return ""
	}
	return pretty.String()
}

func UnknownCondition(condition []corev1.NodeCondition) bool {
	for _, c := range condition {
		if c.Type == corev1.NodeReady {
			return c.Status == corev1.ConditionUnknown || c.Status == corev1.ConditionFalse
		}
	}
	return false
}

func NodeArray(ns map[string]*corev1.Node) []string {
	var nodeArray []string
	for _, v := range ns {
		nodeArray = append(nodeArray, v.Name)
	}
	return nodeArray
}

func FileExist(file string) (bool, error) {
	_, err := os.Stat(file)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func GetDNSIP(subnet string, index int) (net.IP, error) {
	_, cidr, err := net.ParseCIDR(subnet)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse service subnet CIDR %q: %v", subnet, err)
	}

	bip := big.NewInt(0).SetBytes(cidr.IP.To4())
	ip := net.IP(big.NewInt(0).Add(bip, big.NewInt(int64(index))).Bytes())
	if cidr.Contains(ip) {
		return ip, nil
	}
	return nil, fmt.Errorf("can't generate IP with "+
		"index %d from subnet. subnet too small. subnet: %q", index, subnet)
}

func Write(config clientcmdapi.Config) ([]byte, error) {
	Scheme := runtime.NewScheme()
	utilruntime.Must(api.AddToScheme(Scheme))
	utilruntime.Must(clientgov1.AddToScheme(Scheme))
	yamlSerializer := tj.NewSerializerWithOptions(
		tj.DefaultMetaFactory, Scheme, Scheme, tj.SerializerOptions{Yaml: false, Pretty: true, Strict: false},
	)
	codec := versioning.NewDefaultingCodecForScheme(
		Scheme,
		yamlSerializer,
		yamlSerializer,
		schema.GroupVersion{Version: "v1"},
		runtime.InternalGroupVersioner,
	)
	return runtime.Encode(codec, &config)
}

func RenderConfig(
	tplName string,
	tpl string,
	data interface{},
) (string, error) {
	t, err := template.New(tplName).Parse(tpl)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse config template")
	}

	// execute the template
	var buff bytes.Buffer
	err = t.Execute(&buff, data)
	return buff.String(), err
}

func NewConfig(
	cid, ip string,
	ca, crt, key []byte,
) clientcmdapi.Config {
	user := "kubernetes-admin"
	return clientcmdapi.Config{
		Kind:       "Config",
		APIVersion: "v1",
		Clusters: map[string]*clientcmdapi.Cluster{
			cid: {
				CertificateAuthorityData: ca,
				Server:                   fmt.Sprintf("https://%s:6443", ip),
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			fmt.Sprintf("%s@%s", user, cid): {
				Cluster:  cid,
				AuthInfo: user,
			},
		},
		CurrentContext: fmt.Sprintf("%s@%s", user, cid),
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			user: {
				ClientKeyData:         key,
				ClientCertificateData: crt,
			},
		},
	}
}

type RenderParam struct {
	AuthCA      string
	Address     string
	Port        string
	ClientCRT   string
	ClientKey   string
	ClusterName string
	UserName    string
}

var KubeConfigTpl = `
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: {{ .AuthCA }}
    server: https://{{ .Address }}:{{.Port}}
  name: {{ .ClusterName }}
contexts:
- context:
    cluster: {{ .ClusterName }}
    user: {{ .UserName }}
  name: {{.UserName}}@{{.ClusterName}}
current-context: {{.UserName}}@{{.ClusterName}}
kind: Config
preferences: {}
users:
- name: {{ .UserName }}
  user:
    client-certificate-data: {{ .ClientCRT }}
    client-key-data: {{ .ClientKey }}
`

func NodeIsMaster(node *corev1.Node) bool {
	labels := node.Labels
	if labels == nil {
		return false
	}
	if _, ok := labels[NODE_MASTER_LABEL]; ok {
		return true
	}
	if _, ok := labels[NODE_MASTER_LABEL_1]; ok {
		return true
	}
	return false
}

func RandomID(strlen int) string {
	const asciiA = 65
	const asciiZ = 90
	rand.Seed(time.Now().UTC().UnixNano())
	result := make([]byte, strlen)
	for i := 0; i < strlen; i++ {
		result[i] = byte(randInt(asciiA, asciiZ))
	}
	return strings.ToLower(string(result))
}

func randInt(min int, max int) int {
	return min + rand.Intn(max-min)
}

func AddHostResolve(doman, ip string) error {
	pathp := "/etc/hosts"
	content, err := os.Open(pathp)
	if err != nil {
		return err
	}
	defer func() {
		err = content.Close()
		if err != nil {
			klog.Errorf("cannot close /etc/host file: %v", err)
		}
	}()
	var lines []string
	scanner := bufio.NewScanner(content)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, doman) {
			lines = append(lines, line)
		}
	}
	lines = append(lines, fmt.Sprintf("%s\t%s\n", ip, doman))
	return os.WriteFile(pathp, []byte(strings.Join(lines, "\n")), 0644)
}
