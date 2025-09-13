package hostagent

import (
	"context"
	"errors"
	"github.com/aoxn/meridian/internal/tool/downloader"
	"github.com/aoxn/meridian/internal/tool/server"
	"github.com/aoxn/meridian/internal/vmm/host/connectivity"
	"github.com/aoxn/meridian/internal/vmm/meta"
	"github.com/gorilla/mux"
	gerrors "github.com/pkg/errors"
	"github.com/samber/lo"
	"net/http"
	"path"
	//"errors"
	"fmt"
	"github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/tool"
	"github.com/aoxn/meridian/internal/vmm/forward"
	"github.com/aoxn/meridian/internal/vmm/host/event"
	"github.com/aoxn/meridian/internal/vmm/sshutil"
	"k8s.io/klog/v2"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aoxn/meridian/internal/vmm/backend"
	"github.com/aoxn/meridian/internal/vmm/backend/vz"
	"github.com/aoxn/meridian/internal/vmm/backend/wsl2"
	"github.com/aoxn/meridian/internal/vmm/cidata"
	"github.com/sethvargo/go-password/password"
)

type HostAgent struct {
	//vm      *model.Instance
	vmMeta   *meta.Machine
	ssh      *sshutil.SSHMgr
	bootDisk cidata.BootDisk
	driver   backend.Driver
	connect  *connectivity.Connectivity

	onClose []func() error // LIFO

	guestPort int
	signalCh  chan os.Signal

	clientMu sync.RWMutex

	guestAgentAliveCh     chan struct{} // closed on establishing the connection
	guestAgentAliveChOnce sync.Once
}

type options struct {
	nerdctlArchive string // local path, not URL
}

type Opt func(*options) error

// New creates the HostAgent.
//
// stdout is for emitting JSON lines of Events.
func New(vmMeta *meta.Machine, signal chan os.Signal, opts ...Opt) (*HostAgent, error) {
	var o options
	for _, f := range opts {
		if err := f(&o); err != nil {
			return nil, err
		}
	}

	if vmMeta == nil {
		return nil, errors.New("vmMeta is nil")
	}

	newBackend := func() backend.Driver {
		base := &backend.BaseDriver{
			I:          vmMeta,
			VSockPort:  10443,
			VirtioPort: "",
		}
		mt := base.I.Spec.VMType
		switch mt {
		case v1.VZ:
			return vz.New(base)
		case v1.WSL2:
			return wsl2.New(base)
		}
		return vz.New(base)
	}
	driver := newBackend()
	sshMgr := sshutil.NewSSHMgr("127.0.0.1", meta.Local.Config().Dir())
	var bootDisk cidata.BootDisk
	switch strings.ToLower(string(vmMeta.Spec.OS)) {
	case "darwin":
		bootDisk = cidata.NewPreBoot(vmMeta, sshMgr)
	default:
		bootDisk = cidata.NewCloudInit(vmMeta, sshMgr)
	}
	a := &HostAgent{
		vmMeta:            vmMeta,
		bootDisk:          bootDisk,
		connect:           connectivity.NewConnectivity(forward.NewForwardMgr(), driver, vmMeta),
		ssh:               sshMgr,
		signalCh:          signal,
		guestPort:         10443,
		driver:            driver,
		guestAgentAliveCh: make(chan struct{}),
	}
	return a, nil
}

func (ha *HostAgent) Run(ctx context.Context) error {
	klog.Infof("host agent: uid=%d", os.Getuid())
	defer func() {
		exitingEv := event.Event{
			Status: event.Status{
				Exiting: true,
			},
		}
		ha.emitEvent(ctx, exitingEv)
	}()
	vmInfo := ha.vmMeta.Spec

	err := ha.waitOnDisk()
	if err != nil {
		return err
	}
	err = ha.EnsureCIISO(ctx)
	if err != nil {
		return gerrors.Wrapf(err, "failed to ensure CI ISO")
	}
	errCh, err := ha.driver.Start(ctx)
	if err != nil {
		return err
	}

	// WSL instance SSH address isn't known until after vm start
	if vmInfo.VMType == v1.WSL2 {
		sshAddr, err := sshutil.GetSSHAddr(ha.vmMeta.Name, string(vmInfo.VMType))
		if err != nil {
			return err
		}
		ha.ssh.SetAddr(sshAddr)
	}
	go ha.Serve(ctx)
	go ha.connect.SetMappingRoute()

	if vmInfo.Video.Display == "vnc" {
		vncdisplay, vncoptions, _ := strings.Cut(vmInfo.Video.VNC.Display, ",")
		vnchost, vncnum, err := net.SplitHostPort(vncdisplay)
		if err != nil {
			return err
		}
		n, err := strconv.Atoi(vncnum)
		if err != nil {
			return err
		}
		vncport := strconv.Itoa(5900 + n)
		vncpwdfile := filepath.Join(ha.vmMeta.Dir(), v1.VNCPasswordFile)
		vncpasswd, err := generatePassword(8)
		if err != nil {
			return err
		}
		if err := ha.driver.ChangeDisplayPassword(ctx, vncpasswd); err != nil {
			return err
		}
		if err := os.WriteFile(vncpwdfile, []byte(vncpasswd), 0o600); err != nil {
			return err
		}
		if strings.Contains(vncoptions, "to=") {
			vncport, err = ha.driver.GetDisplayConnection(ctx)
			if err != nil {
				return err
			}
			p, err := strconv.Atoi(vncport)
			if err != nil {
				return err
			}
			vncnum = strconv.Itoa(p - 5900)
			vncdisplay = net.JoinHostPort(vnchost, vncnum)
		}
		vncfile := filepath.Join(ha.vmMeta.Dir(), v1.VNCDisplayFile)
		if err := os.WriteFile(vncfile, []byte(vncdisplay), 0o600); err != nil {
			return err
		}
		vncurl := "vnc://" + net.JoinHostPort(vnchost, vncport)
		klog.Infof("VNC server running at %s <%s>", vncdisplay, vncurl)
		klog.Infof("VNC Display: `%s`", vncfile)
		klog.Infof("VNC Password: `%s`", vncpwdfile)
	}

	if ha.driver.CanRunGUI() {
		go func() {
			err = ha.startRoutinesAndWait(ctx, errCh)
			if err != nil {
				klog.Errorf("start routine: %s", err)
			}
		}()
		return ha.driver.RunGUI()
	}
	return ha.startRoutinesAndWait(ctx, errCh)
}

