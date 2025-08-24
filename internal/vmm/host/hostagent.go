package hostagent

import (
	"context"
	"errors"
	"github.com/aoxn/meridian/internal/server"
	"github.com/aoxn/meridian/internal/tool/downloader"
	"github.com/aoxn/meridian/internal/vmm/host/connectivity"
	"github.com/aoxn/meridian/internal/vmm/meta"
	"net/http"
	"path"

	//"errors"
	"fmt"
	"github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/tool"
	"github.com/aoxn/meridian/internal/vmm/forward"
	"github.com/aoxn/meridian/internal/vmm/host/event"
	"github.com/aoxn/meridian/internal/vmm/sshutil"
	"io"
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
	vmMeta  *meta.Machine
	ssh     *sshutil.SSHMgr
	ci      *cidata.CloudInit
	fwd     *forward.ForwardMgr
	connect *connectivity.Connectivity

	driver backend.Driver

	onClose []func() error // LIFO

	guestPort int

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
func New(vmMeta *meta.Machine, stdout io.Writer, opts ...Opt) (*HostAgent, error) {
	var o options
	for _, f := range opts {
		if err := f(&o); err != nil {
			return nil, err
		}
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
	sshMgr := sshutil.NewSSHMgr(vmMeta.Name, "127.0.0.1", 6022)
	a := &HostAgent{
		vmMeta:            vmMeta,
		ci:                cidata.NewCloudInit(vmMeta, sshMgr),
		fwd:               forward.NewForwardMgr(),
		connect:           connectivity.NewConnectivity(forward.NewForwardMgr(), driver, vmMeta),
		ssh:               sshMgr,
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
	go ha.HealthyTick(ctx)
	err := ha.GenDisk(ctx)
	if err != nil {
		return err
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
	go ha.Serve(ctx, ha.vmMeta.Name)
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

func (ha *HostAgent) GenDisk(ctx context.Context) error {
	err := ha.driver.CreateDisk(ctx)
	if err != nil {
		return err
	}
	return ha.ci.GenCIISO()
}

func (ha *HostAgent) EnsureCIISO(ctx context.Context) error {
	vmInfo := ha.vmMeta.Spec
	guestBin := filepath.Join(ha.vmMeta.Dir(), v1.GuestBin)
	if _, err := os.Stat(guestBin); errors.Is(err, os.ErrNotExist) {
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

	return ha.ci.GenCIISO()
}

func (ha *HostAgent) startRoutinesAndWait(ctx context.Context, errCh chan error) error {
	stBase := event.Status{
		SSHLocalPort: ha.ssh.GetPort(),
	}
	stBooting := stBase
	ha.emitEvent(ctx, event.Event{Status: stBooting})
	go func() {
		err := ha.connect.Forward(ctx)
		if err != nil {
			errCh <- err
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
			err := ha.driver.Stop(ctx)
			return err
		case <-ctx.Done():
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

func (ha *HostAgent) Serve(ctx context.Context, name string) {
	err := serve(ctx, name)
	if err != nil {
		klog.Fatalf("host agent serve: %s", err.Error())
	}
}

func serve(ctx context.Context, name string) error {

	sock := fmt.Sprintf("/tmp/guest-%s.sock", name)
	_ = os.RemoveAll(sock)
	var (
		cfg = &server.Config{
			Vsock: false, // listen on vsock
			//BindAddr: ":30443",
			BindAddr: sock,
		}
	)
	klog.Infof("run guest serve: uid=%d", os.Getuid())

	damon := server.NewOrDie(context.TODO(), cfg, map[string]map[string]server.HandlerFunc{})
	damon.AddRoute(map[string]map[string]server.HandlerFunc{
		"GET": {
			"/health": func(contex context.Context, w http.ResponseWriter, r *http.Request) {
				data := tool.PrettyJson(v1.Healthy{Status: "ok"})
				_, _ = w.Write([]byte(data))
			},
		},
	})
	return damon.Start(ctx)
}

func (ha *HostAgent) HealthyTick(ctx context.Context) {
	tick := time.NewTicker(15 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			_ = ha.vmMeta.SavePID()
		}
	}
}
