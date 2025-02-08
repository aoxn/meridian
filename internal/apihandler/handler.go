package apihandler

import (
	"context"
	"fmt"
	api "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/apihandler/resource/image"
	"github.com/aoxn/meridian/internal/apihandler/resource/task"
	"github.com/aoxn/meridian/internal/apihandler/resource/vm"
	"github.com/aoxn/meridian/internal/server"
	"github.com/aoxn/meridian/internal/server/service"
	"github.com/aoxn/meridian/internal/server/service/universal"
	"github.com/aoxn/meridian/internal/tool/cmd"
	"github.com/aoxn/meridian/internal/vma/download"
	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
)

func RunDaemonAPI(ctx context.Context) error {
	var (
		err error
		cfg = &server.Config{
			Vsock: false, // listen on vsock
			//BindAddr: ":30443",
			BindAddr: "/tmp/meridian.sock",
		}
	)
	klog.Infof("meridiand daemon: uid=%d", os.Getuid())
	option := &service.Options{
		Provider: api.AuthInfo{
			Type: "Local",
		},
		Scheme: scheme.Scheme,
	}
	err = api.AddToScheme(option.Scheme)
	if err != nil {
		klog.Infof("failed to add api scheme: %v", err.Error())
		return err
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	for _, pvk := range []service.Provider{
		task.NewTaskPvd(option),
		image.NewImagePvd(option),
		universal.NewUniversalPvd(option),
		vm.NewVirtualMachinePvd(option),
	} {
		grp, err := pvk.NewAPIGroup(cancelCtx)
		if err != nil {
			panic(fmt.Sprintf("pvd %s new api group error: %v", pvk, err))
		}
		service.APIGroup.AddGroupOrDie(grp)
	}
	service.APIGroup.Debug()
	handler := server.NewHandler(service.APIGroup, option.Scheme)
	damon := server.NewOrDie(context.TODO(), cfg, handler.Routes())
	damon.AddRoute(map[string]map[string]server.HandlerFunc{
		"GET": {
			"/health": func(contex context.Context, w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("ok"))
			},
		},
	})
	err = damon.Start(cancelCtx)
	if err != nil {
		cancel()
		return err
	}
	signalFunc := func() {
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
	go signalFunc()
	go EnsureDocker(cancelCtx)
	klog.Infof("waiting for incoming signal")
	select {
	case <-cancelCtx.Done():
		damon.CleanUp()
	}
	return nil
}

func EnsureDocker(ctx context.Context) {
	err := ensureBinary(ctx, "docker")
	if err != nil {
		klog.Errorf("failed to install docker: %v", err)
	}
	err = ensureBinary(ctx, "kubectl")
	if err != nil {
		klog.Errorf("failed to install kubectl: %v", err)
	}
}

func ensureBinary(ctx context.Context, bin string) error {
	klog.Infof("try to install [%s] binary", bin)
	switch runtime.GOOS {
	case "darwin":
		dest := "/usr/local/bin"
		cm := cmd.NewCmd(
			"ls", filepath.Join(dest, bin),
		)
		result := <-cm.Start()
		err := cmd.CmdError(result)
		if err != nil {
			klog.Warningf("find %s command failed: %v", bin, err)
			f, err := api.FindBinary(bin, api.NewArch(runtime.GOARCH))
			if err != nil {
				return errors.Wrapf(err, "find %s runtime location", bin)
			}
			_ = os.MkdirAll(dest, 0775)
			klog.Infof("%s command not found, try install", bin)
			_, err = download.DownloadFile(ctx, filepath.Join(dest, bin), f, true, "bin tgz", api.NewArch(runtime.GOARCH))
			if err != nil {
				return errors.Wrapf(err, "download %s binary", bin)
			}
			cm = cmd.NewCmd("chmod", "+x", filepath.Join(dest, bin))
			result = <-cm.Start()
			return cmd.CmdError(result)
		}
		klog.Infof("%s command found: %s", bin, filepath.Join(dest, bin))
	case "linux":

	case "windows":

	}
	return nil
}