func (ha *HostAgent) waitOnDisk() error {
	for i := 0; i < 20; i++ {
		if ha.vmMeta.StageUtil().Initialized() {
			return nil
		}
		time.Sleep(10 * time.Second)
	}
	return fmt.Errorf("wait disk timeout: %ds", 200)
}

func (ha *HostAgent) GenDisk(ctx context.Context) error {
	err := ha.EnsureCIISO(ctx)
	if err != nil {
		return gerrors.Wrapf(err, "generate cloud-init iso image")
	}
	return ha.driver.CreateDisk(ctx)
}

func (ha *HostAgent) EnsureCIISO(ctx context.Context) error {
	vmInfo := ha.vmMeta.Spec

	extracted := path.Join(ha.vmMeta.Dir(), "bin")

	if _, err := os.Stat(extracted); os.IsNotExist(err) {
		guestBin := filepath.Join(ha.vmMeta.Dir(), v1.GuestBin)
		klog.Infof("guest not found: %s", extracted)
		f := v1.FindGuestBin(vmInfo.GuestVersion, string(vmInfo.OS), string(vmInfo.Arch))
		if f == nil {
			return fmt.Errorf("guest binary %s not found by arch: %s", guestBin, vmInfo.Arch)
		}
		if f.Arch != vmInfo.Arch {
			return fmt.Errorf("%q: unsupported arch: %q, expected=%q", f.Location, f.Arch, vmInfo.Arch)
		}
		res, err := downloader.Download(ctx, guestBin, f.Location,
			downloader.WithCache(),
			downloader.WithDecompress(true),
			downloader.WithDescription(fmt.Sprintf("%s (%s)", "guest binary", path.Base(f.Location))),
			downloader.WithExpectedDigest(f.Digest),
		)
		if err != nil {
			return fmt.Errorf("failed to download %q: %w", f.Location, err)
		}
		klog.Infof("download guest binary: [%s][%s][%s]", f.Arch, f.Location, res.Status)
	}

	return ha.bootDisk.CreateBootDisk()
}

func (ha *HostAgent) startRoutinesAndWait(ctx context.Context, errCh chan error) error {
	stBase := event.Status{
		SSHLocalPort: ha.ssh.GetPort(),
	}
	stBooting := stBase
	ha.emitEvent(ctx, event.Event{Status: stBooting})
	go func() {
		err := ha.connect.ForwardMachine(ctx, ha.vmMeta)
		if err != nil {
			klog.Errorf("failed to forward vm to host agent: %v", err)
		}
		stRunning := stBase
		if haErr := ha.startHostAgentRoutines(ctx); haErr != nil {
			stRunning.Degraded = true
			stRunning.Errors = append(stRunning.Errors, haErr.Error())
		}
		stRunning.Running = true
		ha.emitEvent(ctx, event.Event{Status: stRunning})
	}()
	defer ha.vmMeta.RemovePID()
	for {
		select {
		case driverErr := <-errCh:
			klog.Infof("Driver stopped due to error: %q", driverErr)
			if closeErr := ha.close(); closeErr != nil {
				klog.Errorf("an error during shutting down the host agent: %s", closeErr)
			}
			return driverErr
		case <-ha.signalCh:
			klog.Info("context canceled, shutting down the host agent")
			if closeErr := ha.close(); closeErr != nil {
				klog.Errorf("an error during shutting down the host agent: %s", closeErr)
			}
			err := ha.driver.Stop(ctx)
			return err
		}
	}
}

