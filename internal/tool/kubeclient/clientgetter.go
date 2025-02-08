package kubeclient

import (
	diskcached "k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/util/homedir"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var (
	defaultCacheDir = filepath.Join(homedir.HomeDir(), ".kube", "http-cache")
)

type restClientGetter struct {
	config *clientcmdapi.Config
}

var _ genericclioptions.RESTClientGetter = restClientGetter{}

func (r restClientGetter) ToRESTConfig() (*rest.Config, error) {
	return r.ToRawKubeConfigLoader().ClientConfig()
}

func (r restClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	config, err := r.ToRESTConfig()
	if err != nil {
		return nil, err
	}

	// The more groups you have, the more discovery requests you need to make.
	// given 25 groups (our groups + a few custom resources) with
	// one-ish version each, discovery needs to make 50 requests
	// double it just so we don't end up here again for a while.
	// This config is only used for discovery.
	config.Burst = 100

	discoveryCacheDir := computeDiscoverCacheDir(filepath.Join(homedir.HomeDir(), ".kube", "cache", "discovery"), config.Host)
	return diskcached.NewCachedDiscoveryClientForConfig(config, discoveryCacheDir, defaultCacheDir, time.Duration(10*time.Minute))
}

func (r restClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	client, err := r.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}
	return restmapper.NewDeferredDiscoveryRESTMapper(client), nil
}

func (r restClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	if r.config == nil {
		// use incluster config
		return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{}, &clientcmd.ConfigOverrides{})
	}
	return clientcmd.NewDefaultClientConfig(*r.config, &clientcmd.ConfigOverrides{})
}

func NewClientGetter(config *clientcmdapi.Config) genericclioptions.RESTClientGetter {
	return &restClientGetter{config}
}

func NewClientGetterInCluster() genericclioptions.RESTClientGetter {
	return &restClientGetter{}
}

// ifileChar matches characters that *might* not be supported.
// Windows is really restrictive, so this is really restrictive
var ifileChar = regexp.MustCompile(`[^(\w/\.)]`)

// computeDiscoverCacheDir takes the parentDir and the host and
// comes up with a "usually non-colliding" name.
func computeDiscoverCacheDir(parentDir, host string) string {
	// strip the optional scheme from host if its there:
	schemelessHost := strings.Replace(
		strings.Replace(host, "https://", "", 1), "http://", "", 1,
	)
	// now do a simple collapse of non-AZ09 characters.
	// Collisions are possible but unlikely.
	// Even if we do collide the problem is short lived
	safeHost := ifileChar.ReplaceAllString(schemelessHost, "_")
	return filepath.Join(parentDir, safeHost)
}
