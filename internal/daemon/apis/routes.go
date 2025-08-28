package apis

import (
	"bytes"
	"encoding/json"
	"github.com/aoxn/meridian/internal/daemon/core"
	"github.com/aoxn/meridian/internal/tool/server"
	"io"
	"k8s.io/klog/v2"
	"net/http"
)

func CoreRoute(ctx *core.Context) map[string]map[string]server.HandlerFunc {
	vm := newVmHandler(ctx)
	d := newDockerHandler(ctx)
	var r = map[string]map[string]server.HandlerFunc{
		"GET": {
			"/debug":                vm.debug,
			"/api/v1/vm/{name}":     vm.getVm,
			"/api/v1/vm":            vm.getVm,
			"/api/v1/docker/{name}": d.get,
			"/api/v1/docker":        d.get,
		},
		"PUT": {
			"/api/v1/vm/start/{name}": vm.startVm,
			"/api/v1/vm/stop/{name}":  vm.stopVm,
		},
		"POST": {
			"/api/v1/vm/{name}":     vm.createVm,
			"/api/v1/vm/run/{name}": vm.runVm,
			"/api/v1/docker/{name}": d.create,
		},
		"DELETE": {
			"/api/v1/vm/{name}":     vm.deleteVm,
			"/api/v1/docker/{name}": d.destroy,
		},
	}
	return r
}

func httpJson(w http.ResponseWriter, v interface{}) int {
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

func httpJsonCode(w http.ResponseWriter, v interface{}, code int) int {
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
