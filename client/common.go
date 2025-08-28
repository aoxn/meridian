package client

import (
	"context"
	rest2 "github.com/aoxn/meridian/client/rest"
	"k8s.io/klog/v2"
	"net"
	"strings"
)

func Client(ep string) (Interface, error) {
	cfg := rest2.Config{
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
	rclient, err := rest2.RESTClientFor(&cfg)
	if err != nil {
		return nil, err
	}
	return New(rclient), nil
}

func ClientWith(dialer func(ctx context.Context, network, addr string) (net.Conn, error)) (Interface, error) {
	cfg := rest2.Config{
		Host:        "localhost",
		DialContext: dialer,
		ContentType: "application/json",
		UserAgent:   "kubernetes.meridian",
	}
	klog.V(8).Infof("use dial context")
	rclient, err := rest2.RESTClientFor(&cfg)
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

func Rest() (rest2.Interface, error) {
	return RestClientFor("")
}

func RestClientFor(endpoint string) (rest2.Interface, error) {
	return rest2.RESTClientFor(
		&rest2.Config{
			Host:        endpoint,
			ContentType: "application/json",
			UserAgent:   "kubernetes.meridian",
		},
	)
}

type Interface interface {
	Raw() rest2.Interface
	Healthz(context.Context) error
	Create(context.Context, string, string, any) error
	Update(context.Context, string, string, any) error
	Delete(context.Context, string, string, any) error
	Get(context.Context, string, string, any) error
	List(context.Context, string, any) error
}

func New(
	client rest2.Interface,
) Interface {
	return &resourceSet{
		client: client,
	}
}

type resourceSet struct {
	client rest2.Interface
}

var pathPrefix = "/api/v1/"

func (m *resourceSet) Create(ctx context.Context, r, name string, o any) error {
	err := m.client.
		Post().
		PathPrefix(pathPrefix).
		Resource(r).
		ResourceName(name).
		Body(o).
		Do(o)
	return err
}

func (m *resourceSet) Update(ctx context.Context, r, name string, o any) error {
	err := m.client.
		Put().
		PathPrefix(pathPrefix).
		Resource(r).
		ResourceName(name).
		Body(o).
		Do(o)
	if err == nil {
		klog.Infof("[%s/%s] Accepted", r, name)
	}
	return err
}

func (m *resourceSet) Delete(ctx context.Context, r string, n string, o any) error {
	err := m.client.
		Delete().
		PathPrefix(pathPrefix).
		Resource(r).
		ResourceName(n).
		Body(o).
		DirectDo()
	if err == nil {
		klog.Infof("delete [%s/%s] Accepted", r, n)
	}
	return err
}

func (m *resourceSet) Get(ctx context.Context, r string, name string, o any) error {
	err := m.client.
		Get().
		PathPrefix(pathPrefix).
		Resource(r).
		ResourceName(name).
		Do(o)
	return err
}

func (m *resourceSet) Healthz(ctx context.Context) error {
	err := m.client.
		Get().
		Resource("healthz").
		DirectDo()
	return err
}

func (m *resourceSet) List(ctx context.Context, r string, out any) error {
	err := m.client.
		Get().
		PathPrefix(pathPrefix).
		Resource(r).
		Do(out)
	return err
}

func (m *resourceSet) Raw() rest2.Interface {
	return m.client
}
