package local

import (
	"context"
	"fmt"
	"github.com/aoxn/meridian/internal/vmm/model"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/ghodss/yaml"
	"k8s.io/apimachinery/pkg/api/meta"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	u "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog/v2"

	"github.com/aoxn/meridian/internal/server/service"
	"github.com/fsnotify/fsnotify"
)

var _ service.Standard = &Local{}

func NewLocal() service.Standard {
	return &Local{}
}

var NotFound = notFound{}

type notFound struct {
}

func (notFound) Error() string {
	return "NotFound"
}

type Local struct {
	service.Standard
}

func (l *Local) Get(ctx context.Context, object runtime.Object, options *metav1.GetOptions) (runtime.Object, error) {

	gvk := object.GetObjectKind().GroupVersionKind()
	path, err := mkPath(gvk)
	if err != nil {
		return object, err
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return object, NotFound
		}
		return object, err
	}
	if !info.IsDir() {
		return object, fmt.Errorf("unexpected local dir: %s", path)
	}
	dir, err := os.ReadDir(path)
	if err != nil {
		return object, err
	}
	metav, err := meta.Accessor(object)
	if err != nil {
		return object, err
	}
	klog.Infof("resource get: %s/%s", metav.GetNamespace(), metav.GetName())
	for _, f := range dir {
		if f.IsDir() {
			continue
		}
		if f.Name() != metav.GetName() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(path, f.Name()))
		if err != nil {
			return object, err
		}
		if err := yaml.Unmarshal(data, object); err != nil {
			return object, err
		}
		return object, nil
	}
	return object, NotFound
}

func (l *Local) List(ctx context.Context, out runtime.Object, options *metav1.ListOptions) (runtime.Object, error) {

	listPtr, err := meta.GetItemsPtr(out)
	if err != nil {
		return out, err
	}
	v, err := conversion.EnforcePtr(listPtr)
	if err != nil || v.Kind() != reflect.Slice {
		return out, fmt.Errorf("need ptr to slice: %v", err)
	}
	newItem := getNewItemFunc(out, v)
	gvk := out.GetObjectKind().GroupVersionKind()
	path, err := mkPath(gvk)
	if err != nil {
		return out, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return out, err
	}
	if !info.IsDir() {
		return out, fmt.Errorf("unexpected local dir: %s", path)
	}
	dir, err := os.ReadDir(path)
	if err != nil {
		return out, err
	}
	itmes := make([]runtime.Object, 0)
	for _, f := range dir {
		if f.IsDir() {
			continue
		}

		data, err := os.ReadFile(filepath.Join(path, f.Name()))
		if err != nil {
			return out, err
		}
		var obj = newItem()
		if err := yaml.Unmarshal(data, &obj); err != nil {
			continue
		}
		itmes = append(itmes, obj)
	}
	err = meta.SetList(out, itmes)
	return out, err
}

func (l *Local) Update(ctx context.Context, object runtime.Object, options *metav1.UpdateOptions) (runtime.Object, error) {
	gvk := object.GetObjectKind().GroupVersionKind()
	if gvk.Empty() {
		return nil, service.NewUnknownObjectError(gvk.String())
	}
	path, err := mkPath(gvk)
	if err != nil {
		return object, err
	}
	metav, err := meta.Accessor(object)
	if err != nil {
		return object, err
	}
	klog.Infof("resource updated[%s]: %s/%s", gvk.Kind, metav.GetNamespace(), metav.GetName())
	return object, os.WriteFile(filepath.Join(path, metav.GetName()), []byte(prettyYaml(object)), 0o755)
}

func (l *Local) Create(ctx context.Context, object runtime.Object, options *metav1.CreateOptions) (runtime.Object, error) {

	gvk := object.GetObjectKind().GroupVersionKind()
	if gvk.Empty() {
		return nil, service.NewUnknownObjectError(gvk.String())
	}
	path, err := mkPath(gvk)
	if err != nil {
		return object, err
	}
	metav, err := meta.Accessor(object)
	if err != nil {
		return object, err
	}
	klog.Infof("resource created[%s]: %s/%s", gvk.Kind, metav.GetNamespace(), metav.GetName())
	return object, os.WriteFile(filepath.Join(path, metav.GetName()), []byte(prettyYaml(object)), 0o755)
}

