package v1

import (
	"encoding/json"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"os"
)

var (
	G = Global{
		Debug: true,
	}
)

type Global struct {
	WebhookCA []byte
	Debug     bool
	Resource  string
	OutPut    string
	Cache     bool
	Config    Config
}

func LoadConfig() error {
	cff := "/etc/meridian/config"
	klog.Infof("use config file: %s", cff)
	data, err := os.ReadFile(cff)
	if err != nil {
		return errors.Wrapf(err, "read config file")
	}
	return yaml.Unmarshal(data, &G.Config)
}

// ProviderSpec defines the desired state of Provider
type ProviderSpec struct {
	AuthInfo `json:"authInfo,omitempty"`
	Extra    json.RawMessage `json:"extra,omitempty"`
}

// ProviderStatus defines the observed state of Provider
type ProviderStatus struct {
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Provider is the Schema for the providers API
type Provider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProviderSpec   `json:"spec,omitempty"`
	Status ProviderStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ProviderList contains a list of Provider
type ProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Provider `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Provider{}, &ProviderList{})
}

func (in *Provider) Decode(i interface{}) error { return json.Unmarshal(in.Spec.Extra, i) }

func ToRawMessage(i interface{}) (json.RawMessage, error) {
	data, err := json.Marshal(i)
	if err != nil {
		return nil, errors.Wrap(err, "marshal to raw message")
	}
	raw := json.RawMessage{}
	err = json.Unmarshal(data, &raw)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshal to raw message")
	}
	return raw, nil
}

type Config struct {
	// Legacy field from pkg/api/types.go TypeMeta.
	// TODO(jlowdermilk): remove this after eliminating downstream dependencies.
	// +k8s:conversion-gen=false
	// +optional
	Kind string `json:"kind,omitempty"`
	// Legacy field from pkg/api/types.go TypeMeta.
	// TODO(jlowdermilk): remove this after eliminating downstream dependencies.
	// +k8s:conversion-gen=false
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`

	// AuthInfos is a map of referencable names to user configs
	AuthInfos map[string]*AuthInfo `json:"providers"`

	// CurrentContext is the name of the context that you would like to use by default
	CurrentContext string `json:"current-context"`

	Server ServerConfig `json:"server,omitempty"`
}

type ServerConfig struct {
	WebhookCA []byte `json:"webhookCA,omitempty"`

	WebhookTLSCert []byte `json:"webhookTLSCert,omitempty"`

	WebhookTLSKey []byte `json:"webhookTLSKey,omitempty"`
}

func (d *Config) GetCurrent() *AuthInfo {
	return d.AuthInfos[d.CurrentContext]
}

// AuthInfo contains information that describes identity information.  This is used to tell the kubernetes cluster who you are.
type AuthInfo struct {

	// Type the provider name of cloud
	Type string `json:"type,omitempty"`
	// Region is metadata region
	// +optional
	Region string `json:"region,omitempty"`
	// AccessKey is the key for provider
	// +optional
	AccessKey string `json:"access-key,omitempty"`

	//AccessSecret is the secret for provider
	// +optional
	AccessSecret string `json:"access-secret,omitempty"`

	// ClientCertificate is the path to a client cert file for TLS.
	// +optional
	ClientCertificate string `json:"client-certificate,omitempty"`
	// ClientCertificateData contains PEM-encoded data from a client cert file for TLS. Overrides ClientCertificate
	// +optional
	ClientCertificateData []byte `json:"client-certificate-data,omitempty"`
	// ClientKey is the path to a client key file for TLS.
	// +optional
	ClientKey string `json:"client-key,omitempty"`
	// ClientKeyData contains PEM-encoded data from a client key file for TLS. Overrides ClientKey
	// +optional
	ClientKeyData []byte `json:"client-key-data,omitempty" datapolicy:"security-key"`
	// Token is the bearer token for authentication to the kubernetes cluster.
	// +optional
	Token string `json:"token,omitempty" datapolicy:"token"`
	// TokenFile is a pointer to a file that contains a bearer token (as described above).  If both Token and TokenFile are present, Token takes precedence.
	// +optional
	TokenFile string `json:"tokenFile,omitempty"`
	// Impersonate is the username to act-as.
	// +optional
	Impersonate string `json:"act-as,omitempty"`
	// ImpersonateUID is the uid to impersonate.
	// +optional
	ImpersonateUID string `json:"act-as-uid,omitempty"`
	// ImpersonateGroups is the groups to impersonate.
	// +optional
	ImpersonateGroups []string `json:"act-as-groups,omitempty"`
	// ImpersonateUserExtra contains additional information for impersonated user.
	// +optional
	ImpersonateUserExtra map[string][]string `json:"act-as-user-extra,omitempty"`
	// Username is the username for basic authentication to the kubernetes cluster.
	// +optional
	Username string `json:"username,omitempty"`
	// Password is the password for basic authentication to the kubernetes cluster.
	// +optional
	Password string `json:"password,omitempty" datapolicy:"password"`
}
