package image

import (
	"context"
	"encoding/json"
	"fmt"
	v1 "github.com/aoxn/meridian/api/v1"
	w "github.com/aoxn/meridian/internal/meridian/worker"
	"github.com/aoxn/meridian/internal/server/service"
	"github.com/aoxn/meridian/internal/server/service/universal"
	"github.com/aoxn/meridian/internal/tool"
	"github.com/aoxn/meridian/internal/vma/model"
	"k8s.io/apimachinery/pkg/api/meta"
	u "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
)

func NewImagePvd(opt *service.Options) service.Provider {
	return &imagePvd{options: opt}
}

type imagePvd struct {
	options *service.Options
}

func (m *imagePvd) NewAPIGroup(ctx context.Context) (service.Grouped, error) {
	grp := service.Grouped{}
	t := m.addV1(grp, m.options)
	return grp, m.initPrvd(ctx, t)
}

func (m *imagePvd) addV1(grp service.Grouped, options *service.Options) *image {
	univ := &image{
		Store:   universal.NewUniversal(options),
		scheme:  options.Scheme,
		freezer: w.NewFreeze(),
		// allowedResource
		allowedResource: sets.New[string](),
	}
	grp.AddOrDie(univ)
	return univ
}

func (m *imagePvd) initPrvd(ctx context.Context, t *image) error {
	return nil
}

type image struct {
	service.Store
	work            *w.WorkerMgr
	freezer         *w.Action
	scheme          *runtime.Scheme
	allowedResource sets.Set[string]
}

func (m *image) GVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    v1.GroupVersion.Group,
		Version:  v1.GroupVersion.Version,
		Resource: "images",
	}
}

func (m *image) isAllowed(r string) bool {
	r = strings.ToLower(r)
	if m.allowedResource.Len() == 0 {
		return true
	}
	return m.allowedResource.Has(r)
}

func (m *image) List(ctx context.Context, out runtime.Object, options *metav1.ListOptions) (runtime.Object, error) {
	imageDir, err := model.MdImagesDir()
	if err != nil {
		return nil, err
	}
	var images = &v1.ImageList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ImageList",
			APIVersion: v1.GroupVersion.String(),
		},
		ListMeta: metav1.ListMeta{},
	}
	archs, err := os.Stat(imageDir)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	if !archs.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", imageDir)
	}

	entrys, err := os.ReadDir(imageDir)
	if err != nil {
		return nil, err
	}
	itmes := make([]runtime.Object, 0)
	for _, entry := range entrys {
		if !entry.IsDir() {
			continue
		}
		klog.Infof("image arch: %s", entry.Name())
		imgs, err := os.ReadDir(filepath.Join(imageDir, entry.Name()))
		if err != nil {
			return nil, err
		}
		for _, img := range imgs {
			if !img.IsDir() {
				continue
			}
			f := filepath.Join(
				imageDir,
				entry.Name(),
				img.Name(),
				"data")
			data, err := os.ReadFile(f)
			if err != nil {
				klog.Infof("failed to read data from %s, %s", f, err)
				continue
			}
			m := &v1.File{}
			err = json.Unmarshal(data, m)
			if err != nil {
				klog.Infof("failed to unmarshal data from %s, %s", f, err)
				continue
			}
			klog.Infof("image arch: %s, img=%s", entry.Name(), img.Name())
			image := v1.Image{
				ObjectMeta: metav1.ObjectMeta{
					Name: img.Name(),
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "Image",
					APIVersion: v1.GroupVersion.String(),
				},
				Spec: v1.ImageSpec{
					Name: img.Name(),
					Arch: entry.Name(),
					OS:   m.OS,
				},
			}
			itmes = append(itmes, &image)
		}
	}
	err = meta.SetList(out, itmes)
	klog.Infof("images: %+v", tool.PrettyYaml(out))
	return images, nil
}

func (m *image) Update(ctx context.Context, object runtime.Object, options *metav1.UpdateOptions) (runtime.Object, error) {
	return object, fmt.Errorf("unexpected [update] operator")
}

func (m *image) Create(ctx context.Context, object runtime.Object, options *metav1.CreateOptions) (runtime.Object, error) {
	return nil, fmt.Errorf("unimplemented")
}

func (m *image) Delete(ctx context.Context, object runtime.Object, options *metav1.DeleteOptions) (runtime.Object, error) {
	if object == nil {
		return nil, service.NewUnknownObjectError("")
	}
	gvk := object.GetObjectKind().GroupVersionKind()
	if !m.isAllowed(gvk.Kind) {
		return nil, service.NewNotAllowedError(gvk.String())
	}
	img, ok := object.(*v1.Image)
	if !ok {
		return nil, service.NewUnknownObjectError("unknown object type")
	}
	imgDir, err := model.MdImagesDir()
	if err != nil {
		return nil, err
	}
	return img, os.RemoveAll(filepath.Join(imgDir, img.Spec.Arch, img.Spec.Name))
}

func (m *image) DeleteCollection(ctx context.Context, options *metav1.DeleteOptions) (runtime.Object, error) {

	return nil, fmt.Errorf("unimplemented")
}

func IsNotFound(err error) bool {
	return strings.Contains(err.Error(), "NotFound")
}

func getNewItemFunc(listObj runtime.Object, v reflect.Value) func() runtime.Object {
	// For unstructured lists with a target group/version, preserve the group/version in the instantiated list items
	if unstructuredList, isUnstructured := listObj.(*u.UnstructuredList); isUnstructured {
		if apiVersion := unstructuredList.GetAPIVersion(); len(apiVersion) > 0 {
			return func() runtime.Object {
				return &u.Unstructured{Object: map[string]interface{}{"apiVersion": apiVersion}}
			}
		}
	}

	// Otherwise just instantiate an empty item
	elem := v.Type().Elem()
	return func() runtime.Object {
		return reflect.New(elem).Interface().(runtime.Object)
	}
}
