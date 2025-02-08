package hostagent

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/aoxn/meridian/internal/server"
	"github.com/aoxn/meridian/internal/tool/mapping"
	"k8s.io/apimachinery/pkg/util/wait"
	"net/http"

	//"errors"
	"fmt"
	"github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/tool"
	"github.com/aoxn/meridian/internal/tool/iso9660util"
	"github.com/aoxn/meridian/internal/vma/download"
	"github.com/aoxn/meridian/internal/vma/forward"
	"github.com/aoxn/meridian/internal/vma/host/event"
	"github.com/aoxn/meridian/internal/vma/model"
	nativeimg "github.com/aoxn/meridian/internal/vma/nativeimg"
	"github.com/aoxn/meridian/internal/vma/sshutil"
	"github.com/docker/go-units"
	"io"
	"k8s.io/klog/v2"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aoxn/meridian/internal/vma/backend"
	"github.com/aoxn/meridian/internal/vma/backend/vz"
	"github.com/aoxn/meridian/internal/vma/backend/wsl2"
	"github.com/aoxn/meridian/internal/vma/cidata"
	"github.com/sethvargo/go-password/password"
)

type HostAgent struct {
	vm  *model.Instance
	ssh *sshutil.SSHMgr
	ci  *cidata.CloudInit
	fwd *forward.ForwardMgr

	driver backend.Driver

	udpDNSLocalPort int
	tcpDNSLocalPort int
	onClose         []func() error // LIFO

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
func New(vm *v1.VirtualMachine, stdout io.Writer, opts ...Opt) (*HostAgent, error) {
	var o options
	for _, f := range opts {
		if err := f(&o); err != nil {
			return nil, err
		}
	}
	inst, err := model.NewInstance(vm)
	if err != nil {
		return nil, err
	}

	y := inst.Vm()
	// y is loaded with setDefault() already, so no need to care about nil pointers.

	sshLocalPort, err := determineSSHLocalPort(y, inst.Name)
	if err != nil {
		return nil, err
	}
	if y.VMType == v1.WSL2 {
		sshLocalPort = y.SSH.LocalPort
	}

	var udpDNSLocalPort, tcpDNSLocalPort int
	if y.HostResolver.Enabled {
		udpDNSLocalPort, err = findFreeUDPLocalPort()
		if err != nil {
			return nil, err
		}
		tcpDNSLocalPort, err = findFreeTCPLocalPort()
		if err != nil {
			return nil, err
		}
	}

	newBackend := func() backend.Driver {
		base := &backend.BaseDriver{
			I:            inst,
			Yaml:         inst.VirtualMachine,
			SSHLocalPort: sshLocalPort,
			VSockPort:    10443,
			VirtioPort:   "",
		}
		mt := base.Yaml.Spec.VMType
		switch mt {
		case v1.VZ:
			return vz.New(base)
		case v1.WSL2:
			return wsl2.New(base)
		}
		return vz.New(base)
	}
	sshMgr := sshutil.NewSSHMgr(inst.Name, "127.0.0.1", sshLocalPort)
	a := &HostAgent{
		vm:  inst,
		ci:  cidata.NewCloudInit(inst, sshMgr),
		fwd: forward.NewForwardMgr(),
		ssh: sshMgr,
		// to be deleted
		udpDNSLocalPort:   udpDNSLocalPort,
		tcpDNSLocalPort:   tcpDNSLocalPort,
		guestPort:         10443,
		driver:            newBackend(),
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
	go ha.vm.HealthyTick(ctx)
	err := ha.GenDisk(ctx)
	if err != nil {
		return err
	}
	errCh, err := ha.driver.Start(ctx)
	if err != nil {
		return err
	}

	// WSL instance SSH address isn't known until after vm start
	if ha.vm.Vm().VMType == v1.WSL2 {
		sshAddr, err := sshutil.GetSSHAddr(ha.vm.Name, string(ha.vm.Vm().VMType))
		if err != nil {
			return err
		}
		ha.ssh.SetAddr(sshAddr)
	}
	go ha.Serve(ctx, ha.vm.Name)
	go ha.reconcileMapping()

	if ha.vm.Vm().Video.Display == "vnc" {
		vncdisplay, vncoptions, _ := strings.Cut(ha.vm.Vm().Video.VNC.Display, ",")
		vnchost, vncnum, err := net.SplitHostPort(vncdisplay)
		if err != nil {
			return err
		}
		n, err := strconv.Atoi(vncnum)
		if err != nil {
			return err
		}
		vncport := strconv.Itoa(5900 + n)
		vncpwdfile := filepath.Join(ha.vm.Dir, v1.VNCPasswordFile)
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
		vncfile := filepath.Join(ha.vm.Dir, v1.VNCDisplayFile)
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

func (ha *HostAgent) reconcileMapping() {
	klog.Infof("[mapping]start reconcile upnp port mapping")
	if !ha.vm.Vm().Request.Config.HasFeature(v1.FeatureSupportNodeGroups) {
		klog.Infof("[mapping] nodegroups feature disabled, skip mapping")
		return
	}
	for {
		select {
		case <-time.After(5 * time.Minute):
			i := ha.vm.Vm().Request
			port, err := strconv.Atoi(i.AccessPoint.APIPort)
			if err != nil {
				klog.Errorf("[mapping]failed to parse access point port: %s", err)
				continue
			}
			tport, _ := strconv.Atoi(i.AccessPoint.TunnelPort)
			klog.Infof("[mapping]periodical mapping port: %d", port)
			for _, item := range []mapping.Item{
				{
					ExternalPort: port,
					InternalPort: port,
				},
				{
					ExternalPort: tport,
					InternalPort: tport,
				},
			} {
				err = mapping.AddMapping([]mapping.Item{item})
				if err != nil {
					klog.Errorf("[mapping]failed to add mapping: %s", err)
					continue
				}
				klog.Infof("[mapping] port [%d] mapped", item.ExternalPort)
			}
		}
	}
}

func (ha *HostAgent) GenDisk(ctx context.Context) error {
	guestBin := filepath.Join(ha.vm.Dir, v1.GuestBin)
	if _, err := os.Stat(guestBin); errors.Is(err, os.ErrNotExist) {
		f := v1.FindGuestBin(ha.vm.Vm().GuestVersion, string(ha.vm.Vm().OS), string(ha.vm.Vm().Arch))
		if f == nil {
			return fmt.Errorf("guest binary %s not found by arch: %s", guestBin, ha.vm.Vm().Arch)
		}
		klog.Infof("download guest binary: [%s][%s]", f.Arch, guestBin)
		if _, err := download.DownloadFile(ctx, guestBin, *f, true,
			"the guest binary", ha.vm.Vm().Arch); err != nil {
			return err
		}
	}
	err := ha.EnsureDisk(ctx)
	if err != nil {
		return err
	}
	return ha.ci.GenCIISO()
}

func (ha *HostAgent) EnsureDisk(ctx context.Context) error {
	diffDisk := filepath.Join(ha.vm.Dir, v1.DiffDisk)
	if _, err := os.Stat(diffDisk); err == nil || !errors.Is(err, os.ErrNotExist) {
		// disk is already ensured
		return err
	}

	baseDisk := filepath.Join(ha.vm.Dir, v1.BaseDisk)
	if _, err := os.Stat(baseDisk); errors.Is(err, os.ErrNotExist) {
		f := v1.FindImage(ha.vm.Vm().Image.Name)
		if f == nil {
			return fmt.Errorf("unexpected image name: [%s]", ha.vm.Vm().Image.Name)
		}
		if _, err := download.DownloadFile(ctx, baseDisk, *f, true,
			"the image", ha.vm.Vm().Arch); err != nil {
			return err
		}
	}
	diskSize, _ := units.RAMInBytes(ha.vm.Vm().Disk)
	if diskSize == 0 {
		return nil
	}
	isBaseDiskISO, err := iso9660util.IsISO9660(baseDisk)
	if err != nil {
		return err
	}
	if isBaseDiskISO {
		// Create an empty data volume (sparse)
		diffDiskF, err := os.Create(diffDisk)
		if err != nil {
			return err
		}
		if err = nativeimg.MakeSparse(diffDiskF, diskSize); err != nil {
			diffDiskF.Close()
			return err
		}
		return diffDiskF.Close()
	}
	if err = nativeimg.ConvertToRaw(baseDisk, diffDisk, &diskSize, false); err != nil {
		return fmt.Errorf("failed to convert %q to a raw disk %q: %w", baseDisk, diffDisk, err)
	}
	return err
}

func (ha *HostAgent) startRoutinesAndWait(ctx context.Context, errCh chan error) error {
	stBase := event.Status{
		SSHLocalPort: ha.ssh.GetPort(),
	}
	stBooting := stBase
	ha.emitEvent(ctx, event.Event{Status: stBooting})
	go func() {
		err := ha.setPortForward(ctx)
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
	defer ha.vm.RemovePID()
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

func (ha *HostAgent) setPortForward(ctx context.Context) error {
	addr, err := ha.waitForAddress(ctx)
	if err != nil {
		return err
	}
	dialer, err := ha.driver.Dialer(ctx)
	if err != nil {
		return err
	}
	ha.vm.Spec.SetPortForward(v1.PortForward{
		Proto:       "tcp",
		Source:      fmt.Sprintf("0.0.0.0:%s", ha.vm.Spec.Request.AccessPoint.TunnelPort),
		Destination: fmt.Sprintf("%s:8132", addr),
	})
	for _, f := range ha.vm.Spec.PortForwards {
		if f.VSockPort > 0 {
			rule := fmt.Sprintf("%s://%s->vsock://%d", f.Proto, f.Source, f.VSockPort)
			err = ha.fwd.AddBy(rule, dialer)
			if err != nil {
				return fmt.Errorf("add forwarding rule[vsock]:[%s] %s", rule, err.Error())
			}
		} else {
			rule := fmt.Sprintf("%s://%s->%s://%s", f.Proto, f.Source, f.Proto, f.Destination)
			err = ha.fwd.AddBy(rule)
			if err != nil {
				return fmt.Errorf("add forwarding rule:[%s] %s", rule, err.Error())
			}
		}
	}
	return nil
}

func (ha *HostAgent) waitForAddress(ctx context.Context) (string, error) {
	dial := func(ctx context.Context, network, addr string) (net.Conn, error) {
		return ha.driver.GuestAgentConn(ctx)
	}
	client := http.Client{
		Transport: &http.Transport{DialContext: dial},
	}
	var addr = ""
	waitFunc := func(ctx context.Context) (bool, error) {
		r, err := client.Get(fmt.Sprintf("http://localhost/apis/xdpin.cn/v1/guestinfos/%s", ha.vm.Name))
		if err != nil {
			klog.Errorf("wait guest info: %s", err.Error())
			return false, nil
		}
		klog.Infof("debug wait: %+v", r)
		defer r.Body.Close()
		if r.StatusCode != http.StatusOK {
			return false, nil
		}
		data, err := io.ReadAll(r.Body)
		if err != nil {
			return false, nil
		}
		var guest v1.GuestInfo
		err = json.Unmarshal(data, &guest)
		if err != nil {
			return false, nil
		}
		klog.Infof("[%s]wait guest server: %s, [%s]", ha.vm.Name, guest.Spec.Address, guest.Status.Phase)
		if guest.Status.Phase != v1.Running {
			klog.Infof("guest status is not running: [%s]", guest.Status.Phase)
			return false, nil
		}
		for _, ad := range guest.Spec.Address {
			if strings.Contains(ad, "192.168") {
				addr = ad
				return true, nil
			}
		}
		return false, nil
	}

	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, false, waitFunc)
	if err != nil {
		return "", err
	}
	return addr, err
}

const (
	verbForward = "forward"
	verbCancel  = "cancel"
)

func determineSSHLocalPort(y *v1.VirtualMachineSpec, instName string) (int, error) {
	if y.SSH.LocalPort > 0 {
		return y.SSH.LocalPort, nil
	}
	if y.SSH.LocalPort < 0 {
		return 0, fmt.Errorf("invalid ssh local port %d", y.SSH.LocalPort)
	}
	switch instName {
	case "default":
		// use hard-coded value for "default" instance, for backward compatibility
		return 60022, nil
	default:
		sshLocalPort, err := findFreeTCPLocalPort()
		if err != nil {
			return 0, fmt.Errorf("failed to find a free port, try setting `ssh.localPort` manually: %w", err)
		}
		return sshLocalPort, nil
	}
}

func findFreeTCPLocalPort() (int, error) {
	lAddr0, err := net.ResolveTCPAddr("tcp4", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	l, err := net.ListenTCP("tcp4", lAddr0)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	lAddr := l.Addr()
	lTCPAddr, ok := lAddr.(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("expected *net.TCPAddr, got %v", lAddr)
	}
	port := lTCPAddr.Port
	if port <= 0 {
		return 0, fmt.Errorf("unexpected port %d", port)
	}
	return port, nil
}

func findFreeUDPLocalPort() (int, error) {
	lAddr0, err := net.ResolveUDPAddr("udp4", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	l, err := net.ListenUDP("udp4", lAddr0)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	lAddr := l.LocalAddr()
	lUDPAddr, ok := lAddr.(*net.UDPAddr)
	if !ok {
		return 0, fmt.Errorf("expected *net.UDPAddr, got %v", lAddr)
	}
	port := lUDPAddr.Port
	if port <= 0 {
		return 0, fmt.Errorf("unexpected port %d", port)
	}
	return port, nil
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
