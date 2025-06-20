package svc

import (
	"context"
	"fmt"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/node"
	"github.com/aoxn/meridian/internal/server/service"
	"github.com/aoxn/meridian/internal/server/service/backend/local"
	"github.com/aoxn/meridian/internal/server/service/generic"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
)

func NewKubernetesPvd(opt *service.Options) service.Provider {
	return &kubernetesPvd{options: opt}
}

type kubernetesPvd struct {
	options *service.Options
}

func (v *kubernetesPvd) NewAPIGroup(ctx context.Context) (service.Grouped, error) {
	grp := service.Grouped{}
	v.addV1(grp, v.options)
	return grp, nil
}

func (v *kubernetesPvd) addV1(grp service.Grouped, options *service.Options) {
	univ := NewKubernetes(options)
	grp.AddOrDie(univ)
}

func NewKubernetes(options *service.Options) service.Store {
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
	univ := &kubernetes{
		Store:           store,
		scheme:          options.Scheme,
		allowedResource: sets.New[string](),
	}
	return univ
}

type kubernetes struct {
	service.Store
	scheme          *runtime.Scheme
	allowedResource sets.Set[string]
}

func (u *kubernetes) GVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    v1.GroupVersion.Group,
		Version:  v1.GroupVersion.Version,
		Resource: "requests",
	}
}

func (u *kubernetes) Create(ctx context.Context, obj runtime.Object, opt *metav1.CreateOptions) (runtime.Object, error) {
	klog.Infof("receive kubernetes create event for [%s]", u.GVR())
	_, err := u.Store.Get(ctx, obj.DeepCopyObject(), nil)
	if err != nil {
		if !errors.Is(err, local.NotFound) {
			return nil, err
		}
		klog.Infof("k8s not initialized, try ensure")
	} else {
		klog.Infof("object already exists: %s", obj.GetObjectKind())
		return obj, nil
	}
	req, ok := obj.(*v1.Request)
	if !ok {
		return obj, fmt.Errorf("unexpected object type: %T, expect v1.Request", obj)
	}
	klog.Infof("ensure kubernetes for [%s]", u.GVR())
	md, err := node.NewMeridianNode(v1.ActionInit, v1.NodeRoleMaster, "", "", req, []string{})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to init meridian")
	}
	err = md.EnsureNode()
	if err != nil {
		return nil, fmt.Errorf("ensuring node: %w", err)
	}
	return u.Store.Create(ctx, req, nil)
}

func (u *kubernetes) Get(ctx context.Context, object runtime.Object, options *metav1.GetOptions) (runtime.Object, error) {
	addrs, err := GetLocalIP()
	if err != nil {
		return nil, err
	}
	gi, ok := object.(*v1.GuestInfo)
	if !ok {
		return nil, fmt.Errorf("object is not a v1.GuestInfo")
	}
	gi.Spec.Address = addrs
	gi.Status = v1.GuestInfoStatus{
		Phase: v1.Running,
	}
	klog.Infof("get guest-vm address: %s", addrs)
	return gi, nil
}
