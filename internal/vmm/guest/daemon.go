package guest

import (
	"bytes"
	"context"
	"fmt"
	"github.com/aoxn/meridian/internal/tool/server"
	"github.com/aoxn/meridian/internal/vmm/forward"
	"github.com/aoxn/meridian/internal/vmm/guest/api"
	"github.com/pkg/errors"
	"io"
	"k8s.io/klog/v2"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func RunDaemonAPI() error {
	cfg := &server.Config{
		Vsock:    true, // listen on vsock
		BindAddr: "10443",
	}

	cancelCtx, cancel := context.WithCancel(context.TODO())

	damon := server.NewOrDie(context.TODO(), cfg, newRoute())
	damon.AddRoute(newHealth())

	err := damon.Start(cancelCtx)
	if err != nil {
		cancel()
		return err
	}

	sigFunction := func() {
		sigchan := make(chan os.Signal, 1)
		signal.Notify(sigchan, os.Interrupt, os.Kill, syscall.SIGTERM)
		for {
			select {
			case sig := <-sigchan:
				cancel()
				klog.Infof("received signal: %s", sig.String())
				return
			}
		}
	}
	go sigFunction()
	fwd := forward.NewForwardMgr()
	err = fwd.AddBy(newDockerForwardRule())
	if err != nil {
		return errors.Wrap(err, "add docker rule")
	}

	err = fwd.AddBy(newAPIServerForwardRule())
	if err != nil {
		return errors.Wrap(err, "add api server rule")
	}
	klog.Infof("waiting for incoming signal")
	select {
	case <-cancelCtx.Done():
		damon.CleanUp()
	}
	return nil
}

func newDockerForwardRule() string {
	return fmt.Sprintf("vsock://%d->unix:///var/run/docker.sock", 10240)
}

func newAPIServerForwardRule() string {
	return fmt.Sprintf("vsock://%d->tcp://127.0.0.1:6443", 40443)
}

func newHealth() map[string]map[string]server.HandlerFunc {
	return map[string]map[string]server.HandlerFunc{
		"GET": {
			"/healthz": func(r *http.Request, w http.ResponseWriter) int {
				_, _ = w.Write([]byte("ok"))
				return http.StatusOK
			},
		},
	}
}

func debug(r *http.Request, w http.ResponseWriter) int {
	klog.Infof("debug request: %s", r.RequestURI)
	return http.StatusOK
}

func health(r *http.Request, w http.ResponseWriter) int {
	header := map[string]string{
		"Content-Type": "application/json; charset=utf-8",
	}
	for key, val := range header {
		w.Header().Set(key, val)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, bytes.NewBuffer([]byte("ok")))
	return http.StatusOK
}

func newRoute() map[string]map[string]server.HandlerFunc {

	route := map[string]map[string]server.HandlerFunc{
		"GET": {
			"/health":            health,
			"/api/v1/guest/{id}": api.GetGI,
			"/api/v1/guest":      api.GetGI,
		},
		"PUT": {},
		"POST": {
			"/api/v1/guest": api.GetGI,
		},
		"DELETE": {
			"/api/v1/guest/{id}": api.GetGI,
		},
	}
	return route
}
