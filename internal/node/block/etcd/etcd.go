//go:build linux || darwin
// +build linux darwin

package etcd

import (
	"context"
	"fmt"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/node/block"
	"github.com/aoxn/meridian/internal/node/block/file"
	"github.com/aoxn/meridian/internal/node/host"
	"github.com/aoxn/meridian/internal/tool"
	"github.com/aoxn/meridian/internal/tool/cmd"
	"github.com/aoxn/meridian/internal/tool/sign"
	"github.com/pkg/errors"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	ETCD_USER      = "etcd"
	ETCD_HOME      = "/mnt/disk0/etcd"
	ETCD_TMP       = "/var/tmp/etcd"
	ETCD_UNIT_FILE = "/etc/systemd/system/etcd.service"
)

func EtcdHome() string {
	return ETCD_HOME
}

type etcd struct {
	req  *v1.Request
	file *file.File
	//*block.Local
	host   host.Host
	action string
}

// NewBlock returns a new etcd for kubeadm init
func NewBlock(
	req *v1.Request,
	host host.Host,
	action string,
) (block.Block, error) {
	info := file.PathInfo{
		InnerAddr: false,
		Arch:      host.Arch(),
		OSRelease: host.OS(),
		Region:    host.Region(),
	}
	err := info.Validate()
	if err != nil {
		return nil, err
	}
	return &etcd{
		req:    req,
		action: action,
		file: &file.File{
			Path:        info,
			Pkg:         file.PKG_ETCD,
			Ftype:       file.FILE_BINARY,
			InstallPath: "/usr/local/bin/",
			Version:     req.Spec.Config.Etcd.Version,
		},
		host: host,
	}, nil
}

// Ensure runs the etcd
func (a *etcd) Ensure(ctx context.Context) error {

	//if host == nil {
	//	return fmt.Errorf("can not get host info from ActionContex")
	//}
	klog.Info("try sign etcd cert")
	state := "new"
	// 1. make sure etcd unit file is exist in the whole process
	etcd := a.NewEtcd(a.req)
	if err := etcd.LoadOrSign(a.req); err != nil {
		return fmt.Errorf("sign: %s", err.Error())
	}
	err := etcd.FlushEtcdContent(a.req, a.host.Arch(), state)
	if err != nil {
		return fmt.Errorf("flush etcd content file %s: %s", ETCD_UNIT_FILE, err.Error())
	}
	err = a.file.Ensure(ctx)
	if err != nil {
		return errors.Wrapf(err, "install [%s] failed", a.file.Pkg)
	}
	switch a.action {
	case v1.ActionJoin:
		state = "existing"
		err := BackOffJoinMember(etcd)
		if err != nil {
			return fmt.Errorf("join etcd peer on bootType=%s: %s", a.action, err.Error())
		}
	case tool.BootTypeRecover:
		// todo: snapshot tmp file
		err = etcd.Restore(a.req, "index.SnapshotTMP")
		if err != nil {
			return errors.Wrapf(err, "restore snapshot")
		}
	}

	// 2. flush with the expected etcd [State] again
	err = etcd.FlushEtcdContent(a.req, a.host.Arch(), state)
	if err != nil {
		return fmt.Errorf("write file %s: %s", ETCD_UNIT_FILE, err.Error())
	}
	etcd.AddUser()

	err = a.host.Service().Enable("etcd")
	if err != nil {
		return fmt.Errorf("systecmctl enable etcd error,%s ", err.Error())
	}
	err = a.host.Service().DaemonReload()
	if err != nil {
		return fmt.Errorf("systecmctl enable etcd error,%s ", err.Error())
	}
	err = a.host.Service().Restart("etcd")
	if err != nil {
		return fmt.Errorf("systecmctl enable etcd error,%s ", err.Error())
	}

	return etcd.WaitEndpoints(advertise(etcd.me, "2379"))
}

