//go:build windows
// +build windows

package etcd

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

const (
	ETCD_USER      = "etcd"
	ETCD_HOME      = "/var/lib/etcd"
	ETCD_UNIT_FILE = "/lib/systemd/system/etcd.service"
)

type action struct {
}

// NewBlock returns a new etcd for kubeadm init
func NewAction() actions.Action {
	return &action{}
}

// Ensure runs the etcd
func (a *action) Execute(ctx *actions.ActionContext) error { return nil }

func certHome(name string) string {
	return fmt.Sprintf("%s/cert/%s", ETCD_HOME, name)
}

func LoadOrSign(node *v1.Master) error {
	ips := []string{node.Spec.IP}
	// Sign peer cert
	key, cert, err := sign.SignEtcdMember(
		node.Status.BootCFG.Etcd.PeerCA.Cert,
		node.Status.BootCFG.Etcd.PeerCA.Key,
		ips,
		node.Spec.ID,
	)
	if err != nil {
		return fmt.Errorf("sign etcd peer cert fail, %s", err.Error())
	}

	// Sign peer cert
	skey, scert, err := sign.SignEtcdServer(
		node.Status.BootCFG.Etcd.ServerCA.Cert,
		node.Status.BootCFG.Etcd.ServerCA.Key,
		ips,
		node.Spec.ID,
	)
	if err != nil {
		return fmt.Errorf("sign etcd server cert fail, %s", err.Error())
	}

	// Sign peer cert
	ckey, ccert, err := sign.SignEtcdClient(
		node.Status.BootCFG.Etcd.ServerCA.Cert,
		node.Status.BootCFG.Etcd.ServerCA.Key,
		[]string{},
		node.Spec.ID,
	)
	if err != nil {
		return fmt.Errorf("sign etcd client cert fail, %s", err.Error())
	}
	err = os.MkdirAll(fmt.Sprintf("%s/cert", ETCD_HOME), 0755)
	if err != nil {
		return fmt.Errorf("mkdir etcd home dir: %s", err.Error())
	}
	for name, v := range map[string][]byte{
		"server.crt":    scert,
		"server.key":    skey,
		"server-ca.crt": node.Status.BootCFG.Etcd.ServerCA.Cert,
		"server-ca.key": node.Status.BootCFG.Etcd.ServerCA.Key,
		"client.crt":    ccert,
		"client.key":    ckey,
		"peer.crt":      cert,
		"peer.key":      key,
		"peer-ca.crt":   node.Status.BootCFG.Etcd.PeerCA.Cert,
		"peer-ca.key":   node.Status.BootCFG.Etcd.PeerCA.Key,
	} {
		if err := ioutil.WriteFile(certHome(name), v, 0644); err != nil {
			return fmt.Errorf("write file %s: %s", name, err.Error())
		}
	}
	return nil
}

func EtcdUnitFileContent(node *v1.Master, state string) string {
	up := []string{
		"[Unit]",
		"Description=etcd service",
		"After=network.target",
		"",
		"[Service]",
		fmt.Sprintf("WorkingDirectory=%s", ETCD_HOME),
		"User=etcd",
	}
	down := []string{
		"ExecStart=/usr/bin/etcd",
		"LimitNOFILE=65536",
		"Restart=always",
		"RestartSec=15s",
		"OOMScoreAdjust=-999",
		"[Install]",
		"WantedBy=multi-user.target",
	}
	var mid []string
	for k, v := range map[string]string{
		"ETCD_INITIAL_CLUSTER_TOKEN":       node.Status.BootCFG.Etcd.InitToken,
		"ETCD_PEER_TRUSTED_CA_FILE":        certHome("peer-ca.crt"),
		"ETCD_PEER_CERT_FILE":              certHome("peer.crt"),
		"ETCD_PEER_KEY_FILE":               certHome("peer.key"),
		"ETCD_NAME":                        fmt.Sprintf("etcd-%s.peer", node.Spec.ID),
		"ETCD_DATA_DIR":                    "data.etcd",
		"ETCD_ELECTION_TIMEOUT":            "3000",
		"ETCD_HEARTBEAT_INTERVAL":          "500",
		"ETCD_SNAPSHOT_COUNT":              "50000",
		"ETCD_CLIENT_CERT_AUTH":            "true",
		"ETCD_TRUSTED_CA_FILE":             certHome("server-ca.crt"),
		"ETCD_CERT_FILE":                   certHome("server.crt"),
		"ETCD_KEY_FILE":                    certHome("server.key"),
		"ETCD_PEER_CLIENT_CERT_AUTH":       "true",
		"ETCD_INITIAL_ADVERTISE_PEER_URLS": advertise(node.Spec.IP, "2380"),
		"ETCD_LISTEN_PEER_URLS":            advertise(node.Spec.IP, "2380"),
		"ETCD_ADVERTISE_CLIENT_URLS":       advertise(node.Spec.IP, "2379"),
		"ETCD_LISTEN_CLIENT_URLS":          advertise(node.Spec.IP, "2379"),
		"ETCD_INITIAL_CLUSTER":             EtcdHosts(node),
		"ETCD_INITIAL_CLUSTER_STATE":       state,
	} {
		mid = append(mid, fmt.Sprintf("Environment=\"%s=%s\"", k, v))
	}
	tmp := append(
		append(up, mid...),
		down...,
	)
	return strings.Join(tmp, "\n")
}

func EtcdHosts(node *v1.Master) string {
	hosts := node.Status.BootCFG.Etcd.Endpoints
	if hosts == "" {
		return fmt.Sprintf("etcd-%s.peer=%s", node.Spec.IP, advertise(node.Spec.IP, "2380"))
	}
	var result []string
	for _, host := range strings.Split(hosts, ",") {
		result = append(result, fmt.Sprintf("etcd-%s.peer=%s", host, advertise(host, "2380")))
	}
	return strings.Join(result, ",")
}

func advertise(ip, port string) string {
	return fmt.Sprintf("https://%s:%s", ip, port)
}
