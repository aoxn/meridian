package worker

import (
	"k8s.io/klog/v2"
	"sync"
)

func NewFreeze() *Action {
	return &Action{
		// mode step or running
		mode:        "running",
		lock:        &sync.RWMutex{},
		redispatch:  false,
		freezeState: false,
		event:       map[string]*Event{},
		notify:      make(chan string, 100),
	}
}

type Action struct {
	mode        string
	freezeState bool
	lock        *sync.RWMutex
	event       map[string]*Event
	redispatch  bool

	notify chan string
}

const (
	EV_PAUSE   = "pause"
	EV_RESUME  = "resume"
	EV_START   = "start"
	EV_CANCEL  = "cancel"
	EV_RESTART = "restart"

	EV_DELETE = "delete"

	EV_FIX_CVE = "fix"

	EV_FREEZE   = "freeze"
	EV_UNFREEZE = "unfreeze"
	EV_STATUS   = "status"

	EV_RESIZE = "resize"
	EV_MODE   = "mode"
)

type Event struct {
	// Action pause|resume|cancel
	Action string
	// State open|done
	State string
}

func (e *Event) Empty() bool {
	return e.Action == ""
}

func (f *Action) SetRedispatch(r bool) { f.redispatch = r }

func (f *Action) GetRedispatch() bool { return f.redispatch }

func (f *Action) SetMode(m string) { f.mode = m }

func (f *Action) GetMode() string { return f.mode }

func (f *Action) SingleStepMode() bool {
	return f.mode == "step"
}

func (f *Action) Freeze() {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.freezeState = true
}

func (f *Action) Unfreeze() {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.freezeState = false
}

func (f *Action) Frozen() bool { return f.freezeState }

func (f *Action) Event(plan, ev string) {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.event[plan] = &Event{Action: ev, State: "open"}
	klog.Infof("receive control event: user need plan[%s] in [%s] state", plan, ev)
	f.notify <- plan
}

func (f *Action) Watch() chan string { return f.notify }

func (f *Action) Events() map[string]*Event { return f.event }

func (f *Action) PickEvent(plan string) Event {
	f.lock.Lock()
	defer f.lock.Unlock()

	ev, ok := f.event[plan]
	if !ok {
		return Event{}
	}
	if ev.State != "open" {
		return Event{}
	}
	klog.Infof("pick event %s, state %s", ev.Action, ev.State)
	return *ev
}

func (f *Action) Done(plan, ev string) {
	f.lock.Lock()
	defer f.lock.Unlock()

	event, ok := f.event[plan]
	if !ok {
		klog.Infof("done event: no specified plan[%s] found. event=%s", plan, ev)
		return
	}
	if event.Action != ev {
		klog.Infof("done event: no specified event[%s] found for plan %s", ev, plan)
		return
	}
	event.State = "done"
	delete(f.event, plan)
	klog.Infof("done event: set event %s to state [done], plan %s", ev, plan)
}
