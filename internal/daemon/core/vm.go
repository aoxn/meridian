package core

import (
	"context"
	"fmt"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/client"
	"github.com/aoxn/meridian/internal/tool"
	hostagent "github.com/aoxn/meridian/internal/vmm/host"
	"github.com/aoxn/meridian/internal/vmm/meta"
	"github.com/aoxn/meridian/internal/vmm/sshutil"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

func NewContext() (*Context, error) {
	backend, err := meta.NewLocal()
	if err != nil {
		return nil, err
	}
	vmMgr, err := NewLocalVMMgr(backend)
	if err != nil {
		return nil, err
	}
	dockerMgr, err := NewLocalDockerMgr(vmMgr.stateMgr)
	if err != nil {
		return nil, err
	}
	k8sMgr, err := NewK8sMgr(vmMgr.stateMgr)
	return &Context{
		meta:      backend,
		vmMgr:     vmMgr,
		imageMgr:  vmMgr.imgMgr,
		dockerMgr: dockerMgr,
		k8sMgr:    k8sMgr,
	}, nil
}

type Context struct {
	meta      meta.Backend
	vmMgr     *LocalVMMgr
	imageMgr  *LocalImageMgr
	dockerMgr *LocalDockerMgr
	k8sMgr    *LocalK8sMgr
}

func (ctx *Context) Backend() meta.Backend { return ctx.meta }

func (ctx *Context) VMMgr() *LocalVMMgr { return ctx.vmMgr }

func (ctx *Context) ImageMgr() *LocalImageMgr { return ctx.imageMgr }

func (ctx *Context) DockerMgr() *LocalDockerMgr { return ctx.dockerMgr }

func (ctx *Context) K8sMgr() *LocalK8sMgr { return ctx.k8sMgr }

func NewLocalVMMgr(backend meta.Backend) (*LocalVMMgr, error) {
	stateMgr, err := newVMStateMgr(backend)
	if err != nil {
		return nil, err
	}
	local := &LocalVMMgr{
		backend:  backend,
		tskMgr:   newTaskMgr(),
		stateMgr: stateMgr,
		imgMgr:   NewLocalImageMgr(backend),
	}
	go local.periodical()
	return local, nil
}

type LocalVMMgr struct {
	backend  meta.Backend
	tskMgr   *taskMgr
	stateMgr *vmStateMgr
	imgMgr   *LocalImageMgr
}

func (mgr *LocalVMMgr) periodical() {
	tickFn := func() {
		for _, m := range mgr.stateMgr.vms {
			if m.machine.StageUtil().Initialized() {
				continue
			}
			err := mgr.tskMgr.Send(InitializeVM, m.name, func(ctx context.Context) error {
				return mgr.initialVm(ctx, m)
			})
			if err != nil {
				klog.Errorf("periodical initialize-vm err: %v", err)
			}
		}
	}
	wait.Until(func() {
		tick := time.NewTicker(10 * time.Second)
		for {
			select {
			case <-tick.C:
				tickFn()
			}
		}
	}, 10*time.Second, make(<-chan struct{}))
}

func (mgr *LocalVMMgr) Create(ctx context.Context, vm *meta.Machine) error {
	state := mgr.stateMgr.Get(vm.Name)
	if state != nil && state.machine != nil {
		return fmt.Errorf("AlreadyExist: %s exist", vm.Name)
	}

	err := vm.SetDefault()
	if err != nil {
		return errors.Wrapf(err, "set default machine value: %s", vm.Name)
	}
	err = allocateAddress(vm, mgr.stateMgr.List())
	if err != nil {
		return errors.Wrapf(err, "allocate machine address")
	}

	klog.V(5).Infof("debug create machine %s: %s", vm.Name, tool.PrettyJson(vm))
	state, err = mgr.stateMgr.Create(vm)
	if err != nil {
		return errors.Wrapf(err, "create machine %s", vm.Name)
	}

	return mgr.tskMgr.Send(InitializeVM, vm.Name, func(ctx context.Context) error {
		klog.Infof("try initialize vm: %s", vm.Name)
		return mgr.initialVm(ctx, state)
	})
}

func (mgr *LocalVMMgr) Run(ctx context.Context, name string, vm *meta.Machine) error {
	err := mgr.Create(ctx, vm)
	if err != nil {
		return errors.Wrapf(err, "create machine %s", vm.Name)
	}
	state := mgr.stateMgr.Get(vm.Name)
	if state == nil || state.machine == nil {
		return fmt.Errorf("unexpected MachineNotFound: %s", vm.Name)
	}

	switch state.machine.State {
	case Running, Stopping, Starting:
		return fmt.Errorf("UnexpectedCurrentState: %s=[%s], can not run", name, state.machine.State)
	case Created, Error, Stopped:
		// can start
		err := state.runVm(context.TODO())
		if err != nil {
			return err
		}
	default:
		klog.Infof("[%10s]unkown machine state: %s, skipped", state.name, state.machine.State)
	}
	return nil
}

func (mgr *LocalVMMgr) Stop(ctx context.Context, name string) error {
	vm := mgr.stateMgr.Get(name)
	if vm == nil {
		return fmt.Errorf("vm %s not exist", name)
	}
	switch vm.machine.State {
	case Created, Stopped:
		return nil
	case Running, Starting, Stopping, Error:
		err := vm.stopVm(ctx)
		if err != nil {
			return errors.Wrapf(err, "stop vm %s", name)
		}
	default:
		klog.Infof("[%10s]unkown machine state: %s, skipped", vm.name, vm.machine.State)
	}
	return nil
}

func (mgr *LocalVMMgr) Start(ctx context.Context, name string) error {
	vm := mgr.stateMgr.Get(name)
	if vm == nil {
		return fmt.Errorf("vm %s not exist", name)
	}
	switch vm.machine.State {
	case Running, Stopping, Starting:
		_, err := vm.machine.LoadPID()
		if err != nil {
			if !strings.Contains(err.Error(), "NotFound") {
				return errors.Wrapf(err, "find vm by pid")
			}
			// not found, start
			return vm.runVm(context.TODO())
		}
		return fmt.Errorf("UnexpectedCurrentState: %s=[%s], can not run", name, vm.machine.State)
	case Created, Error, Stopped:
		// can start
		err := vm.runVm(context.TODO())
		if err != nil {
			return errors.Wrapf(err, "start vm %s", name)
		}
	default:
		klog.Infof("[%10s]unkown machine state: %s, skipped", vm.name, vm.machine.State)
	}
	return nil
}

func (mgr *LocalVMMgr) Destroy(ctx context.Context, name string) error {
	vm := mgr.stateMgr.Get(name)
	if vm == nil {
		return nil
	}
	// if vm still in Created state, should cancel vm initialization first
	if vm.machine.State == Created {
		err := mgr.tskMgr.Terminate(ctx, InitializeVM, name)
		if err != nil {
			return errors.Wrapf(err, "terminate vm initialization process")
		}
	}
	err := vm.stopVm(ctx)
	if err != nil {
		return errors.Wrapf(err, "destroy vm %s", name)
	}
	err = vm.machine.Destroy(ctx)
	if err != nil {
		return errors.Wrapf(err, "destroy machine %s", name)
	}
	mgr.stateMgr.Delete(vm.name)
	return nil
}

func (mgr *LocalVMMgr) RunCommand(name string, cmd Command) error {
	panic("not implement")
}

func (mgr *LocalVMMgr) initialVm(ctx context.Context, state *vmState) error {
	klog.Infof("current stage: %s", state.machine.StageUtil().Get())
	if state.machine.StageUtil().Initialized() {
		return nil
	}
	var (
		vm  = state.machine
		img = &meta.Image{
			Name: vm.Spec.Image.Name,
		}
	)
	_ = vm.StageUtil().Set(meta.StageInitializing)
	defer mgr.backend.Machine().Update(vm)
	klog.Infof("start to initialize vm: %s", vm.Name)
	state.restStage(StatePulling, "pulling image: [%s]", img.Name)
	pull, err := mgr.imgMgr.Pull(img.Name)
	if err != nil {
		state.addStage(Error, "pull image error: [%s], %s", img.Name, err.Error())
		return fmt.Errorf("[%s]pull image %s failed: %v", vm.Name, img.Name, err)
	}
	// todo wait image pull
	err = pull.Wait(ctx)
	if err != nil && !strings.Contains(err.Error(), "PullComplete") {
		return errors.Wrapf(err, "wait for image pulling")
	}
	host, err := hostagent.New(vm, nil)
	if err != nil {
		return err
	}
	state.addStage(StatePulled, "image pulled")
	state.addStage(PrepareDisk, "prepare base disk: [%s]", "diff")
	err = host.GenDisk(ctx)
	if err != nil {
		state.addStage(Error, "prepare disk error: %s, %s", vm.Name, err.Error())
		return errors.Wrapf(err, "gen disk machine %s failed", vm.Name)
	}
	state.addStage(DiskPrepared, "disk prepared")
	err = vm.StageUtil().Set(meta.StageInitialized)
	if err != nil {
		return errors.Wrapf(err, "set stage %s", meta.StageInitialized)
	}
	return lo.Ternary(state.nextAction != Starting, nil, state.runVm(ctx))
}

func newVMStateMgr(bk meta.Backend) (*vmStateMgr, error) {
	machines, err := bk.Machine().List()
	if err != nil {
		return nil, errors.Wrap(err, "read machine config error")
	}
	var vms = make(map[string]*vmState)
	for _, vm := range machines {
		vms[vm.Name] = &vmState{
			name:    vm.Name,
			machine: vm,
			meta:    bk,
			mu:      &sync.RWMutex{},
		}
	}
	return &vmStateMgr{mu: &sync.RWMutex{}, vms: vms, meta: bk}, nil
}

type vmStateMgr struct {
	mu   *sync.RWMutex
	vms  map[string]*vmState
	meta meta.Backend
}

type vmState struct {
	name       string
	starting   bool
	nextAction string // Start
	mu         *sync.RWMutex
	machine    *meta.Machine
	meta       meta.Backend
	cancelFn   context.CancelFunc
}

const (
	Unknown   = "UnKnown"
	Created   = "Created"
	Running   = "Running"
	Deploying = "Deploying"
	Stopping  = "Stopping"
	Stopped   = "Stopped"
	Starting  = "Starting"
	Error     = "Error"
)
const (
	StatePulling = "PullingImage"
	StatePulled  = "ImagePulled"
	StateFailed  = "Failed"
	PrepareDisk  = "PrepareDisk"
	DiskPrepared = "DiskPrepared"
)

func (mgr *vmStateMgr) Get(name string) *vmState {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	return mgr.vms[name]
}

func (mgr *vmStateMgr) List() []*meta.Machine {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	transFn := func(key string, value *vmState) *meta.Machine {
		return value.machine
	}
	return lo.MapToSlice(mgr.vms, transFn)
}

func (mgr *vmStateMgr) Delete(name string) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	delete(mgr.vms, name)
}

