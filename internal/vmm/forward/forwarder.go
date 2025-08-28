package forward

import (
	"fmt"
	"github.com/mdlayher/vsock"
	"github.com/pkg/errors"
	dialer "golang.org/x/net/proxy"
	"k8s.io/klog/v2"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

//type Dialer func(network, addr string) (c net.Conn, err error)

type Forwarder interface {
	Rule() string
	Stop()
	BindAddr() string
	Forward() error
}

func NewForwardMgr() *ForwardMgr {
	return &ForwardMgr{mu: &sync.RWMutex{}, fwd: make(map[string]Forwarder)}
}

type ForwardMgr struct {
	mu  *sync.RWMutex
	fwd map[string]Forwarder
}

func (mgr *ForwardMgr) AddBy(rule string, dialer ...dialer.Dialer) error {
	fwd, err := NewForwarderBy(rule, dialer...)
	if err != nil {
		return err
	}
	mgr.Add(fwd)
	return nil
}

func (mgr *ForwardMgr) Add(fwd Forwarder) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	f, ok := mgr.fwd[fwd.Rule()]
	if ok {
		klog.Warningf("duplicated forward rule: %s", fwd.BindAddr())
		f.Stop()
		delete(mgr.fwd, fwd.Rule())
	}
	mgr.fwd[fwd.Rule()] = fwd
	go func() {
		err := fwd.Forward()
		if err == nil {
			return
		}
		klog.Errorf("request forwarder: %s", err)
	}()
}

func (mgr *ForwardMgr) Remove(key string) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	fwd, ok := mgr.fwd[key]
	if ok {
		fwd.Stop()
		delete(mgr.fwd, key)
	}
}

func (mgr *ForwardMgr) String() string {
	var fwds []string
	for _, fwd := range mgr.fwd {
		if len(fwds) > 10 {
			continue
		}
		fwds = append(fwds, fwd.BindAddr())
	}

	return fmt.Sprintf("total[%d] forward rule: %s", len(fwds), fwds)
}

func NewForwarderBy(rule string, dialer ...dialer.Dialer) (Forwarder, error) {
	block := strings.Split(rule, "->")
	if len(block) != 2 {
		return nil, fmt.Errorf("invalid rule: %s", rule)
	}

	addrFrom := strings.Split(block[0], "://")
	addrTo := strings.Split(block[1], "://")
	if len(addrFrom) != 2 || len(addrTo) != 2 {
		return nil, fmt.Errorf("invalid address rule: %s", rule)
	}
	klog.Infof("add forwarding rule: %s %s -> %s %s", addrFrom[0], addrFrom[1], addrTo[0], addrTo[1])
	return NewForward(rule, addrFrom[0], addrFrom[1], addrTo[0], addrTo[1], dialer...)
}

func NewForward(rule string, bindNetwork, bindAddr, forwardNetwork, forwardAddr string, dialer ...dialer.Dialer) (Forwarder, error) {
	fwd := &forwarder{
		rule: rule,
		quit: make(chan struct{}, 10),
		bindAt: &addr{
			network: bindNetwork,
			address: bindAddr,
		},
		forwardTo: &addr{
			network: forwardNetwork,
			address: forwardAddr,
		},
		remoteDialer: addrDialer{},
	}
	if (forwardNetwork == "" || forwardNetwork == "vsock") && len(dialer) == 0 {
		return nil, fmt.Errorf("virtio need dialer")
	}
	if dialer != nil && len(dialer) != 0 {
		fwd.remoteDialer = dialer[0]
	}
	return fwd, nil
}

type forwarder struct {
	rule string

	quit chan struct{}

	lt net.Listener

	bindAt *addr

	forwardTo *addr

	remoteDialer dialer.Dialer
}

type addr struct {
	network string
	address string
}

func (ad *addr) String() string {
	return fmt.Sprintf("%s@%s", ad.network, ad.address)
}

type addrDialer struct{}

func (addrDialer) Dial(network, address string) (net.Conn, error) {
	var (
		contextId uint32 = 0
		err       error
		conn      net.Conn
	)
	switch network {
	case "vsock":
		conn, err = vsock.Dial(contextId, intPort(address), &vsock.Config{})
	case "tcp", "udp", "unix":
		conn, err = net.Dial(network, address)
	default:
		return nil, errors.New("unknown network for dial")
	}
	return conn, err
}

func intPort(sp string) uint32 {
	port, err := strconv.Atoi(sp)
	if err != nil {
		panic(fmt.Sprintf("convert port: %s, %s", sp, err.Error()))
	}
	return uint32(port)
}

func (p *forwarder) Stop() {
	_ = p.lt.Close()
	klog.Infof("stop forwarder: %s", p.bindAt)
	close(p.quit)
	if p.bindAt.network != "unix" {
		return
	}
	_ = os.Remove(p.bindAt.address)
}

func (p *forwarder) Rule() string {
	return p.rule
}

func (p *forwarder) BindAddr() string {
	return p.bindAt.String()
}
func (p *forwarder) Forward() error {
	var err error
	p.lt, err = p.Listen()
	if err != nil {
		return errors.Wrapf(err, "bind listener, %s: %v", p.bindAt, err)
	}
	klog.Infof("forwarder listen at: %s", p.bindAt)
	for {
		select {
		case <-p.quit:
			return fmt.Errorf("actively quit forwarder %s", p.bindAt)
		default:
		}
		conn, err := p.lt.Accept()
		if err != nil {
			time.Sleep(15 * time.Second)
			klog.Warningf("forwarder: accepting connection [addr %s] with %s", p.bindAt, err)
			continue
		}
		klog.V(5).Infof("connection accepted: remote=[%s] -> local=[%s]", conn.RemoteAddr(), conn.LocalAddr())
		forwardConn, err := p.remoteDialer.Dial(p.forwardTo.network, p.forwardTo.address)
		if err != nil {
			_ = conn.Close()
			time.Sleep(15 * time.Second)
			klog.Warningf("forwarder: dialing connection [addr %s] with %s", p.forwardTo, err)
			continue
		}
		klog.V(5).Infof("forward new connection: client=[%s] -> local=[%s] -> destination=[%s]", conn.RemoteAddr(), conn.LocalAddr(), p.forwardTo)
		go Bicopy(conn, forwardConn, p.quit)
	}
}

func (p *forwarder) Listen() (net.Listener, error) {
	var (
		err error
		lt  net.Listener
	)
	bindAt := p.bindAt
	switch bindAt.network {
	case "vsock":
		lt, err = vsock.Listen(intPort(bindAt.address), &vsock.Config{})
	case "tcp", "udp", "unix":
		if bindAt.network == "unix" {
			if err = ensureSock(bindAt.address); err != nil {
				return nil, err
			}
		}
		lt, err = net.Listen(bindAt.network, bindAt.address)
	default:
		return nil, errors.New("unknown network for listen")
	}
	return lt, err
}

func ensureSock(path string) error {
	spath := filepath.Dir(path)
	err := os.MkdirAll(spath, 0700)
	if err != nil {
		return err
	}

	err = syscall.Unlink(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
