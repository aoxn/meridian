package service

import (
	"context"
	"fmt"
	v1 "github.com/aoxn/meridian/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	"reflect"
)

type Options struct {
	Scheme   *runtime.Scheme
	Provider v1.AuthInfo
}

type Provider interface {
	// NewAPIGroup returns a new storage object.
	NewAPIGroup(ctx context.Context) (Grouped, error)
}

var APIGroup = Grouped{}

func newResourced() Resourced {
	return make(map[string]Standard)
}

func newVersiond() Versiond {
	return make(map[string]Resourced)
}

type Grouped map[string]Versiond

type Versiond map[string]Resourced

type Resourced map[string]Standard

func (g Grouped) Debug() {
	for i, r := range g {
		for j, v := range r {
			for k, _ := range v {
				klog.Infof("resource: %s/%s/%s", i, j, k)
			}
		}
	}
}

func (g Grouped) AddGroupOrDie(grp Grouped) {
	for _, r := range grp {
		for _, v := range r {
			for _, s := range v {
				g.AddOrDie(s)
			}
		}
	}
}

func (g Grouped) AddOrDie(stg Standard) {
	gvr := stg.GVR()
	versiond, ok := g[gvr.Group]
	if !ok {
		versiond = newVersiond()
	}
	versiond.AddOrDie(stg)
	g[gvr.Group] = versiond
}

func (v Versiond) AddOrDie(sr Standard) {
	gvr := sr.GVR()
	resource, ok := v[gvr.Version]
	if !ok {
		resource = newResourced()
	}
	resource.AddOrDie(sr)
	v[gvr.Version] = resource
}

func (v Resourced) AddOrDie(sr Standard) {
	gvr := sr.GVR()
	_, ok := v[gvr.Resource]
	if ok {
		panic(fmt.Sprintf("already exist: %s, %s", gvr.Resource, gvr.String()))
	}
	v[gvr.Resource] = sr
	klog.Infof("add versioned storage resource: [%s/%s/%s]", gvr.Group, gvr.Version, gvr.Resource)
}

func (g Grouped) Service(gv *schema.GroupVersionResource) Standard {
	grp, ok := g[gv.Group]
	if !ok {
		return g.universal()
	}
	version, ok := grp[gv.Version]
	if !ok {
		return g.universal()
	}
	resourced, ok := version[gv.Resource]
	if !ok {
		return g.universal()
	}
	klog.V(5).Infof("find service resource: [%s/%s/%s], %s", gv.Group, gv.Version, gv.Resource, reflect.TypeOf(version))
	return resourced
}

func (g Grouped) universal() Standard {
	grp, ok := g[UniversalGrp]
	if !ok {
		return nil
	}
	resourced, ok := grp[UniversalResource]
	if !ok {
		return nil
	}
	version, ok := resourced[UniversalVersion]
	if !ok {
		return nil
	}
	return version
}

const (
	UniversalGrp      = "universal"
	UniversalResource = "universal"
	UniversalVersion  = "universal"
)

func NewNotAllowedError(r string) error {
	return fmt.Errorf("ResourceNotAllowed: %s", r)
}

func NewUnknownObjectError(r string) error {
	return fmt.Errorf("UnknownObject: [%s]", r)
}