func (a *etcd) Purge(ctx context.Context) error {
	err := a.host.Service().Disable("etcd")
	if err != nil {
		return fmt.Errorf("systecmctl disable etcd error,%s ", err.Error())
	}
	err = a.host.Service().Stop("etcd")
	if err != nil {
		return fmt.Errorf("systecmctl disable etcd error,%s ", err.Error())
	}
	for _, r := range []string{
		EtcdHome(),
		"/lib/systemd/system/etcd.service",
		"/usr/local/bin/etcd",
		"/usr/local/bin/etcdctl",
	} {
		klog.Infof("remove etcd file[%s]", r)
		err = os.RemoveAll(r)
		if err != nil {
			return errors.Wrapf(err, "remove file[%s]", r)
		}
	}
	return nil
}

func (a *etcd) CleanUp(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (a *etcd) Name() string {
	return fmt.Sprintf("etcd init [%s]", a.host.NodeID())
}

func (a *etcd) NewEtcd(node *v1.Request) *Etcd {
	var peer []string
	for _, n := range node.Spec.AccessPoint.Backends {
		peer = append(peer, n.Ip)
	}
	return &Etcd{me: a.host.NodeIP(), peer: peer}
}

// Etcd todo: use etcd API to reimplement etcd operations
type Etcd struct {
	// my IP addr
	me string
	// peer IP addrs
	peer []string
	//host *api.Master
}

func (m *Etcd) Me() string { return m.me }

func (m *Etcd) Peer() []string { return m.peer }

func (m *Etcd) AddUser() {
	klog.Infof("add etcd user...")
	sta := <-cmd.NewCmd("groupadd", "-r", ETCD_USER).Start()
	if err := cmd.CmdError(sta); err != nil {
		klog.Errorf("add etcd group error: %s", err.Error())
	}
	sta = <-cmd.NewCmd(
		"useradd",
		"-r",
		"-g", ETCD_USER,
		"-d", EtcdHome(),
		"-s", "/sbin/nologin",
		"-c", "etcd user", ETCD_USER,
	).Start()
	if err := cmd.CmdError(sta); err != nil {
		klog.Errorf("add etcd user error: %s", err.Error())
	}
	sta = <-cmd.NewCmd("chown",
		"-R", "etcd:etcd", EtcdHome(),
	).Start()
	if err := cmd.CmdError(sta); err != nil {
		klog.Errorf("chown etcd dir: %s", err.Error())
	}
}

// BackOffJoinMember
//  1. list all members and wait for them all ready
//  2. trying to add one peer, wait for them ready
//  3. remove myself if error occurred. repeat
func BackOffJoinMember(etcd *Etcd) error {

	return wait.Poll(
		5*time.Second,
		5*time.Minute,
		func() (done bool, err error) {
			err = etcd.Join()
			if err != nil {
				klog.Errorf("wait join etcd peer: %s", err)
				return false, nil
			}
			return true, nil
		},
	)
}

// MemberList load existing etcd peer
func (m *Etcd) MemberList() (Members, error) {

	mems := Members{}
	fortry := func(ip string) error {
		cm := cmd.NewCmd(
			"/usr/local/bin/etcdctl",
			"--endpoints",
			advertise(ip, "2379"),
			"--cacert",
			certHome(EtcdHome(), "server-ca.crt"),
			"--cert",
			certHome(EtcdHome(), "client.crt"),
			"--key",
			certHome(EtcdHome(), "client.key"),
			"-w", "json",
			"member", "list",
			//"--peer-urls", PeerURLs(peer[0].IP,"2379"),
		)
		cm.Env = []string{"ETCDCTL_API=3"}
		result := <-cm.Start()
		err := cmd.CmdError(result)
		if err != nil {
			return fmt.Errorf("peer list error: %s", err.Error())
		}
		err = Load(result.Stdout, &mems)
		if err != nil {
			return fmt.Errorf("unmarshal member: %s", err.Error())
		}

		for i, p := range mems.Members {
			if len(p.PeerURLs) < 1 {
				return fmt.Errorf("empty peer url: %+v", mems)
			}
			ips := strings.Split(p.PeerURLs[0], "//")
			if len(ips) < 2 {
				return fmt.Errorf("member list: unknown Advertise addr format, %s. skip", p.ClientURLs)
			}
			addr := strings.Split(ips[1], ":")
			if len(addr) < 2 {
				return fmt.Errorf("member list: unkown addr format, %s", addr)
			}
			mems.Members[i].IP = addr[0]
			klog.Infof("debug etcd member: %+v", mems.Members[i])
		}
		return nil
	}
	// TODO: try each peer on error.
	err := TryEachPeer(m.peer, 2*time.Second, fortry)
	return mems, err
}

func (m *Etcd) Restore(node *v1.Request, dir string) error {
	if dir == "" {
		return fmt.Errorf("empty snapshot path")
	}
	dataDir := filepath.Join(EtcdHome(), DataDir)
	bakDir := filepath.Join("/root", "db.bak")
	exist, err := tool.FileExist(bakDir)
	if err != nil {
		return errors.Wrap(err, "etcd backup file check")
	}
	if !exist {
		err = os.Rename(dataDir, bakDir)
		if err != nil {
			klog.Errorf("mv %s to %s: %s", dataDir, bakDir, err.Error())
		}
	}
	err = os.RemoveAll(dataDir)
	if err != nil {
		klog.Errorf("remove backup dir: %s", err.Error())
	}
	cm := cmd.NewCmd(
		"/usr/local/bin/etcdctl", "snapshot", "restore", dir,
		"--data-dir", dataDir,
		"--skip-hash-check=true",
		"--name", memberName(m.me),
		"--initial-cluster", m.InitialEtcdCluster(node, NewEmptyMembers()),
		"--initial-cluster-token", node.Spec.Config.Token,
		"--initial-advertise-peer-urls", advertise(m.me, "2380"),
		"--cacert", certHome(EtcdHome(), "server-ca.crt"),
		"--cert", certHome(EtcdHome(), "server.crt"),
	)
	cm.Env = []string{"ETCDCTL_API=3"}
	result := <-cm.Start()
	return cmd.CmdError(result)
}

func (m *Etcd) Snapshot(dir string) error {
	if dir == "" {
		return fmt.Errorf("empty snapshot target path")
	}
	if len(m.peer) <= 0 {
		return fmt.Errorf("empty peer endpoint, can not snapshot")
	}

	err := os.MkdirAll(filepath.Dir(dir), 0755)
	if err != nil {
		return errors.Wrapf(err, "make dir: %s", filepath.Dir(dir))
	}
	// todo:
	// 		should use local ip to backup
	first := []string{m.peer[0]}
	cm := cmd.NewCmd(
		"/usr/local/bin/etcdctl",
		"--endpoints",
		advertises(first, "2379"),
		"--cacert",
		certHome(EtcdHome(), "server-ca.crt"),
		"--cert",
		certHome(EtcdHome(), "client.crt"),
		"--key",
		certHome(EtcdHome(), "client.key"),
		"snapshot", "save", dir,
	)
	cm.Env = []string{"ETCDCTL_API=3"}
	result := <-cm.Start()
	return cmd.CmdError(result)
}

func (m *Etcd) Endpoints() ([]EndpointStatus, error) {
	var endpoints []EndpointStatus
	cm := cmd.NewCmd(
		"/usr/local/bin/etcdctl",
		"--endpoints",
		advertises(m.peer, "2379"),
		"--cacert",
		certHome(EtcdHome(), "server-ca.crt"),
		"--cert",
		certHome(EtcdHome(), "client.crt"),
		"--key",
		certHome(EtcdHome(), "client.key"),
		"-w", "json",
		"endpoint", "status",
	)
	cm.Env = []string{"ETCDCTL_API=3"}
	result := <-cm.Start()
	err := cmd.CmdError(result)
	if err != nil {
		return endpoints, fmt.Errorf("endpoint status error: %s", err.Error())
	}
	err = Load(result.Stdout, &endpoints)
	return endpoints, err
}

// Join is the entry of peer join method
// join a peer into an existing etcd cluster.
// m.host.status.peer must not be empty
func (m *Etcd) Join() error {
	omems, err := m.MemberList()
	if err != nil {
		return fmt.Errorf("peer: %s", err.Error())
	}
	err = m.WaitEndpoints(memAdvertise(omems.Members))
	if err != nil {
		return fmt.Errorf("wait etcd ready: %s", err.Error())
	}
	err = m.JoinMe()
	if err != nil {
		return fmt.Errorf("join peer[%s] fail, %s", m.me, err.Error())
	}
	nmems, err := m.MemberList()
	if err != nil {
		return fmt.Errorf("try list peer: %s", err.Error())
	}
	if len(omems.Members)+1 < len(nmems.Members) {
		klog.Errorf("concurrent etcd peer join. backoff, old=%v, new=%v", omems, nmems)
		err = m.RemoveMember(FindMemberByIP(nmems.Members, m.me))
		if err != nil {
			return fmt.Errorf("remove myself error: %s", err.Error())
		}
		return fmt.Errorf("concurrent etcd peer join: old=[%v], new=[%v]", omems, nmems)
	}
	// join finished
	klog.Infof("join peer finished: %s", memberName(m.me))
	return nil
}

func (m *Etcd) RemoveMember(mem Member) error {
	klog.Infof("trying to remove etcd member: %v", mem)
	if mem.ID == nil || mem.ID.String() == "0" {
		klog.Infof("empty member ip: skip remove member")
		return nil
	}
	remove := func(ip string) error {
		cm := cmd.NewCmd(
			"/usr/local/bin/etcdctl",
			"--endpoints",
			advertise(ip, "2379"),
			"--cacert",
			certHome(EtcdHome(), "server-ca.crt"),
			"--cert",
			certHome(EtcdHome(), "client.crt"),
			"--key",
			certHome(EtcdHome(), "client.key"),
			"member", "remove", fmt.Sprintf("%x", mem.ID),
			//"--peer-urls", PeerURLs(peer[0].IP,"2379"),
		)
		cm.Env = []string{"ETCDCTL_API=3"}
		result := <-cm.Start()
		return cmd.CmdError(result)
	}
	return TryEachPeer(
		m.peer, 2*time.Second, remove,
	)
}

func FindMemberByIP(
	mems []Member, ip string,
) Member {

	for _, mem := range mems {
		if len(mem.PeerURLs) == 0 {
			continue
		}
		adv := advertise(ip, "2380")
		if mem.PeerURLs[0] == adv {
			return mem
		}
	}
	return Member{}
}

func (m *Etcd) JoinMe() error {
	klog.Infof("try join etcd peer: %s", memberName(m.me))

	// TODO:
	//   check peer exists before add
	//   try each peer on error
	if len(m.peer) == 0 {
		return fmt.Errorf("major master does not exists: %s", m.peer)
	}

	return TryEachPeer(
		m.peer, 3*time.Second,
		func(ip string) error {
			mems, err := m.MemberList()
			if err != nil {
				return fmt.Errorf("join me: %s", err.Error())
			}
			for _, v := range mems.Members {
				if v.IP != "" &&
					v.IP == m.me {
					klog.Infof("etcd.me %s already exist, skip join", m.me)
					return nil
				}
			}
			// start to join me with backoff
			cm := cmd.NewCmd(
				"/usr/local/bin/etcdctl",
				"--endpoints",
				advertise(ip, "2379"),
				"--cacert",
				certHome(EtcdHome(), "server-ca.crt"),
				"--cert",
				certHome(EtcdHome(), "client.crt"),
				"--key",
				certHome(EtcdHome(), "client.key"),
				"member", "add", memberName(m.me),
				"--peer-urls", advertise(m.me, "2380"),
				//"--peer-urls", PeerURLs(peer[0].IP,"2379"),
			)
			cm.Env = []string{"ETCDCTL_API=3"}
			result := <-cm.Start()
			return cmd.CmdError(result)
		},
	)
}

func TryEachPeer(
	peer []string,
	interval time.Duration,
	mfunc func(ip string) error,
) error {
	var lastError error
	klog.Infof("try each peer list: %s", peer)
	for _, p := range peer {
		for i := 0; i < 2; i++ {
			err := mfunc(p)
			if err == nil {
				return nil
			}
			lastError = err
			if interval != 0 {
				time.Sleep(interval)
			}
		}
	}
	return errors.Wrapf(lastError, "NoMoreEndpointsToTry")
}

// FlushEtcdContent flush etcd content into system unit file
// use host.Spec.IP as initial cluster if m.mem is empty.
func (m *Etcd) FlushEtcdContent(
	node *v1.Request,
	arch, state string,
) error {
	mems := Members{}
	if state != "new" {
		var err error
		mems, err = m.MemberList()
		if err != nil {
			return fmt.Errorf("member list: on etcd content, %s", err.Error())
		}
	}
	return ioutil.WriteFile(
		ETCD_UNIT_FILE,
		[]byte(m.EtcdUnitFileContent(node, mems.Members, arch, state)), 0644,
	)
}

func NewEmptyMembers() []Member { return []Member{} }

type Member struct {
	ID         *big.Int `json:"ID,omitempty" protobuf:"bytes,1,opt,name=ID"`
	IP         string   `json:"IP,omitempty" protobuf:"bytes,2,opt,name=IP"`
	State      string   `json:"State,omitempty" protobuf:"bytes,3,opt,name=State"`
	Name       string   `json:"name,omitempty" protobuf:"bytes,4,opt,name=name"`
	PeerURLs   []string `json:"peerURLs,omitempty" protobuf:"bytes,5,opt,name=peerURLs"`
	ClientURLs []string `json:"clientURLs,omitempty" protobuf:"bytes,6,opt,name=clientURLs"`
}

func memAdvertise(mems []Member) string {
	var addrs []string
	for _, mem := range mems {
		addrs = append(addrs, mem.ClientURLs[0])
	}
	return strings.Join(addrs, ",")
}

func (m *Etcd) WaitEndpoints(endpints string) error {
	var (
		err     error
		cnt     = 0
		timeout = time.After(3 * time.Minute)
	)

	for {
		select {
		case <-timeout:
			return fmt.Errorf("wait for etcd ready timeout, %v", err)
		default:
			time.Sleep(1 * time.Second)
			cm := cmd.NewCmd(
				"/usr/local/bin/etcdctl",
				"--endpoints", endpints,
				"--cacert",
				certHome(EtcdHome(), "server-ca.crt"),
				"--cert",
				certHome(EtcdHome(), "client.crt"),
				"--key",
				certHome(EtcdHome(), "client.key"),
				"endpoint", "health",
			)
			cm.Env = []string{"ETCDCTL_API=3"}
			result := <-cm.Start()
			if err := cmd.CmdError(result); err != nil {
				klog.Infof("waiting for etcd ready...%s", err.Error())
				continue
			}
			cnt++
			if cnt == 3 {
				klog.Infof("wait ectd ready: 3round success")
				return nil
			}
		}
	}
}

func (m *Etcd) EndpointHealth(ip string) error {
	endpoint := advertise(ip, "2379")
	cm := cmd.NewCmd(
		"/usr/local/bin/etcdctl",
		"--endpoints", endpoint,
		"--cacert",
		certHome(EtcdHome(), "server-ca.crt"),
		"--cert",
		certHome(EtcdHome(), "client.crt"),
		"--key",
		certHome(EtcdHome(), "client.key"),
		"endpoint", "health",
	)
	cm.Env = []string{"ETCDCTL_API=3"}
	result := <-cm.Start()
	return cmd.CmdError(result)
}

func certHome(home, name string) string {
	if home == "" {
		home = ETCD_HOME
	}
	return fmt.Sprintf("%s/cert/%s", home, name)
}

func (m *Etcd) LoadOrSign(node *v1.Request) error {
	home := EtcdHome()
	ips := []string{m.me}
	peer := node.Spec.Config.TLS["etcd-peer"]
	// Sign peer cert
	key, cert, err := sign.SignEtcdMember(peer.Cert, peer.Key, ips, m.me)
	if err != nil {
		return fmt.Errorf("sign etcd peer cert fail, %s", err.Error())
	}

	server := node.Spec.Config.TLS["etcd-server"]
	// Sign peer cert
	skey, scert, err := sign.SignEtcdServer(server.Cert, server.Key, ips, m.me)
	if err != nil {
		return fmt.Errorf("sign etcd server cert fail, %s", err.Error())
	}

	// Sign peer cert
	ckey, ccert, err := sign.SignEtcdClient(server.Cert, server.Key, []string{}, m.me)
	if err != nil {
		return fmt.Errorf("sign etcd client cert fail, %s", err.Error())
	}
	err = os.MkdirAll(fmt.Sprintf("%s/cert", home), 0755)
	if err != nil {
		return fmt.Errorf("mkdir etcd home dir: %s", err.Error())
	}
	for name, v := range map[string][]byte{
		"server.crt":    scert,
		"server.key":    skey,
		"server-ca.crt": server.Cert,
		"server-ca.key": server.Key,
		"client.crt":    ccert,
		"client.key":    ckey,
		"peer.crt":      cert,
		"peer.key":      key,
		"peer-ca.crt":   peer.Cert,
		"peer-ca.key":   peer.Key,
	} {
		if err := ioutil.WriteFile(certHome(home, name), v, 0644); err != nil {
			return fmt.Errorf("write file %s: %s", name, err.Error())
		}
	}
	return nil
}

func (m *Etcd) EtcdUnitFileContent(node *v1.Request, mems []Member, arch, state string) string {
	up := []string{
		"[Unit]",
		"Description=etcd service",
		"After=network.target",
		"",
		"[Service]",
		fmt.Sprintf("WorkingDirectory=%s", EtcdHome()),
		"User=etcd",
	}
	if arch != "amd64" {
		up = append(up, fmt.Sprintf("Environment=\"ETCD_UNSUPPORTED_ARCH=%s\"", arch))
	}
	var mid []string
	for k, v := range m.FlagEnv(node, mems, state) {
		mid = append(mid, fmt.Sprintf("%s=%s", k, v))
	}
	execs := fmt.Sprintf("ExecStart=/usr/local/bin/etcd %s", strings.Join(mid, " "))
	down := []string{
		execs,
		"LimitNOFILE=65536",
		"Restart=always",
		"RestartSec=15s",
		"OOMScoreAdjust=-999",
		"[Install]",
		"WantedBy=multi-user.target",
	}

	tmp := append(
		up,
		down...,
	)
	return strings.Join(tmp, "\n")
}

const DataDir = "data.etcd"

func (m *Etcd) Env(node *v1.Request, mems []Member, state string) map[string]string {
	return map[string]string{
		"ETCD_INITIAL_CLUSTER_TOKEN":       node.Spec.Config.Etcd.InitToken,
		"ETCD_PEER_TRUSTED_CA_FILE":        certHome(EtcdHome(), "peer-ca.crt"),
		"ETCD_PEER_CERT_FILE":              certHome(EtcdHome(), "peer.crt"),
		"ETCD_PEER_KEY_FILE":               certHome(EtcdHome(), "peer.key"),
		"ETCD_NAME":                        memberName(m.me),
		"ETCD_DATA_DIR":                    DataDir,
		"ETCD_ELECTION_TIMEOUT":            "3000",
		"ETCD_HEARTBEAT_INTERVAL":          "500",
		"ETCD_SNAPSHOT_COUNT":              "50000",
		"ETCD_CLIENT_CERT_AUTH":            "true",
		"ETCD_TRUSTED_CA_FILE":             certHome(EtcdHome(), "server-ca.crt"),
		"ETCD_CERT_FILE":                   certHome(EtcdHome(), "server.crt"),
		"ETCD_KEY_FILE":                    certHome(EtcdHome(), "server.key"),
		"ETCD_PEER_CLIENT_CERT_AUTH":       "true",
		"ETCD_INITIAL_ADVERTISE_PEER_URLS": advertise(m.me, "2380"),
		"ETCD_LISTEN_PEER_URLS":            advertise(m.me, "2380"),
		"ETCD_ADVERTISE_CLIENT_URLS":       advertise(m.me, "2379"),
		"ETCD_LISTEN_CLIENT_URLS":          advertise(m.me, "2379"),
		"ETCD_INITIAL_CLUSTER":             m.InitialEtcdCluster(node, mems),
		"ETCD_INITIAL_CLUSTER_STATE":       state,
	}
}

func (m *Etcd) FlagEnv(node *v1.Request, mems []Member, state string) map[string]string {
	return map[string]string{
		"--initial-cluster-token":       node.Spec.Config.Etcd.InitToken,
		"--peer-trusted-ca-file":        certHome(EtcdHome(), "peer-ca.crt"),
		"--peer-cert-file":              certHome(EtcdHome(), "peer.crt"),
		"--peer-key-file":               certHome(EtcdHome(), "peer.key"),
		"--name":                        memberName(m.me),
		"--data-dir":                    DataDir,
		"--election-timeout":            "3000",
		"--heartbeat-interval":          "500",
		"--snapshot-count":              "50000",
		"--client-cert-auth":            "true",
		"--trusted-ca-file":             certHome(EtcdHome(), "server-ca.crt"),
		"--cert-file":                   certHome(EtcdHome(), "server.crt"),
		"--key-file":                    certHome(EtcdHome(), "server.key"),
		"--peer-client-cert-auth":       "true",
		"--initial-advertise-peer-urls": advertise(m.me, "2380"),
		"--listen-peer-urls":            advertise(m.me, "2380"),
		"--advertise-client-urls":       advertise(m.me, "2379"),
		"--listen-client-urls":          advertise(m.me, "2379"),
		"--initial-cluster":             m.InitialEtcdCluster(node, mems),
		"--initial-cluster-state":       state,
	}
}

func memberName(ip string) string {
	return fmt.Sprintf("etcd-%s.member", ip)
}

func (m *Etcd) InitialEtcdCluster(node *v1.Request, mem []Member) string {
	member := func(ip string) string {
		return fmt.Sprintf("etcd-%s.member=%s", ip, advertise(ip, "2380"))
	}
	if len(mem) == 0 {
		return member(m.me)
	}
	var addr []string
	for _, m := range mem {
		if len(m.PeerURLs) == 0 {
			klog.Errorf("empty peer address: %v", m)
			continue
		}
		addr = append(addr, fmt.Sprintf("etcd-%s.member=%s", m.IP, m.PeerURLs[0]))
	}
	return strings.Join(addr, ",")
}

func advertise(ip, port string) string {
	return fmt.Sprintf("https://%s:%s", ip, port)
}

func advertises(ips []string, port string) string {
	var hosts []string
	for _, ip := range ips {
		hosts = append(hosts, advertise(ip, port))
	}
	return strings.Join(hosts, ",")
}
