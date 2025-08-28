package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"github.com/aoxn/meridian/internal/tool/cmd"
	"github.com/gorilla/mux"
	"github.com/mdlayher/vsock"
	"github.com/pkg/errors"
	"io"
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

type HandlerFunc func(r *http.Request, w http.ResponseWriter) int

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

func (svr *Server) setHandler(routes map[string]map[string]HandlerFunc) *Server {
	for method, mappings := range routes {
		for r, hfn := range mappings {
			klog.V(6).Infof("start to register http router:[%s] %s", method, r)
			svr.handler.Path(r).Methods(method).HandlerFunc(
				func(w http.ResponseWriter, req *http.Request) {

					if err := svr.auth.Authorize(req); err != nil {
						http.Error(w, "authentication failure", http.StatusBadRequest)
						return
					}
					code := hfn(req, w)
					klog.V(9).Infof("run %s [%s] with http code:[%d]", method, r, code)
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

func DecodeBody(body io.ReadCloser, v interface{}) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, v)
}

func HttpJson(w http.ResponseWriter, v interface{}) int {
	var text string
	code := http.StatusOK
	switch v.(type) {
	case error:
		text = v.(error).Error()
		code = http.StatusInternalServerError
	case string:
		text = v.(string)
	default:
		resp, err := json.Marshal(v)
		if err != nil {
			text = err.Error()
			code = http.StatusInternalServerError
			break
		}
		text = string(resp)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_, err := io.Copy(w, bytes.NewBuffer([]byte(text)))
	if err != nil {
		klog.Errorf("httpJson copy response: %s", err.Error())
	}
	return code
}

func HttpJsonCode(w http.ResponseWriter, v interface{}, code int) int {
	var text string
	switch v.(type) {
	case error:
		text = v.(error).Error()
	case string:
		text = v.(string)
	default:
		resp, err := json.Marshal(v)
		if err != nil {
			text = err.Error()
			break
		}
		text = string(resp)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_, err := io.Copy(w, bytes.NewBuffer([]byte(text)))
	if err != nil {
		klog.Errorf("httpJsonCode copy response: %s", err.Error())
	}
	return code
}
