package universal

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/aoxn/meridian/internal/server/service"
	"github.com/aoxn/meridian/internal/server/service/backend/local"
	"github.com/aoxn/meridian/internal/server/service/generic"
)

func NewUniversalPvd(opt *service.Options) service.Provider {
	return &universalPvd{options: opt}
}

type universalPvd struct {
	options *service.Options
}

func (v *universalPvd) NewAPIGroup(ctx context.Context) (service.Grouped, error) {
	grp := service.Grouped{}
	v.addV1(grp, v.options)
	return grp, nil
}

func (v *universalPvd) addV1(grp service.Grouped, options *service.Options) {
	univ := NewUniversal(options)
	grp.AddOrDie(univ)
}

func NewUniversal(options *service.Options) service.Store {
	var store service.Store
	switch options.Provider.Type {
	case "Local":
		store = &local.Local{
			Standard: &generic.Store{
				Scheme: options.Scheme,
			},
		}
	default:
		panic(fmt.Sprintf("unimplemented provider type: [%s]", options.Provider.Type))
	}
	univ := &universal{
		Store:           store,
		scheme:          options.Scheme,
		allowedResource: sets.New[string](),
	}
	return univ
}

type universal struct {
	service.Store
	scheme          *runtime.Scheme
	allowedResource sets.Set[string]
}

func (u *universal) GVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    service.UniversalGrp,
		Version:  service.UniversalVersion,
		Resource: service.UniversalResource,
	}
}

func (u *universal) isAllowed(r string) bool {
	r = strings.ToLower(r)
	if u.allowedResource.Len() == 0 {
		return true
	}
	return u.allowedResource.Has(r)
}

func (u *universal) Get(ctx context.Context, object runtime.Object, options *metav1.GetOptions) (runtime.Object, error) {
	if object == nil {
		return nil, service.NewUnknownObjectError("universal.get")
	}
	gvk := object.GetObjectKind().GroupVersionKind()
	if !u.isAllowed(gvk.Kind) {
		return nil, service.NewNotAllowedError(gvk.String())
	}
	return u.Store.Get(ctx, object, options)
}

func (u *universal) List(ctx context.Context, out runtime.Object, options *metav1.ListOptions) (runtime.Object, error) {
	if out == nil {
		return nil, service.NewUnknownObjectError("universal.list")
	}
	gvk := out.GetObjectKind().GroupVersionKind()
	if !u.isAllowed(gvk.Kind) {
		return nil, service.NewNotAllowedError(gvk.String())
	}
	return u.Store.List(ctx, out, options)
}

func (u *universal) Update(ctx context.Context, object runtime.Object, options *metav1.UpdateOptions) (runtime.Object, error) {
	if object == nil {
		return nil, service.NewUnknownObjectError("")
	}
	gvk := object.GetObjectKind().GroupVersionKind()
	if !u.isAllowed(gvk.Kind) {
		return nil, service.NewNotAllowedError(gvk.String())
	}
	return u.Store.Update(ctx, object, options)
}

func (u *universal) Create(ctx context.Context, object runtime.Object, options *metav1.CreateOptions) (runtime.Object, error) {
	if object == nil {
		return nil, service.NewUnknownObjectError("")
	}
	gvk := object.GetObjectKind().GroupVersionKind()
	if !u.isAllowed(gvk.Kind) {
		return nil, service.NewNotAllowedError(gvk.String())
	}
	return u.Store.Create(ctx, object, options)
}

func (u *universal) Delete(ctx context.Context, object runtime.Object, options *metav1.DeleteOptions) (runtime.Object, error) {
	if object == nil {
		return nil, service.NewUnknownObjectError("")
	}
	gvk := object.GetObjectKind().GroupVersionKind()
	if !u.isAllowed(gvk.Kind) {
		return nil, service.NewNotAllowedError(gvk.String())
	}
	return u.Store.Delete(ctx, object, options)
}

func (u *universal) DeleteCollection(ctx context.Context, options *metav1.DeleteOptions) (runtime.Object, error) {

	return u.Store.DeleteCollection(ctx, options)
}

func (u *universal) Watch(ctx context.Context, in runtime.Object, options *metav1.ListOptions) (watch.Interface, error) {
	if in == nil {
		return nil, service.NewUnknownObjectError("")
	}
	gvk := in.GetObjectKind().GroupVersionKind()
	if !u.isAllowed(gvk.Kind) {
		return nil, service.NewNotAllowedError(gvk.String())
	}
	return u.Store.Watch(ctx, in, options)
}