func (mgr *vmStateMgr) Create(vm *meta.Machine) (*vmState, error) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	_, ok := mgr.vms[vm.Name]
	if ok {
		return nil, fmt.Errorf("vm %s already exists", vm.Name)
	}
	vm.State, vm.Message = Created, fmt.Sprintf("machine %s created", vm.Name)
	state := &vmState{
		name:    vm.Name,
		machine: vm,
		meta:    mgr.meta,
		mu:      &sync.RWMutex{},
	}
	mgr.vms[vm.Name] = state
	err := mgr.meta.Machine().Create(vm)
	return state, err
}

func fmtMessage(msg ...any) string {
	var description string
	switch len(msg) {
	case 0:
	case 1:
		description = fmt.Sprintf("%s", msg[0])
	default:
		description = fmt.Sprintf(fmt.Sprintf("%s", msg[0]), msg[1:]...)
	}
	return description
}

func (m *vmState) setState(state string, msg ...any) {

	m.machine.State, m.machine.Message = state, fmtMessage(msg...)
	err := m.meta.Machine().Update(m.machine)
	if err != nil {
		klog.Errorf("update machine %s state failed: %v", m.machine.Name, err)
	}
}

func (m *vmState) restStage(phase string, msg ...any) {

	stage := meta.Stage{
		Phase:       phase,
		Timestamp:   time.Now(),
		Description: fmtMessage(msg...),
	}
	m.machine.Stage = []meta.Stage{stage}
}

