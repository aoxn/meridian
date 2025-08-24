package guest

import (
	"bytes"
	"context"
	"fmt"
	v1 "github.com/aoxn/meridian/api/v1"
	srv "github.com/aoxn/meridian/internal/server"
	"github.com/aoxn/meridian/internal/server/service"
	"github.com/aoxn/meridian/internal/server/service/universal"
	"github.com/aoxn/meridian/internal/vmm/forward"
	"github.com/aoxn/meridian/internal/vmm/guest/svc"
	"github.com/pkg/errors"
	"io"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func RunDaemonAPI() error {
	cfg := &srv.Config{
		Vsock:    true, // listen on vsock
		BindAddr: "10443",
	}

	option := &service.Options{
		Provider: v1.AuthInfo{
			Type: "Local",
		},
		Scheme: scheme.Scheme,
	}
	err := v1.AddToScheme(option.Scheme)
	if err != nil {
		klog.Infof("failed to add api scheme: %v", err.Error())
		return err
	}
	cancelCtx, cancel := context.WithCancel(context.TODO())

	pvd := universal.NewUniversalPvd(option)
	gst := svc.NewGuestInfoPvd(option)
	kube := svc.NewKubernetesPvd(option)
	for _, pvk := range []service.Provider{gst, pvd, kube} {
		grp, err := pvk.NewAPIGroup(cancelCtx)
		if err != nil {
			panic(fmt.Sprintf("pvd %s new api group error: %v", pvk, err))
		}
		service.APIGroup.AddGroupOrDie(grp)
	}

	service.APIGroup.Debug()
	handler := srv.NewHandler(service.APIGroup, option.Scheme)

	damon := srv.NewOrDie(context.TODO(), cfg, handler.Routes())
	damon.AddRoute(newHealth())

	err = damon.Start(cancelCtx)
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

func newHealth() map[string]map[string]srv.HandlerFunc {
	return map[string]map[string]srv.HandlerFunc{
		"GET": {
			"/health": func(contex context.Context, w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("ok"))
			},
		},
	}
}

func debug(contex context.Context, w http.ResponseWriter, r *http.Request) {
	klog.Infof("debug request: %s", r.RequestURI)
}

func health(_ context.Context, w http.ResponseWriter, r *http.Request) {
	header := map[string]string{
		"Content-Type": "application/json; charset=utf-8",
	}
	for key, val := range header {
		w.Header().Set(key, val)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, bytes.NewBuffer([]byte("ok")))
}

func newRoute() map[string]map[string]srv.HandlerFunc {

	route := map[string]map[string]srv.HandlerFunc{
		"GET": {
			"/health":                 health,
			"/api/v1/{resource}/{id}": debug,
			"/api/v1/{resource}":      debug,
		},
		"PUT": {},
		"POST": {
			"/api/v1/{resource}": debug,
		},
		"DELETE": {
			"/api/v1/{resource}/{id}": debug,
		},
	}
	return route
}
