package node

import (
	"context"
	"crypto"
	"crypto/sha512"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"fmt"
	"github.com/aoxn/meridian/internal/tool"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"

	"k8s.io/klog/v2"

	certificatesv1 "k8s.io/api/certificates/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/certificate/csr"
	"k8s.io/client-go/util/keyutil"
)

const tmpPrivateKeyFile = "meridian-client.key.tmp"

var Usages = []certificatesv1.KeyUsage{
	certificatesv1.UsageDigitalSignature,
	certificatesv1.UsageKeyEncipherment,
	certificatesv1.UsageClientAuth,
}

func newBootstrapCfg(apiserver, token string) clientcmdapi.Config {
	var (
		user        = "bootstrap"
		cluster     = "kubernetes"
		contextName = fmt.Sprintf("%s@%s", user, cluster)
	)

	return clientcmdapi.Config{
		Kind:       "Spec",
		APIVersion: "v1",
		Clusters: map[string]*clientcmdapi.Cluster{
			cluster: {
				Server:                apiserver,
				InsecureSkipTLSVerify: true,
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			contextName: {
				Cluster:  cluster,
				AuthInfo: user,
			},
		},
		AuthInfos:      map[string]*clientcmdapi.AuthInfo{user: {Token: token}},
		CurrentContext: contextName,
	}
}

func GetCA(client clientset.Interface) ([]byte, error) {

	public, err := client.CoreV1().
		ConfigMaps("kube-public").
		Get(context.TODO(), "cluster-info", metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "getting cluster-info configmap")
	}

	auth, err := clientcmd.Load([]byte(public.Data["kubeconfig"]))
	if err != nil {
		return nil, errors.Wrapf(err, "loading cluster-info kubeconfig")
	}
	return auth.Clusters[""].CertificateAuthorityData, nil
}

func newClientCfg(cfg clientcmdapi.Config) (*restclient.Config, error) {
	data, err := clientcmd.Write(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "write bootstrapCfg data")
	}
	bcfg, err := clientcmd.NewClientConfigFromBytes(data)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load admin kubeconfig")
	}
	return bcfg.ClientConfig()
}

func newKubeconfig(apiserver string, caData, keyData, certData []byte, insecure bool) clientcmdapi.Config {
	// Build resulting kubeconfig.
	kubeconfigData := clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{"default-cluster": {
			Server:                   fmt.Sprintf("https://%s", apiserver),
			InsecureSkipTLSVerify:    insecure,
			CertificateAuthorityData: caData,
		}},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{"default-auth": {
			ClientKeyData:         keyData,
			ClientCertificateData: certData,
		}},
		Contexts: map[string]*clientcmdapi.Context{"default-context": {
			Cluster:   "default-cluster",
			AuthInfo:  "default-auth",
			Namespace: "default",
		}},
		CurrentContext: "default-context",
	}
	return kubeconfigData
}

func GetCSR(
	apiserver, token string,
	nodeName types.NodeName,
) (*clientcmdapi.Config, error) {
	var (
		ctx          = context.Background()
		kcfgPath     = KubeconfigTmp
		bootstrapCfg = newBootstrapCfg(withSchema(apiserver), token)
	)
	klog.V(5).Infof("bootstrap cfg: %s", tool.PrettyYaml(bootstrapCfg))
	clientCfg, err := newClientCfg(bootstrapCfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get client config")
	}
	bootstrapClient, err := clientset.NewForConfig(clientCfg)
	if err != nil {
		return nil, fmt.Errorf("unable to create certificates signing request client: %v", err)
	}

	keyData, err := keyutil.MakeEllipticPrivateKeyPEM()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to make private key")
	}

	err = waitForServer(ctx, *clientCfg, 1*time.Minute)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to wait for server")
	}

	cfg := Conf{
		Subject: &pkix.Name{
			Organization: []string{"system:meridian"},
			CommonName:   "system:meridian:" + string(nodeName),
		},
		CSRName:    "meridian-csr",
		Usage:      Usages,
		NodeName:   nodeName,
		PrivateKey: keyData,
		SingerName: certificatesv1.KubeAPIServerClientSignerName,
	}
	certData, err := SendCSR(ctx, bootstrapClient, &cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to send CSR")
	}

	kubeconfigData := newKubeconfig(apiserver, nil, keyData, certData, true)
	return &kubeconfigData, clientcmd.WriteToFile(kubeconfigData, kcfgPath)
}

const (
	CSR_OPERATOR = "xdpin-operator"
)