func (m *vmState) addStage(phase string, msg ...any) {

	stage := meta.Stage{
		Phase:       phase,
		Timestamp:   time.Now(),
		Description: fmtMessage(msg...),
	}
	m.machine.Stage = append(m.machine.Stage, stage)
}

func (m *vmState) SSH() *sshutil.SSHMgr {
	n := lo.FirstOr(m.machine.Spec.Networks, v1.Network{})

	return sshutil.NewSSHMgr(strings.Split(n.Address, "/")[0], m.meta.Config().Dir())
}

func (m *vmState) stopVm(ctx context.Context) error {

	get := func() bool {
		m.mu.RLock()
		defer m.mu.RUnlock()
		return m.starting
	}

	if get() {
		if m.cancelFn != nil {
			m.cancelFn()
		}
	}
	klog.Infof("[%s]waiting for vm steady ", m.machine.Name)
	// wait starting false
	for i := 0; i < 600; i++ {
		if i > 598 {
			return fmt.Errorf("timeout wait for stop vm: %s", m.name)
		}
		if !get() {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.setState(Stopping, "stop vm")
	sdbx, err := client.Client(m.machine.SandboxSock())
	if err != nil {
		return errors.Wrapf(err, "new sandbox client")
	}

	finished := make(chan error)

	ctx, cancelFn := context.WithCancel(ctx)
	go func(ctx context.Context) {
		err = sdbx.Update(ctx, "vm/stop", m.name, m.machine)
		if err != nil {
			klog.Infof("stop remote host-vm: %s", err.Error())
		}
		// wait for pid gone
		err = m.machine.WaitStop(ctx, 3*time.Minute)
		finished <- err
	}(ctx)

	defer cancelFn()

	select {
	case <-time.After(30 * time.Second):
		m.setState(Error, "timeout 30s")
		return fmt.Errorf("timeout after 30s seconds wait for vm stop: %s", m.name)
	case <-ctx.Done():
		m.setState(Error, "stop vm: context done")
		return fmt.Errorf("context done: %s, %s", m.machine.Name, ctx.Err())
	case errMsg := <-finished:
		klog.Infof("vm stopped(call sandbox stop): %s", m.machine.Name)
		if errMsg != nil {
			m.setState(Error, "stop vm: %s", errMsg.Error())
			return fmt.Errorf("vm stop err: %s", errMsg.Error())
		}
	}
	err = m.machine.Stop(ctx)
	if err != nil {
		m.setState(Error, "stop vm error: %s", err.Error())
		return err
	}
	m.setState(Stopped, "vm stopped")
	return nil
}

func (m *vmState) runVm(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.machine.StageUtil().Initialized() {
		m.nextAction = Starting
		return fmt.Errorf("vm %s is still initializing", m.machine.Name)
	}
	if m.starting {
		return fmt.Errorf("vm [%s] already in starting", m.name)
	}
	klog.V(5).Infof("[%s]run vm", m.machine.Name)
	gaClient, err := client.Client(m.machine.SandboxSock())
	if err != nil {
		return errors.Wrap(err, "get guest client error")
	}

	m.starting = true

	ctx, m.cancelFn = context.WithCancel(ctx)

	err = m.run(context.TODO())
	if err != nil {
		m.starting = false
		m.setState(Error, "vm start with error: %s", m.name)
		return err
	}

	go m.waitVm(ctx, gaClient)
	return nil
}

func (m *vmState) run(ctx context.Context) error {
	var (
		begin = time.Now() // used for logrus propagation
		vm    = m.machine
	)

	m.setState(Starting, "starting vm: %s", vm.Name)

	pid, err := vm.LoadPID()
	if err != nil {
		// pid not exist
		klog.Infof("[%-10s]read pid failed: [%s], %s", vm.Name, vm.PIDFile(), err.Error())
		p, err := m.runDaemon(vm)
		if err != nil {
			return err
		}
		klog.Infof("[%-10s]virtual machine started: %d", vm.Name, p)
	} else {
		klog.Infof("[%-10s] pid exist, [%d]", vm.Name, pid)
	}

	klog.Infof("[%-10s]vm started in %f(m)", vm.Name, time.Now().Sub(begin).Seconds())
	return nil
}

func (m *vmState) runDaemon(vm *meta.Machine) (int, error) {
	_ = os.MkdirAll(vm.Dir(), 0o700)
	vmBin, err := vmBinaryPath()
	if err != nil {
		return 0, err
	}
	klog.Infof("boot vm from: %s", vmBin)
	var cfg = path.Join(vm.Dir(), "config.mirror.yml")
	err = os.WriteFile(
		cfg, []byte(tool.PrettyYaml(vm)), 0755)
	if err != nil {
		return 0, err
	}
	var args = []string{"start", vm.Name, "-v", "6", "-c", cfg}
	haCmd := exec.CommandContext(context.TODO(), vmBin, args...)
	haCmd.Stdin = strings.NewReader(tool.PrettyYaml(vm))
	haCmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
	err = haCmd.Start()
	if err != nil {
		return 0, errors.Wrapf(err, "start vm %s error", vm.Name)
	}

	klog.Infof("[%-10s]start vm: [%d], %s", vm.Name, os.Getpid(), strings.Join(append([]string{vmBin}, args...), " "))
	return 0, haCmd.Wait()
}

func (m *vmState) waitVm(ctx context.Context, gaClient client.Interface) {
	set := func(v bool) {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.starting = v
	}
	defer set(false)

	err := wait.PollUntilContextTimeout(
		ctx, 3*time.Second,
		2*time.Minute, false,
		func(ctx context.Context) (bool, error) {
			err := gaClient.Healthz(ctx)
			if err != nil {
				klog.V(6).Infof("[%-10s]wait host agent start: %v", m.name, err)
				return false, nil
			}
			return true, nil
		},
	)
	if err == nil {
		m.setState(Running, "vm is now running: %s", m.name)
		return
	}
	m.setState(Error, "vm[%s] start with error: %s", m.name, err.Error())
}

func vmBinaryPath() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	link, err := filepath.EvalSymlinks(self)
	if err != nil {
		return "", err
	}
	return path.Join(path.Dir(link), "meridian-vm"), nil
}

type Command struct {
}

const (
	InitializeVM = "initialize-vm"
)

func newTaskMgr() *taskMgr {
	mgr := &taskMgr{
		mu:     &sync.RWMutex{},
		notify: make(chan string),
		tasks:  make(map[string]context.CancelFunc),
	}
	go mgr.loop()
	return mgr
}

type taskMgr struct {
	mu     *sync.RWMutex
	notify chan string
	tasks  map[string]context.CancelFunc
	class  map[string]func(ctx context.Context, state *vmState) error
}

func (mgr *taskMgr) loop() {
	clean := func(name string) {
		mgr.mu.Lock()
		defer mgr.mu.Unlock()
		delete(mgr.tasks, name)
	}
	for {
		select {
		case key := <-mgr.notify:
			clean(key)
		}
	}
}

type tskFn func(ctx context.Context) error

func (mgr *taskMgr) Send(class string, name string, tskFn tskFn) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	var key = fmt.Sprintf("%s-%s", class, name)
	_, ok := mgr.tasks[key]
	if ok {
		return fmt.Errorf("task already exists: %s", key)
	}
	ctx, cancelFn := context.WithCancel(context.TODO())
	mgr.tasks[key] = cancelFn
	go func(ctx context.Context, key string) {
		err := tskFn(ctx)
		if err != nil {
			klog.Errorf("[%s]run task error: %s", key, err.Error())
		}
		mgr.notify <- key
	}(ctx, key)
	return nil
}

func (mgr *taskMgr) Terminate(ctx context.Context, class string, name string) error {
	var cancelCtx = func() {
		mgr.mu.Lock()
		defer mgr.mu.Unlock()
		var key = fmt.Sprintf("%s-%s", class, name)
		cancelFn, ok := mgr.tasks[key]
		if !ok {
			return
		}
		cancelFn()
	}

	cancelCtx()

	var terminated = func() bool {
		mgr.mu.Lock()
		defer mgr.mu.Unlock()
		var key = fmt.Sprintf("%s-%s", class, name)
		_, ok := mgr.tasks[key]
		return !ok
	}

	tick := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context canceld: %s/%s", class, name)
		case <-tick.C:
		}
		if terminated() {
			return nil
		}
	}
}
