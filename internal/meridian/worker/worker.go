package worker

import (
	"context"
	"fmt"
	"github.com/aoxn/meridian/internal/tool"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"math/rand"
	"strings"
	"sync"
	"time"
)

const DefaultQueueSize = 10

func NewWorkerMgr(ctx context.Context, name string, handle WorkerFunc, freeze *Action) (*WorkerMgr, error) {

	mwork := &WorkerMgr{
		ctx:    ctx,
		name:   name,
		handle: handle,
		worker: make(map[string]*thread),
		lock:   &sync.RWMutex{},
		freeze: freeze,
		queue:  NewDelayQueue(name, 0),
	}
	defer mwork.ResizeWorker(DefaultQueueSize)
	klog.Infof("worker manager name: %s", mwork.name)
	return mwork, mwork.runWorker(tool.RandomID(10))
}

func randInt(min int, max int) int {
	return min + rand.Intn(max-min)
}

type WorkerMgr struct {
	ctx     context.Context
	name    string
	enabled bool
	lock    *sync.RWMutex
	worker  map[string]*thread
	handle  WorkerFunc
	queue   DelayingInterface
	freeze  *Action
}

func (w *WorkerMgr) Name() string { return w.name }

func (w *WorkerMgr) ThreadCount() int { return len(w.worker) }

func (w *WorkerMgr) QueueLen() int { return w.queue.Len() }

// Reset 重置工作队列。向所有工作线程发送终止命令，
// 并将工作队列禁用，size设置为0。调用resize可以恢复工作队列
func (w *WorkerMgr) Reset() {
	w.lock.Lock()
	defer w.lock.Unlock()
	w.enabled = false
	klog.Infof("[%s]reset worker", w.name)
	for k, v := range w.worker {
		v.cancel()
		klog.Infof("[%s]Reset worker thread", w.id(k))
	}
	w.queue.ShutDown()
	// 清空已有队列。
	w.queue = NewDelayQueue(w.name, 0)
}

func (w *WorkerMgr) id(key string) string {
	return fmt.Sprintf("%s.%s", w.name, key)
}

func (w *WorkerMgr) Enqueue(item string) {

	enabled := func() bool {
		w.lock.Lock()
		defer w.lock.Unlock()
		return w.enabled
	}
	if !enabled() {
		klog.Infof("worker[%s] is not enabled", w.name)
		return
	}

	w.queue.Add(item)
}

func (w *WorkerMgr) EnqueueAfter(item string, du time.Duration) {

	enabled := func() bool {
		w.lock.Lock()
		defer w.lock.Unlock()
		return w.enabled
	}
	if !enabled() {
		klog.Infof("worker[%s] is not enabled", w.name)
		return
	}

	w.queue.AddAfter(item, du)
}

func (w *WorkerMgr) ResizeWorker(size int) {
	w.lock.Lock()
	defer w.lock.Unlock()
	if size < 0 {
		klog.Infof("unexpected negative size: %d", size)
		return
	}

	if size == 0 {
		w.enabled = false
	} else {
		w.enabled = true
	}

	diff := size - len(w.worker)
	w.queue.Resize(size)

	klog.Infof("[%s]WorkerMgr resize to %d", w.name, size)
	if diff > 0 {
		for i := 0; i < diff; i++ {
			key := tool.RandomID(10)
			err := w.runWorker(key)
			if err != nil {
				klog.Errorf("[%s]run new queue error: %s", w.id(key), err.Error())
			}
		}
		return
	}
	if diff < 0 {
		cnt := 0 - diff
		var keys []string
		for k, v := range w.worker {
			v.cancel()
			klog.Infof("[%s]cancel worker", w.id(k))
			cnt--
			keys = append(keys, k)
			if cnt <= 0 {
				break
			}
		}
		for _, k := range keys {
			delete(w.worker, k)
		}
		return
	}

}

type thread struct {
	id            string
	currentObject string
	currentCancel context.CancelFunc
	cancel        context.CancelFunc
}

type WorkerFunc func(ctx context.Context, req *Request, rep *Response) error

func (w *WorkerMgr) WorkerInfo() string {
	w.lock.RLock()
	defer w.lock.RUnlock()
	var info []string
	for _, v := range w.worker {
		name := "NoExecutionPlan"
		if v.currentObject != "" {
			name = v.currentObject
		}
		info = append(info, fmt.Sprintf("[%s:%s]", v.id, name))
	}
	return strings.Join(info, ", ")
}

func (w *WorkerMgr) CancelBy(name string) {
	w.lock.RLock()
	defer w.lock.RUnlock()
	var threads []*thread
	for _, th := range w.worker {
		if th.currentObject == name {
			threads = append(threads, th)
			continue
		}
	}
	cancel := func() {
		for _, th := range threads {
			th.currentCancel()
			klog.Infof("cancel thread: %s for %s", th.id, th.currentObject)
		}
	}
	go cancel()
}

func (w *WorkerMgr) runWorker(id string) error {
	if _, ok := w.worker[id]; ok {
		return fmt.Errorf("worker by uuid: %s already exist", id)
	}

	ctx, cancel := context.WithCancel(w.ctx)
	trd := &thread{id: id, cancel: cancel}
	poller := func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				delete(w.worker, id)
				return
			default:
				if w.freeze.Frozen() {
					time.Sleep(30 * time.Second)
					continue
				}
				o, shutdown := w.queue.Get()
				if shutdown {
					return
				}
				key, ok := o.(string)
				if !ok {
					return
				}
				trd.currentObject = key
				req := &Request{
					Key:       key,
					QueueName: w.name,
					WorkerID:  id,
				}
				mctx, mcancel := context.WithCancel(context.TODO())
				trd.currentCancel = mcancel
				err := w.handle(mctx, req, nil)
				if err != nil {
					select {
					case <-mctx.Done():
						w.queue.Done(o)
					default:
					}
					w.queue.Done(o)
					klog.Errorf("[%s]run task error, %s", w.id(id), err.Error())
					return
				}
				// finished
				trd.currentObject = ""
				trd.currentCancel = nil
				w.queue.Done(o)
				klog.Infof("[%s]==============================> [done: %s]\n\n", w.id(id), o)
				return
			}
		}
	}
	w.worker[id] = trd
	klog.V(5).Infof("[%s]start worker thread", w.id(id))
	go wait.UntilWithContext(ctx, poller, 5*time.Second)
	return nil
}
