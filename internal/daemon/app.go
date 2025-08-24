package daemon

import (
	"context"
	"github.com/aoxn/meridian/internal/daemon/apis"
	"github.com/aoxn/meridian/internal/tool/server"
)

type Configuration struct {
}

func NewApp(ctx context.Context, cfg *Configuration) App {
	scfg := &server.Config{
		Vsock: false, // listen on vsock
		//BindAddr: ":30443",
		BindAddr: "/tmp/meridian.sock",
	}
	app := App{
		cfg: cfg,
		svr: server.NewOrDie(ctx, scfg, apis.Routers),
	}
	return app
}

type App struct {
	ctx context.Context
	cfg *Configuration
	svr *server.Server
}

func (ap *App) GetConfig() *Configuration {
	return ap.cfg
}

func (ap *App) Start() error {
	return ap.svr.Start(ap.ctx)
}
