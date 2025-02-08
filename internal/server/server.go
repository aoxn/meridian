package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/aoxn/meridian/internal/tool/cmd"
	"github.com/gorilla/mux"
	"github.com/mdlayher/vsock"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// Config server config
type Config struct {
	Vsock       bool
	BindAddr    string
	TLSKeyPath  string
	TLSCertPath string
	TLSCAPath   string
}

func (cfg *Config) isUnixAddr() bool {
	return strings.HasPrefix(cfg.BindAddr, "/")
}

type HandlerFunc func(contex context.Context, w http.ResponseWriter, r *http.Request)

type Server struct {
	ctx     context.Context
	handler *mux.Router
	cfg     *Config
	auth    Authenticate
}

func NewOrDie(ctx context.Context, cfg *Config, handler map[string]map[string]HandlerFunc) *Server {
	if cfg == nil || cfg.BindAddr == "" {
		panic(fmt.Sprintf("bind address must not be empty"))
	}
	svr := &Server{
		ctx:     ctx,
		cfg:     cfg,
		handler: mux.NewRouter(),
		auth:    &TokenAuthenticator{},
	}
	return svr.setHandler(handler)
}

func (svr *Server) setHandler(handler map[string]map[string]HandlerFunc) *Server {
	for method, mappings := range handler {
		for r, h := range mappings {
			handler := h

			klog.V(6).Infof("start to register http router:[%s] %s", method, r)

			svr.handler.Path(r).Methods(method).HandlerFunc(
				func(w http.ResponseWriter, req *http.Request) {

					if err := svr.auth.Authorize(req); err != nil {
						http.Error(w, "authentication failure", http.StatusBadRequest)
						return
					}
					handler(context.TODO(), w, req)
				},
			)
		}
	}
	return svr
}

func (svr *Server) Start(ctx context.Context) error {
	lt, err := newListener(svr.cfg)
	if err != nil {
		return err
	}
	if svr.cfg.isUnixAddr() {
		m := cmd.NewCmd("chmod", "777", svr.cfg.BindAddr)
		result := <-m.Start()
		err := cmd.CmdError(result)
		if err != nil {
			return err
		}
	}
	go func() {
		err := http.Serve(lt, svr.handler)
		if err != nil {
			klog.Errorf("run server: %s", err.Error())
		}
	}()
	go func() {
		select {
		case <-ctx.Done():
			svr.CleanUp()
		}
	}()
	return nil
}

func (svr *Server) CleanUp() {
	if svr.cfg.isUnixAddr() {
		err := os.Remove(svr.cfg.BindAddr)
		klog.Infof("cleanup bind sock %s, %v", svr.cfg.BindAddr, err)
	}
}

func (svr *Server) AddRoute(handler map[string]map[string]HandlerFunc) *Server {
	return svr.setHandler(handler)
}

func newListener(cfg *Config) (net.Listener, error) {
	if cfg.Vsock {
		port, err := strconv.Atoi(cfg.BindAddr)
		if err != nil {
			return nil, err
		}
		return vsock.Listen(uint32(port), &vsock.Config{})
	}
	var (
		network = "tcp"
	)
	if cfg.isUnixAddr() {
		network = "unix"
	}
	if cfg.TLSCAPath == "" ||
		cfg.TLSCertPath == "" ||
		cfg.TLSKeyPath == "" {
		klog.Infof("cert not provided or secure listen not enabled, serve insecurely on[%s] %s", network, cfg.BindAddr)

		return net.Listen(network, cfg.BindAddr)
	}
	pool := x509.NewCertPool()
	ca, err := os.ReadFile(cfg.TLSCAPath)
	if err != nil {
		return nil, errors.Wrapf(err, "read ca file")
	}
	pool.AppendCertsFromPEM(ca)
	crt, err := tls.LoadX509KeyPair(cfg.TLSCertPath, cfg.TLSKeyPath)
	if err != nil {
		return nil, errors.Wrapf(err, "load cert pair")
	}

	tlsconfig := &tls.Config{
		RootCAs:      pool,
		ClientCAs:    pool,
		Certificates: []tls.Certificate{crt},
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}
	return tls.Listen(network, cfg.BindAddr, tlsconfig)
}
