package tool

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
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

const NODE_MASTER_LABEL = "host-role.kubernetes.io/master"

func NodeIsMaster(node *corev1.Node) bool {
	labels := node.Labels
	if _, ok := labels[NODE_MASTER_LABEL]; ok {
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