// RequestFastCSR used for test, generate xdpin-operator kubeconfig
func RequestFastCSR(
	ctx context.Context,
	bootstrapClientConfig *restclient.Config,
	nodeName types.NodeName, cn string,
) (*clientcmdapi.Config, error) {
	bootstrapClient, err := clientset.NewForConfig(bootstrapClientConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to create certificates signing request client: %v", err)
	}

	keyData, err := keyutil.MakeEllipticPrivateKeyPEM()
	if err != nil {
		return nil, fmt.Errorf("error generating key: %v", err)
	}

	cfg := Conf{
		Subject: &pkix.Name{
			Organization: []string{"system:meridian"},
		},
		CSRName:    "meridian-csr",
		Usage:      Usages,
		NodeName:   nodeName,
		PrivateKey: keyData,
		SingerName: certificatesv1.KubeAPIServerClientSignerName,
	}
	switch cn {
	case CSR_OPERATOR:
		cfg.Subject.CommonName = "xdpin-operator"
	default:
		cfg.Subject.CommonName = "system:meridian:" + string(nodeName)
	}
	certData, err := SendCSR(ctx, bootstrapClient, &cfg)
	if err != nil {
		return nil, err
	}

	// Build resulting kubeconfig.
	kubeconfigData := clientcmdapi.Config{
		AuthInfos: map[string]*clientcmdapi.AuthInfo{"default-auth": {
			ClientCertificateData: certData,
			ClientKeyData:         keyData,
		}},
		CurrentContext: "default-context",
		// Define a context that connects the auth info and cluster, and set it as the default
		Contexts: map[string]*clientcmdapi.Context{"default-context": {
			Namespace: "default",
			Cluster:   "default-cluster",
			AuthInfo:  "default-auth",
		}},
		// Define a cluster stanza based on the bootstrap kubeconfig.
		Clusters: map[string]*clientcmdapi.Cluster{"default-cluster": {
			Server:                   bootstrapClientConfig.Host,
			InsecureSkipTLSVerify:    bootstrapClientConfig.Insecure,
			CertificateAuthorityData: bootstrapClientConfig.CAData,
		}},
	}
	return &kubeconfigData, nil
}

// RequestWriteCSR used for test, generate xdpin-operator kubeconfig
func RequestWriteCSR(
	ctx context.Context,
	bootstrapClientConfig *restclient.Config,
	dstKubeconfig string, nodeName types.NodeName, cn string,
) (*clientcmdapi.Config, error) {

	kubeconfigData, err := RequestFastCSR(ctx, bootstrapClientConfig, nodeName, cn)
	if err != nil {
		return nil, err
	}
	// Marshal to disk
	return kubeconfigData, clientcmd.WriteToFile(*kubeconfigData, dstKubeconfig)
}

func waitForServer(ctx context.Context, cfg restclient.Config, deadline time.Duration) error {
	cfg.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	cfg.Timeout = 1 * time.Second
	cli, err := restclient.UnversionedRESTClientFor(&cfg)
	if err != nil {
		return fmt.Errorf("couldn't create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()

	var connected bool
	wait.JitterUntil(func() {
		if _, err := cli.Get().AbsPath("/healthz").Do(ctx).Raw(); err != nil {
			klog.InfoS("Failed to connect to apiserver", "err", err)
			return
		}
		cancel()
		connected = true
	}, 2*time.Second, 0.2, true, ctx.Done())

	if !connected {
		return errors.New("timed out waiting to connect to apiserver")
	}
	return nil
}

type Conf struct {
	PrivateKey []byte
	CSRName    string
	NodeName   types.NodeName
	SingerName string
	Subject    *pkix.Name
	Usage      []certificatesv1.KeyUsage
}

func SendCSR(ctx context.Context, client clientset.Interface, cfg *Conf) (certData []byte, err error) {

	privateKey, err := keyutil.ParsePrivateKeyPEM(cfg.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("invalid private key for certificate request: %v", err)
	}
	csrData, err := certutil.MakeCSR(privateKey, cfg.Subject, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to generate certificate request: %v", err)
	}

	// The Signer interface contains the Public() method to get the public key.
	signer, ok := privateKey.(crypto.Signer)
	if !ok {
		return nil, fmt.Errorf("private key does not implement crypto.Signer")
	}

	name, err := digestedName(signer.Public(), cfg)
	if err != nil {
		return nil, err
	}

	reqName, reqUID, err := csr.RequestCertificate(client, csrData, name, cfg.SingerName, nil, Usages, privateKey)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 3600*time.Second)
	defer cancel()

	klog.V(2).InfoS("Waiting for client certificate to be issued")
	return csr.WaitForCertificate(ctx, client, reqName, reqUID)
}

func digestedName(publicKey interface{}, cfg *Conf) (string, error) {
	hash := sha512.New512_256()
	const delimiter = '|'
	encode := base64.RawURLEncoding.EncodeToString

	write := func(data []byte) {
		hash.Write([]byte(encode(data)))
		hash.Write([]byte{delimiter})
	}

	publicKeyData, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", err
	}
	write(publicKeyData)

	write([]byte(cfg.Subject.CommonName))
	for _, v := range cfg.Subject.Organization {
		write([]byte(v))
	}
	for _, v := range cfg.Usage {
		write([]byte(v))
	}

	return fmt.Sprintf("%s-%s", cfg.CSRName, encode(hash.Sum(nil))), nil
}
