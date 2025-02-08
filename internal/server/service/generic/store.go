package generic

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aoxn/meridian/internal/server/service"
)

var _ service.Standard = &Store{}

type Store struct {
	Scheme *runtime.Scheme
	// NewFunc returns a new instance of the type this registry returns for a
	// GET of a single object, e.g.:
	//
	// curl GET /apis/group/version/namespaces/my-ns/myresource/name-of-object
	NewFunc func() client.Object

	// NewListFunc returns a new list of the type this registry; it is the
	// type returned when the resource is listed, e.g.:
	//
	// curl GET /apis/group/version/namespaces/my-ns/myresource
	NewListFunc func() client.Object

	GVKGetterFunc func() schema.GroupVersionKind

	// KeyRootFunc returns the root etcd key for this resource; should not
	// include trailing "/".  This is used for operations that work on the
	// entire collection (listing and watching).
	//
	// KeyRootFunc and KeyFunc must be supplied together or not at all.
	KeyRootFunc func(ctx context.Context) string

	// KeyFunc returns the key for a specific object in the collection.
	// KeyFunc is called for Create/Update/Get/Delete. Note that 'namespace'
	// can be gotten from ctx.
	//
	// KeyFunc and KeyRootFunc must be supplied together or not at all.
	KeyFunc func(ctx context.Context, name string) (string, error)

	// Decorator is an optional exit hook on an object returned from the
	// underlying storage. The returned object could be an individual object
	// (e.g. Pod) or a list type (e.g. PodList). Decorator is intended for
	// integrations that are above storage and should only be used for
	// specific cases where storage of the value is not appropriate, since
	// they cannot be watched.
	Decorator func(client.Object)

	// Storage is the interface for the underlying storage for the
	// resource. It is wrapped into a "DryRunnableStorage" that will
	// either pass-through or simply dry-run.
	Storage service.Standard
}

func (u *Store) GVR() schema.GroupVersionResource {
	//TODO implement me
	panic("implement me")
}

func (u *Store) newObject(r *schema.GroupVersionResource, list bool) runtime.Object {
	knowns := u.Scheme.AllKnownTypes()
	for k := range knowns {
		if k.GroupVersion().String() != r.GroupVersion().String() {
			continue
		}
		var (
			kind     = strings.ToLower(k.Kind)
			resource = strings.ToLower(r.Resource)
		)
		if list {
			resource = fmt.Sprintf("%slist", resource[0:len(resource)-1])
		} else {
			kind = fmt.Sprintf("%ss", kind)
		}
		if kind != resource {
			continue
		}
		object, err := u.Scheme.New(k)
		if err != nil {
			klog.Infof("resource[%s] constructed in schema[%s], %s", r.String(), u.Scheme.Name(), err.Error())
			return nil
		}
		metav, err := meta.TypeAccessor(object)
		if err != nil {
			klog.Infof("accesse resource[%s] constructed in schema[%s], %s", r.String(), u.Scheme.Name(), err.Error())
			return nil
		}
		metav.SetKind(k.Kind)
		metav.SetAPIVersion(k.GroupVersion().String())
		return object
	}
	klog.Infof("resource[%s] not found in schema[%s]", r.String(), u.Scheme.Name())
	return nil
}

func (u *Store) New(r *schema.GroupVersionResource) runtime.Object {
	if u.NewFunc != nil {
		return u.NewFunc()
	}
	return u.newObject(r, false)
}

func (u *Store) NewList(r *schema.GroupVersionResource) runtime.Object {
	if u.NewListFunc != nil {
		return u.NewListFunc()
	}
	return u.newObject(r, true)
}
