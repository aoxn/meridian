package daemon

import (
	"context"
	"fmt"
	"github.com/aoxn/meridian/internal/daemon/apis"
	"github.com/aoxn/meridian/internal/daemon/core"
	"github.com/aoxn/meridian/internal/tool/server"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"
	"os"
	"os/signal"
	"syscall"
)

type Configuration struct {
}

func NewApp(ctx context.Context, cfg *Configuration) App {
	scfg := &server.Config{
		Vsock: false, // listen on vsock
		//BindAddr: ":30443",
		BindAddr: "/tmp/meridian.sock",
	}
	mgr, err := core.NewContext()
	if err != nil {
		panic(fmt.Errorf("init core context failed: %v", err))
	}
	app := App{
		cfg: cfg,
		ctx: ctx,
		svr: server.NewOrDie(ctx, scfg, apis.CoreRoute(mgr)),
	}
	return app
}

type App struct {
	ctx    context.Context
	cfg    *Configuration
	svr    *server.Server
	appCtx *core.Context
}

func (ap *App) GetConfig() *Configuration {
	return ap.cfg
}

func (ap *App) Start() error {
	sigchan := make(chan os.Signal, 10)
	signal.Notify(sigchan, os.Interrupt, os.Kill, syscall.SIGTERM)
	defer ap.svr.CleanUp()
	err := ap.svr.Start(ap.ctx)
	if err != nil {
		return errors.Wrapf(err, "start server failed")
	}

	for {
		klog.Infof("waiting for signal")
		select {
		case sig := <-sigchan:
			klog.Infof("received signal: %s", sig.String())
			return nil
		}
	}
}
