package crds

import (
	"context"
	"fmt"
	"github.com/aoxn/meridian/internal/tool"
	"k8s.io/klog/v2"
	"time"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextcli "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	checkCRDInterval = 2 * time.Second
	crdReadyTimeout  = 3 * time.Minute
)

var (
	defCategories = []string{"all"}
)

// Scope is the scope of a CRD.
type Scope = apiextv1.ResourceScope

const (
	// ClusterScoped represents a type of cluster scoped CRD.
	ClusterScoped = apiextv1.ClusterScoped
	// NamespaceScoped represents a type of namespaced scoped CRD.
	NamespaceScoped = apiextv1.NamespaceScoped
)

// Conf is the configuration required to create a CRD
type Conf struct {
	// Kind is the kind of the CRD.
	Kind string
	// NamePlural is the plural name of the CRD (in most cases the plural of Kind).
	NamePlural string
	// ShortNames are short names of the CRD.  It must be all lowercase.
	ShortNames []string
	// Group is the group of the CRD.
	Group string
	// Version is the version of the CRD.
	Version string
	// Scope is the scode of the CRD (cluster scoped or namespace scoped).
	Scope Scope
	// Categories is a way of grouping multiple resources (example `kubectl get all`),
	// adds the CRD to `all` and `knode` categories(apart from the described in Caregories).
	Categories []string
	// EnableStatus will enable the Status subresource on the CRD. This is feature
	// entered in v1.10 with the CRD subresources.
	// By default, is disabled.
	EnableStatusSubresource bool
	// EnableScaleSubresource by default will be nil and means disabled, if
	// the object is present it will set this scale configuration to the subresource.
	EnableScaleSubresource *apiextv1.CustomResourceSubresourceScale
}

func (c *Conf) getName() string {
	return fmt.Sprintf("%s.%s", c.NamePlural, c.Group)
}

// Interface is the CRD client that knows how to interact with k8s to manage them.
type Interface interface {
	// EnsurePresent EnsureCreated will ensure the the CRD is present, this also means that
	// apart from creating the CRD if is not present it will wait until is
	// ready, this is a blocking operation and will return an error if timesout
	// waiting.
	EnsurePresent(conf Conf) error
	// WaitToBePresent will wait until the CRD is present, it will check if
	// is present at regular intervals until it timesout, in case of timeout
	// will return an error.
	WaitToBePresent(name string, timeout time.Duration) error
	// Delete will delete the CRD.
	Delete(name string) error
}

// Client is the CRD client implementation using API calls to kubernetes.
type Client struct {
	client apiextcli.Interface
}

// NewClient returns a new CRD client.
func NewClient(client apiextcli.Interface) *Client {
	return NewCustomClient(client)
}

// NewCustomClient returns a new CRD client letting you set all the required parameters
func NewCustomClient(
	client apiextcli.Interface,
) *Client {
	return &Client{
		client: client,
	}
}

// EnsurePresent satisfies tool.Interface.
func (c *Client) EnsurePresent(conf Conf) error {
	err := c.validateCRD()
	if err != nil {
		return fmt.Errorf("validate tool: %s", err.Error())
	}

	// Get the generated name of the CRD.
	crdName := conf.getName()
	unknownFields := true
	crd := &apiextv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: crdName,
		},
		Spec: apiextv1.CustomResourceDefinitionSpec{
			Group: conf.Group,
			Names: apiextv1.CustomResourceDefinitionNames{
				Plural:     conf.NamePlural,
				Kind:       conf.Kind,
				ShortNames: conf.ShortNames,
				Categories: c.addDefaultCaregories(conf.Categories),
			},
			Scope: conf.Scope,
			Versions: []apiextv1.CustomResourceDefinitionVersion{
				{
					Name: conf.Version, // todo: Added by Aoxn, tobe verified
					Subresources: &apiextv1.CustomResourceSubresources{
						Status: &apiextv1.CustomResourceSubresourceStatus{},
					},
					Storage: true,
					Served:  true,
					Schema: &apiextv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextv1.JSONSchemaProps{
							Type:                   "object",
							XPreserveUnknownFields: &unknownFields,
						},
					},
				},
			},
		},
		Status: apiextv1.CustomResourceDefinitionStatus{},
	}
	klog.V(5).Infof("do create crd: %s", tool.PrettyYaml(crd))
	_, err = c.client.
		ApiextensionsV1().
		CustomResourceDefinitions().
		Create(context.TODO(), crd, metav1.CreateOptions{})
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("error creating tool %s: %s", crdName, err)
		}
		return nil
	}
	klog.Infof("tool %s created, waiting to be ready...", crdName)
	return c.WaitToBePresent(crdName, crdReadyTimeout)
}

// WaitToBePresent satisfies tool.Interface.
func (c *Client) WaitToBePresent(name string, timeout time.Duration) error {
	err := c.validateCRD()
	if err != nil {
		return fmt.Errorf("wait validate tool: %s", err.Error())
	}

	tick := time.NewTicker(checkCRDInterval)
	for {
		select {
		case <-tick.C:
			_, err := c.client.
				ApiextensionsV1().
				CustomResourceDefinitions().
				Get(
					context.TODO(), name, metav1.GetOptions{},
				)
			// Is present, finish.
			if err == nil {
				return nil
			}
		case <-time.After(timeout):
			return fmt.Errorf("timeout waiting for CRD")
		}
	}
}

// Delete satisfies tool.Interface.
func (c *Client) Delete(name string) error {
	err := c.validateCRD()
	if err != nil {
		return fmt.Errorf("validate tool: %s", err.Error())
	}

	return c.client.
		ApiextensionsV1beta1().
		CustomResourceDefinitions().
		Delete(
			context.TODO(), name, metav1.DeleteOptions{},
		)
}

// validateCRD returns nil if cluster is ok to be used for CRDs, otherwise error.
func (c *Client) validateCRD() error {
	// Check cluster version.
	_, err := c.client.Discovery().ServerVersion()
	if err != nil {
		return fmt.Errorf("get server version: %s", err.Error())
	}
	// klog.Infof("kubernetes version should great then >v1.7.0 to use tool. current version=%s", v)
	return nil
}

// addAllCaregory adds the `all` category if isn't present
func (c *Client) addDefaultCaregories(categories []string) []string {
	currentCats := make(map[string]bool)
	for _, ca := range categories {
		currentCats[ca] = true
	}

	// Add default categories if required.
	for _, ca := range defCategories {
		if _, ok := currentCats[ca]; !ok {
			categories = append(categories, ca)
		}
	}

	return categories
}