func (l *Local) Delete(ctx context.Context, object runtime.Object, options *metav1.DeleteOptions) (runtime.Object, error) {
	gvk := object.GetObjectKind().GroupVersionKind()
	if gvk.Empty() {
		return nil, service.NewUnknownObjectError(gvk.String())
	}
	path, err := mkPath(gvk)
	if err != nil {
		return object, err
	}
	metav, err := meta.Accessor(object)
	if err != nil {
		return object, err
	}
	if metav.GetName() == "" {
		return object, fmt.Errorf("unexpected empty object name")
	}
	klog.Infof("resource deleted: %s/%s", metav.GetNamespace(), metav.GetName())
	return object, os.Remove(filepath.Join(path, metav.GetName()))
}

func (l *Local) DeleteCollection(ctx context.Context, options *metav1.DeleteOptions) (runtime.Object, error) {
	//TODO implement me
	klog.Infof("local store called")
	return nil, nil
}

func (l *Local) Watch(ctx context.Context, in runtime.Object, options *metav1.ListOptions) (watch.Interface, error) {
	gvk := in.GetObjectKind().GroupVersionKind()
	if gvk.Empty() {
		return nil, service.NewUnknownObjectError(gvk.String())
	}
	path, err := mkPath(gvk)
	if err != nil {
		return nil, err
	}
	// Create new watcher.
	return NewFsWatcher(path, in)
}

func NewFsWatcher(path string, in runtime.Object) (watch.Interface, error) {
	// Create new watcher.
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	// Add a path.
	err = fsw.Add(path)
	if err != nil {
		_ = fsw.Close()
		return nil, err
	}
	watcher := fsWatcher{
		watcher:  fsw,
		object:   in,
		incoming: make(chan watch.Event),
	}
	return &watcher, nil
}

type fsWatcher struct {
	object   runtime.Object
	watcher  *fsnotify.Watcher
	incoming chan watch.Event
}

func (f *fsWatcher) read(name string) (runtime.Object, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("failed to read event file [%s]: %v", name, err)
	}
	obj := f.object.DeepCopyObject()
	err = yaml.Unmarshal(data, obj)
	return nil, err

}

func (f *fsWatcher) process() {
	for {
		select {
		case event, ok := <-f.watcher.Events:
			if !ok {
				continue
			}
			switch event.Op {
			case fsnotify.Write:
				obj, err := f.read(event.Name)
				if err != nil {
					klog.Errorf("failed to read event file [%s]: %v", event.Name, err)
					continue
				}
				f.incoming <- watch.Event{
					Type:   watch.Modified,
					Object: obj,
				}
			case fsnotify.Remove:
				f.incoming <- watch.Event{
					Type:   watch.Deleted,
					Object: f.object,
				}
			case fsnotify.Create:
				obj, err := f.read(event.Name)
				if err != nil {
					klog.Errorf("failed to read event file [%s]: %v", event.Name, err)
					continue
				}
				f.incoming <- watch.Event{
					Type:   watch.Added,
					Object: obj,
				}
			default:
				continue
			}
		case err, ok := <-f.watcher.Errors:
			if !ok {
				continue
			}
			klog.Errorf("fsWatcher error: %v", err)
		}
	}
}

func (f *fsWatcher) Stop() {
	err := f.watcher.Close()
	if err != nil {
		klog.Errorf("stop watcher: %v", err)
	}
	close(f.incoming)
}

func (f *fsWatcher) ResultChan() <-chan watch.Event {
	return f.incoming
}

func (l *Local) Destroy() {
	//TODO implement me
	klog.Infof("local store called")
}

func (l *Local) GVR() schema.GroupVersionResource {
	//TODO implement me
	klog.Infof("local store called")
	return schema.GroupVersionResource{}
}

func mkPath(resource schema.GroupVersionKind) (string, error) {
	root, err := model.MdHOME()
	if err != nil {
		return "", err
	}
	ppath := []string{
		root,
		"_registry",
		resource.Group,
		resource.Version,
		strings.Replace(strings.ToLower(resource.Kind), "list", "", 1),
	}
	path := filepath.Join(ppath...)
	return path, os.MkdirAll(path, 0755)
}

func prettyYaml(v interface{}) string {
	val, _ := yaml.Marshal(v)
	return string(val)
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
