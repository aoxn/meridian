package oss

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aoxn/meridian/internal/cloud"
	"github.com/aoxn/meridian/internal/server/service"
	"github.com/aoxn/meridian/internal/tool"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"strings"
)

var (
	_ service.Standard = &oss{}
	_ service.Store    = &oss{}
)

func NewIndex(store cloud.IObjectStorage, kind string) service.Store {
	return newIndex(store.BucketName(), kind, store)
}

func newIndex(
	bucket string, kind string, store cloud.IObjectStorage,
) *oss {
	return &oss{bucket: bucket, store: store, gvk: kind}
}

type oss struct {
	service.Standard
	bucket string
	gvk    string
	store  cloud.IObjectStorage
}

func (n *oss) resource() string {
	return fmt.Sprintf("%s", strings.ToLower(n.gvk))
}

func (n *oss) kind() string {
	return fmt.Sprintf("meridian/%s", n.resource())
}

func (n *oss) root() string {
	return fmt.Sprintf("oss://%s/%s", n.bucket, n.kind())
}

func (n *oss) location(id string) string {
	return fmt.Sprintf("%s/%s", n.root(), id)
}

func decode(contents [][]byte, out runtime.Object) error {

	var (
		u *unstructured.Unstructured
		o = &unstructured.UnstructuredList{}
	)

	var items []interface{}
	for i, data := range contents {
		m := unstructured.Unstructured{}
		err := m.UnmarshalJSON(data)
		if err != nil {
			return err
		}
		if i == 0 {
			u = &m
		}
		items = append(items, m.Object)
	}
	if u == nil {
		return fmt.Errorf("no resource found")
	}
	gvk := u.GroupVersionKind()
	gvk.Kind = fmt.Sprintf("%sList", gvk.Kind)
	o.SetUnstructuredContent(map[string]interface{}{"items": items})
	o.SetGroupVersionKind(gvk)
	data, _ := o.MarshalJSON()
	err := json.Unmarshal(data, out)
	return err
}

func (n *oss) Get(ctx context.Context, in runtime.Object, option *metav1.GetOptions) (runtime.Object, error) {
	metav, err := meta.Accessor(in)
	if err != nil {
		return nil, err
	}
	data, err := n.store.GetObject(n.location(metav.GetName()))
	if err != nil {
		return nil, errors.Wrapf(err, "get oss: %s", metav.GetName())
	}
	err = json.Unmarshal(data, in)
	if err != nil {
		return nil, err
	}
	return in, nil
}

func (n *oss) List(ctx context.Context, out runtime.Object, options *metav1.ListOptions) (runtime.Object, error) {
	contents, err := n.store.ListObject(n.kind())
	if err != nil {
		return nil, errors.Wrapf(err, "fetch resource list")
	}
	err = decode(contents, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (n *oss) Update(ctx context.Context, in runtime.Object, option *metav1.UpdateOptions) (runtime.Object, error) {
	metav, err := meta.Accessor(in)
	if err != nil {
		return nil, err
	}
	return in, n.store.PutObject([]byte(tool.PrettyJson(in)), n.location(metav.GetName()))
}

func (n *oss) Create(ctx context.Context, in runtime.Object, option *metav1.CreateOptions) (runtime.Object, error) {
	metav, err := meta.Accessor(in)
	if err != nil {
		return nil, err
	}
	return in, n.store.PutObject([]byte(tool.PrettyJson(in)), n.location(metav.GetName()))
}

func (n *oss) Delete(ctx context.Context, in runtime.Object, option *metav1.DeleteOptions) (runtime.Object, error) {
	metav, err := meta.Accessor(in)
	if err != nil {
		return nil, err
	}
	return in, n.store.DeleteObject(n.location(metav.GetName()))
}

func (n *oss) DeleteCollection(ctx context.Context, options *metav1.DeleteOptions) (runtime.Object, error) {
	//TODO implement me
	panic("implement me")
}

func (n *oss) Watch(ctx context.Context, in runtime.Object, options *metav1.ListOptions) (watch.Interface, error) {
	//TODO implement me
	panic("implement me")
}

func (n *oss) Destroy() {
	//TODO implement me
	panic("implement me")
}