func (ha *HostAgent) startHostAgentRoutines(ctx context.Context) error {

	ha.onClose = append(ha.onClose, func() error {
		klog.Info("shutting down the SSH master")
		//cfg, err := ha.ssh.SSHConfig(ha.vm.Dir)
		//if err != nil {
		//	return err
		//}
		//if exitMasterErr := ssh.ExitMaster(ha.ssh.GetAddr(), ha.ssh.GetPort(), cfg); exitMasterErr != nil {
		//	klog.WithError(exitMasterErr).Warn("failed to exit SSH master")
		//}
		return nil
	})

	go ha.watchGuestAgentEvents(ctx)

	klog.Info("Waiting for the guest agent to be running")
	select {
	case <-ha.guestAgentAliveCh:
		// NOP
	case <-time.After(time.Minute):
		return errors.New("guest agent does not seem to be running; port forwards will not work")
	}
	klog.Infof("guest agent is running")
	return nil
}

func (ha *HostAgent) close() error {
	klog.Info("Shutting down the host agent")
	var errs []error
	for i := len(ha.onClose) - 1; i >= 0; i-- {
		f := ha.onClose[i]
		if err := f(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (ha *HostAgent) watchGuestAgentEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Second):
		}
	}
}

func (ha *HostAgent) emitEvent(_ context.Context, ev event.Event) {

}

func generatePassword(length int) (string, error) {
	// avoid any special symbols, to make it easier to copy/paste
	return password.Generate(length, length/4, 0, false, false)
}

func (ha *HostAgent) Serve(ctx context.Context) {
	var sock = ha.vmMeta.SandboxSock()
	_ = os.RemoveAll(sock)
	var (
		cfg = &server.Config{
			Vsock:    false,
			BindAddr: sock,
		}
	)
	klog.Infof("serve sandbox: uid=%d, on [%s]", os.Getuid(), sock)

	sandbox := &sandboxHandler{host: ha}
	damon := server.NewOrDie(
		context.TODO(), cfg,
		map[string]map[string]server.HandlerFunc{},
	)
	damon.AddRoute(map[string]map[string]server.HandlerFunc{
		"GET": {
			"/healthz": func(r *http.Request, w http.ResponseWriter) int {
				data := tool.PrettyJson(v1.Healthy{Status: "ok"})
				_, _ = w.Write([]byte(data))
				return http.StatusOK
			},
		},
		"POST": {
			"/api/v1/forward/{name}": sandbox.Forward,
		},
		"DELETE": {
			"/api/v1/forward/{name}": sandbox.RemoveForward,
		},
		"PUT": {
			"/api/v1/vm/stop/{name}": sandbox.StopVm,
		},
	})
	err := damon.Start(ctx)
	if err != nil {
		klog.Errorf("failed to start the damon: %s", err)
	}
	select {}
}

type sandboxHandler struct {
	host *HostAgent
}

func (sbx *sandboxHandler) Forward(r *http.Request, w http.ResponseWriter) int {
	name := mux.Vars(r)["name"]
	switch name {
	case "":
		return server.HttpJson(w, fmt.Errorf("unexpected empty name"))
	default:
	}
	klog.Infof("forwarding %s", name)
	var spec []v1.PortForward
	err := server.DecodeBody(r.Body, &spec)
	if err != nil {
		return server.HttpJson(w, err)
	}
	klog.Infof("start forwarding [%s]: %s", name, lo.Map(spec, func(item v1.PortForward, index int) string {
		return item.Rule()
	}))
	err = sbx.host.connect.Forward(r.Context(), spec)
	if err != nil {
		return server.HttpJson(w, err)
	}
	return server.HttpJson(w, spec)
}

func (sbx *sandboxHandler) RemoveForward(r *http.Request, w http.ResponseWriter) int {
	name := mux.Vars(r)["name"]
	switch name {
	case "":
		return server.HttpJson(w, fmt.Errorf("unexpected empty name"))
	default:
	}
	var spec []v1.PortForward
	err := server.DecodeBody(r.Body, &spec)
	if err != nil {
		return server.HttpJson(w, err)
	}
	klog.Infof("remove forward entry: %s", lo.Map(spec, func(item v1.PortForward, index int) string {
		return item.Rule()
	}))
	err = sbx.host.connect.Remove(spec)
	if err != nil {
		return server.HttpJson(w, err)
	}
	return server.HttpJson(w, spec)
}

func (sbx *sandboxHandler) StopVm(r *http.Request, w http.ResponseWriter) int {
	name := mux.Vars(r)["name"]
	switch name {
	case "":
		return server.HttpJson(w, fmt.Errorf("unexpected empty name"))
	default:
	}
	klog.Infof("sandbox: receieve vm stop request, %s", name)
	go func() {
		klog.Infof("sandbox: tring to stop vm %s", name)

		err := sbx.host.driver.Stop(context.TODO())
		if err != nil {
			klog.Errorf("sandbox: stop vm error, %s", err.Error())
		}
	}()
	return server.HttpJson(w, "Accepted")
}
