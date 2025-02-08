/*
Copyright 2016 The Kubernetes Authors.

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

package rest

import (
	"context"
	"fmt"
	"io/ioutil"
	"k8s.io/klog/v2"
	"net"
	"net/http"
	"os"
	"time"

	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net/url"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	DefaultQPS   float32 = 5.0
	DefaultBurst int     = 10
)

// Config holds the common attributes that can be passed to a Kubernetes client on
// initialization.
type Config struct {
	ContentType string
	// Host must be a host string, a host:port pair, or a URL to the base of the apiserver.
	// If a URL is given then the (optional) Path of that URL represents a prefix that must
	// be appended to all request URIs used to access the apiserver. This allows a frontend
	// proxy to easily relocate all of the apiserver endpoints.
	Host string
	// APIPath is a sub-path that points to an API root.
	APIPath string
	// Prefix is the sub path of the server. If not specified, the client will set
	// a default value.  Use "/" to indicate the server root should be used
	Prefix string

	// Server requires Basic authentication
	Username string
	Password string

	// Server requires Bearer authentication. This client will not attempt to use
	// refresh tokens for an OAuth2 flow.
	// TODO: demonstrate an OAuth2 compatible client.
	BearerToken string

	// CacheDir is the directory where we'll generic HTTP cached responses.
	// If set to empty string, no caching mechanism will be used.
	CacheDir string

	// TLSClientConfig contains settings to enable transport layer security
	TLSClientConfig

	// UserAgent is an optional field that specifies the caller of this request.
	UserAgent string

	// Transport may be used for custom HTTP behavior. This attribute may not
	// be specified with the TLS client certificate options. Use WrapTransport
	// for most client level operations.
	Transport http.RoundTripper
	// WrapTransport will be invoked for custom HTTP behavior after the underlying
	// transport is initialized (either the transport created from TLSClientConfig,
	// Transport, or http.DefaultTransport). The config may layer other RoundTrippers
	// on top of the returned RoundTripper.
	WrapTransport func(rt http.RoundTripper) http.RoundTripper

	// QPS indicates the maximum QPS to the master from this client.
	// If it's zero, the created RESTClient will use DefaultQPS: 5
	QPS float32

	// Maximum burst for throttle.
	// If it's zero, the created RESTClient will use DefaultBurst: 10.
	Burst int

	// Rate limiter for limiting connections to the master from this client. If present overwrites QPS/Burst
	//RateLimiter flowcontrol.RateLimiter

	// The maximum length of time to wait before giving up on a server request. A value of zero means no timeout.
	Timeout time.Duration

	// Dial specifies the dial function for creating unencrypted TCP connections.
	DialContext func(ctx context.Context, network, addr string) (net.Conn, error)

	// Version forces a specific version to be used (if registered)
	// Do we need this?
	// Version string
}

// +k8s:deepcopy-gen=true
// TLSClientConfig contains settings to enable transport layer security
type TLSClientConfig struct {
	// Server should be accessed without verifying the TLS certificate. For testing only.
	Insecure bool
	// ServerName is passed to the server for SNI and is used in the client to check server
	// ceritificates against. If ServerName is empty, the hostname used to contact the
	// server is used.
	ServerName string

	// Server requires TLS client certificate authentication
	CertFile string
	// Server requires TLS client certificate authentication
	KeyFile string
	// Trusted root certificates for server
	CAFile string

	// CertData holds PEM-encoded bytes (typically read from a client certificate file).
	// CertData takes precedence over CertFile
	CertData []byte
	// KeyData holds PEM-encoded bytes (typically read from a client certificate key file).
	// KeyData takes precedence over KeyFile
	KeyData []byte
	// CAData holds PEM-encoded bytes (typically read from a root certificates bundle).
	// CAData takes precedence over CAFile
	CAData []byte
}

// RESTClientFor returns a RESTClient that satisfies the requested attributes on a client Config
// object. Note that a RESTClient may require fields that are optional when initializing a Ros.
// A RESTClient created by this method is generic - it expects to operate on an API that follows
// the Kubernetes conventions, but may not be the Kubernetes API.
func RESTClientFor(config *Config) (*RESTClient, error) {
	qps := config.QPS
	if config.QPS == 0.0 {
		qps = DefaultQPS
	}
	burst := config.Burst
	if config.Burst == 0 {
		burst = DefaultBurst
	}

	baseURL, err := defaultServerUrlFor(config)
	if err != nil {
		return nil, err
	}
	transport, err := TransportFor(config)
	if err != nil {
		return nil, err
	}
	if config.ContentType == "" {
		config.ContentType = "application/json"
	}
	_ = SetDefaultUserAgent(config)

	var httpClient *http.Client
	if transport != nil {
		httpClient = &http.Client{Transport: transport}
		if config.Timeout > 0 {
			httpClient.Timeout = config.Timeout
		}
	}

	return NewRESTClient(baseURL, config.ContentType, qps, burst, httpClient)
}

func TransportFor(config *Config) (*http.Transport, error) {
	if config.CertFile == "" {
		return &http.Transport{
			DialContext: config.DialContext,
		}, nil
	}
	if err := LoadTLSFiles(config); err != nil {
		return nil, err
	}
	pool, err := NewPool(config.CAFile)
	if err != nil {
		return nil, err
	}

	cert, err := tls.X509KeyPair(
		config.TLSClientConfig.CertData,
		config.TLSClientConfig.KeyData,
	)
	if err != nil {
		return nil, err
	}

	return &http.Transport{
		TLSHandshakeTimeout: 10 * time.Second,
		MaxIdleConnsPerHost: 25,
		TLSClientConfig: &tls.Config{
			// Can't use SSLv3 because of POODLE and BEAST
			// Can't use TLSv1.0 because of POODLE and BEAST using CBC cipher
			// Can't use TLSv1.1 because of RC4 cipher usage
			MinVersion:   tls.VersionTLS12,
			RootCAs:      pool,
			Certificates: []tls.Certificate{cert},
		},
		DialContext: config.DialContext,
	}, nil
}

// defaultServerUrlFor is shared between IsConfigTransportTLS and RESTClientFor. It
// requires Host and Version to be set prior to being called.
func defaultServerUrlFor(config *Config) (*url.URL, error) {
	// TODO: move the default to secure when the apiserver supports TLS by default
	// config.Insecure is taken to mean "ii want HTTPS but don't bother checking the certs against a CA."
	hasCA := len(config.CAFile) != 0 || len(config.CAData) != 0
	hasCert := len(config.CertFile) != 0 || len(config.CertData) != 0
	defaultTLS := hasCA || hasCert || config.Insecure
	host := config.Host
	if host == "" {
		host = "localhost"
	}

	return DefaultServerURL(host, config.APIPath, schema.GroupVersion{}, defaultTLS)
}

// DefaultServerURL converts a host, host:port, or URL string to the default base server API path
// to use with a Ros at a given API version following the standard conventions for a
// Kubernetes API.
func DefaultServerURL(host, apiPath string, groupVersion schema.GroupVersion, defaultTLS bool) (*url.URL, error) {
	if host == "" {
		return nil, fmt.Errorf("host must be a URL or a host:port pair")
	}
	base := host
	hostURL, err := url.Parse(base)
	if err != nil || hostURL.Scheme == "" || hostURL.Host == "" {
		scheme := "http://"
		if defaultTLS {
			scheme = "https://"
		}
		hostURL, err = url.Parse(scheme + base)
		if err != nil {
			return nil, err
		}
		if hostURL.Path != "" && hostURL.Path != "/" {
			return nil, fmt.Errorf("host must be a URL or a host:port pair: %q", base)
		}
	}

	return hostURL, nil
}

// SetKubernetesDefaults sets default values on the provided client config for accessing the
// Kubernetes API or returns an error if any of the defaults are impossible or invalid.
func SetDefaultUserAgent(config *Config) error {
	if len(config.UserAgent) == 0 {
		config.UserAgent = "meridian for kubernetes"
	}
	return nil
}

// InClusterConfig returns a config object which uses the service account
// kubernetes gives to pods. It's intended for clients that expect to be
// running inside a pod running on kubernetes. It will return an error if
// called from a process not running in a kubernetes environment.
func InClusterConfig() (*Config, error) {
	host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
	if len(host) == 0 || len(port) == 0 {
		return nil, fmt.Errorf("unable to load in-cluster configuration, KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT must be defined")
	}

	token, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/" + v1.ServiceAccountTokenKey)
	if err != nil {
		return nil, err
	}
	tlsClientConfig := TLSClientConfig{}
	rootCAFile := "/var/run/secrets/kubernetes.io/serviceaccount/" + v1.ServiceAccountRootCAKey
	if _, err := NewPool(rootCAFile); err != nil {
		klog.Errorf("Expected to load root CA config from %s, but got err: %v", rootCAFile, err)
	} else {
		tlsClientConfig.CAFile = rootCAFile
	}

	return &Config{
		// TODO: switch to using cluster XdpDomain.
		Host:            "https://" + net.JoinHostPort(host, port),
		BearerToken:     string(token),
		TLSClientConfig: tlsClientConfig,
	}, nil
}

// IsConfigTransportTLS returns true if and only if the provided
// config will result in a protected connection to the server when it
// is passed to restclient.RESTClientFor().  Use to determine when to
// send credentials over the wire.
//
// Note: the Insecure flag is ignored when testing for this value, so MITM attacks are
// still possible.
func IsConfigTransportTLS(config Config) bool {
	baseURL, err := defaultServerUrlFor(&config)
	if err != nil {
		return false
	}
	return baseURL.Scheme == "https"
}

// LoadTLSFiles copies the data from the CertFile, KeyFile, and CAFile fields into the CertData,
// KeyData, and CAFile fields, or returns an error. If no error is returned, all three fields are
// either populated or were empty to start.
func LoadTLSFiles(c *Config) error {
	var err error
	c.CAData, err = dataFromSliceOrFile(c.CAData, c.CAFile)
	if err != nil {
		return err
	}

	c.CertData, err = dataFromSliceOrFile(c.CertData, c.CertFile)
	if err != nil {
		return err
	}

	c.KeyData, err = dataFromSliceOrFile(c.KeyData, c.KeyFile)
	if err != nil {
		return err
	}
	return nil
}

// dataFromSliceOrFile returns data from the slice (if non-empty), or from the file,
// or an error if an error occurred reading the file
func dataFromSliceOrFile(data []byte, file string) ([]byte, error) {
	if len(data) > 0 {
		return data, nil
	}
	if len(file) > 0 {
		fileData, err := ioutil.ReadFile(file)
		if err != nil {
			return []byte{}, err
		}
		return fileData, nil
	}
	return nil, nil
}

func AddUserAgent(config *Config, userAgent string) *Config {
	fullUserAgent := "meridian/" + userAgent
	config.UserAgent = fullUserAgent
	return config
}

// NewPool returns an x509.CertPool containing the certificates in the given PEM-encoded file.
// Returns an error if the file could not be read, a certificate could not be parsed, or if the file does not contain any certificates
func NewPool(filename string) (*x509.CertPool, error) {
	certs, err := CertsFromFile(filename)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	for _, cert := range certs {
		pool.AddCert(cert)
	}
	return pool, nil
}

// CertsFromFile returns the x509.Certificates contained in the given PEM-encoded file.
// Returns an error if the file could not be read, a certificate could not be parsed, or if the file does not contain any certificates
func CertsFromFile(file string) ([]*x509.Certificate, error) {
	pemBlock, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	certs, err := ParseCertsPEM(pemBlock)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %s", file, err)
	}
	return certs, nil
}

// ParseCertsPEM returns the x509.Certificates contained in the given PEM-encoded byte array
// Returns an error if a certificate could not be parsed, or if the data does not contain any certificates
func ParseCertsPEM(pemCerts []byte) ([]*x509.Certificate, error) {
	ok := false
	var certs []*x509.Certificate
	for len(pemCerts) > 0 {
		var block *pem.Block
		block, pemCerts = pem.Decode(pemCerts)
		if block == nil {
			break
		}
		// Only use PEM "CERTIFICATE" blocks without extra headers
		if block.Type != "CERTIFICATE" || len(block.Headers) != 0 {
			continue
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return certs, err
		}

		certs = append(certs, cert)
		ok = true
	}

	if !ok {
		return certs, fmt.Errorf("data does not contain any valid RSA or ECDSA certificates")
	}
	return certs, nil
}
