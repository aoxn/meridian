package api

import (
	"context"
	"fmt"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/node"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
)

func Create(ctx context.Context, obj runtime.Object, opt *metav1.CreateOptions) (runtime.Object, error) {
	req, ok := obj.(*v1.Request)
	if !ok {
		return obj, fmt.Errorf("unexpected object type: %T, expect v1.Request", obj)
	}
	md, err := node.NewMeridianNode(v1.ActionInit, v1.NodeRoleMaster, "", "", req, []string{})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to init meridian")
	}
	err = md.EnsureNode()
	if err != nil {
		return nil, fmt.Errorf("ensuring node: %w", err)
	}
	return nil, nil
}

func Get(ctx context.Context, object runtime.Object, options *metav1.GetOptions) (runtime.Object, error) {
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
