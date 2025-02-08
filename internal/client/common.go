package client

import (
	"context"
	"fmt"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/client/rest"
	"k8s.io/klog/v2"
	"net"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

func Client(ep string) (Interface, error) {
	cfg := rest.Config{
		Host:        ep,
		ContentType: "application/json",
		UserAgent:   "kubernetes.meridian",
	}

	if strings.HasPrefix(ep, "/") {
		cfg.Host = "localhost"
		dialer := func(_ context.Context, network, addr string) (net.Conn, error) {
			return net.Dial("unix", ep)
		}
		cfg.DialContext = dialer
		klog.V(8).Infof("use dial context: %s", ep)
	}
	rclient, err := rest.RESTClientFor(&cfg)
	if err != nil {
		return nil, err
	}
	return New(rclient), nil
}

func GetClient() (Interface, error) {
	rest, err := Rest()
	if err != nil {
		return nil, err
	}
	return New(rest), nil
}

func Rest() (rest.Interface, error) {
	return RestClientFor("")
}

func RestClientFor(endpoint string) (rest.Interface, error) {
	return rest.RESTClientFor(
		&rest.Config{
			Host:        endpoint,
			ContentType: "application/json",
			UserAgent:   "kubernetes.meridian",
		},
	)
}

type Interface interface {
	Raw() rest.Interface
	Create(context.Context, client.Object) error
	Update(context.Context, client.Object) error
	Delete(context.Context, client.Object) error
	Get(context.Context, client.Object) error
	List(context.Context, client.ObjectList) error
}

func New(
	client rest.Interface,
) Interface {
	return &resourceSet{
		client: client,
	}
}

type resourceSet struct {
	client rest.Interface
}

func (m *resourceSet) kind(o client.Object) string {
	return fmt.Sprintf("%ss", strings.ToLower(o.GetObjectKind().GroupVersionKind().Kind))
}

var pathPrefix = fmt.Sprintf("/apis/%s/v1/", v1.GroupVersion.Group)

func (m *resourceSet) Create(ctx context.Context, o client.Object) error {
	err := m.client.
		Post().
		PathPrefix(pathPrefix).
		Resource(m.kind(o)).
		Body(o).
		Do(o)
	return err
}

func (m *resourceSet) Update(ctx context.Context, o client.Object) error {
	err := m.client.
		Put().
		PathPrefix(pathPrefix).
		Resource(m.kind(o)).
		Body(o).
		Do(o)
	return err
}

func (m *resourceSet) Delete(ctx context.Context, o client.Object) error {
	err := m.client.
		Delete().
		PathPrefix(pathPrefix).
		Resource(m.kind(o)).
		ResourceName(o.GetName()).
		Do(o)
	return err
}

func (m *resourceSet) Get(ctx context.Context, o client.Object) error {
	err := m.client.
		Get().
		PathPrefix(pathPrefix).
		Resource(m.kind(o)).
		ResourceName(o.GetName()).
		Do(o)
	return err
}

func (m *resourceSet) List(ctx context.Context, out client.ObjectList) error {
	kind := strings.ToLower(out.GetObjectKind().GroupVersionKind().Kind)
	if strings.HasSuffix(kind, "list") {
		kind = kind[0 : len(kind)-4]
	}
	err := m.client.
		Get().
		PathPrefix(pathPrefix).
		Resource(fmt.Sprintf("%ss", kind)).
		Do(out)
	return err
}

func (m *resourceSet) Raw() rest.Interface {
	return m.client
}
