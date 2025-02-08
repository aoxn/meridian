package task

import (
	"context"
	"fmt"
	"k8s.io/klog/v2"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"

	v1 "github.com/aoxn/meridian/api/v1"
	w "github.com/aoxn/meridian/internal/meridian/worker"
	"github.com/aoxn/meridian/internal/server/service"
	"github.com/aoxn/meridian/internal/server/service/universal"
)

func NewTaskPvd(opt *service.Options) service.Provider {
	return &taskPvd{options: opt}
}

type taskPvd struct {
	options *service.Options
}

func (m *taskPvd) NewAPIGroup(ctx context.Context) (service.Grouped, error) {
	grp := service.Grouped{}
	t := m.addV1(grp, m.options)
	return grp, m.initPrvd(ctx, t)
}

func (m *taskPvd) addV1(grp service.Grouped, options *service.Options) *task {
	univ := &task{
		Store:   universal.NewUniversal(options),
		scheme:  options.Scheme,
		freezer: w.NewFreeze(),
		// allowedResource
		allowedResource: sets.New[string](),
	}
	grp.AddOrDie(univ)
	return univ
}

func (m *taskPvd) initPrvd(ctx context.Context, t *task) error {
	work, err := w.NewWorkerMgr(ctx, "meridian", t.HandleTask, t.freezer)
	if err != nil {
		return err
	}
	t.work = work

	klog.Infof("task worker started")

	return t.InitTask()
}

type task struct {
	service.Store
	work            *w.WorkerMgr
	freezer         *w.Action
	scheme          *runtime.Scheme
	allowedResource sets.Set[string]
}

func (m *task) GVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    v1.GroupVersion.Group,
		Version:  v1.GroupVersion.Version,
		Resource: "tasks",
	}
}

func (m *task) isAllowed(r string) bool {
	r = strings.ToLower(r)
	if m.allowedResource.Len() == 0 {
		return true
	}
	return m.allowedResource.Has(r)
}

func (m *task) Update(ctx context.Context, object runtime.Object, options *metav1.UpdateOptions) (runtime.Object, error) {
	if object == nil {
		return nil, service.NewUnknownObjectError("")
	}
	gvk := object.GetObjectKind().GroupVersionKind()
	if !m.isAllowed(gvk.Kind) {
		return nil, service.NewNotAllowedError(gvk.String())
	}
	return m.Store.Update(ctx, object, options)
}

func (m *task) Create(ctx context.Context, object runtime.Object, options *metav1.CreateOptions) (runtime.Object, error) {
	if object == nil {
		return nil, service.NewUnknownObjectError("")
	}
	gvk := object.GetObjectKind().GroupVersionKind()
	if !m.isAllowed(gvk.Kind) {
		return nil, service.NewNotAllowedError(gvk.String())
	}
	return m.Store.Create(ctx, object, options)
}

func (m *task) Delete(ctx context.Context, object runtime.Object, options *metav1.DeleteOptions) (runtime.Object, error) {
	if object == nil {
		return nil, service.NewUnknownObjectError("")
	}
	gvk := object.GetObjectKind().GroupVersionKind()
	if !m.isAllowed(gvk.Kind) {
		return nil, service.NewNotAllowedError(gvk.String())
	}
	return m.Store.Delete(ctx, object, options)
}

func (m *task) DeleteCollection(ctx context.Context, options *metav1.DeleteOptions) (runtime.Object, error) {

	return m.Store.DeleteCollection(ctx, options)
}

func (m *task) InitTask() error {
	o := v1.TaskList{}
	_, err := m.List(context.TODO(), &o, nil)
	if err != nil {
		return err
	}
	for i, _ := range o.Items {
		task := &o.Items[i]
		switch task.Status.Phase {
		case v1.TaskFail, v1.TaskSuccess:
			klog.Infof("skip [%s] task", task.Status.Phase)
			continue
		default:
		}
		klog.Infof("send init task: %+v", task.Name)
		_ = m.SendTask(task)
	}
	return nil
}

func (m *task) SendTask(task *v1.Task) error {
	m.work.Enqueue(task.Name)
	return nil
}

func (m *task) HandleTask(ctx context.Context, req *w.Request, rep *w.Response) error {

	var (
		err     error
		cluster = &v1.Cluster{}
		t       = emptyTask(req.Key)
	)
	_, err = m.Store.Get(ctx, t, nil)
	if err != nil {
		return err
	}
	switch t.Spec.Type {
	case v1.ResourceKindVM:
		return m.handleVm(ctx, t)
	case v1.ResourceKindCluster:
		return fmt.Errorf("unimplemented")
	default:
		klog.Infof("handler t: %s, %s", req.Key, cluster.Name)
	}
	return fmt.Errorf("unimplemented")
}

func (m *task) handleVm(ctx context.Context, t *v1.Task) error {

	var (
		err     error
		cluster = &v1.Cluster{}
	)
	cluster.Name = t.Spec.ClusterName
	_, err = m.Store.Get(ctx, cluster, nil)
	if err != nil {
		return err
	}

	return err
}

func emptyTask(k string) *v1.Task {
	return &v1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: k},
	}
}
