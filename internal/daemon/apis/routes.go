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
	v := newVmHandler(ctx)
	d := newDockerHandler(ctx)
	k := newK8sHandler(ctx)
	i := newImageHandler(ctx)
	var r = map[string]map[string]server.HandlerFunc{
		"PUT": {
			"/api/v1/vm/start/{name}":        v.startVm,
			"/api/v1/vm/stop/{name}":         v.stopVm,
			"/api/v1/k8s/redeploy/{name}":    k.redeploy,
			"/api/v1/docker/redeploy/{name}": v.debug,
		},
		"POST": {
			"/api/v1/docker/{name}": d.create,
			"/api/v1/k8s/{name}":    k.create,
			"/api/v1/vm/run/{name}": v.runVm,
			"/api/v1/vm/{name}":     v.createVm,
		},
		"DELETE": {
			"/api/v1/docker/{name}": d.destroy,
			"/api/v1/k8s/{name}":    k.destroy,
			"/api/v1/vm/{name}":     v.deleteVm,
			"/api/v1/image/{name}":  i.delete,
		},
		"GET": {
			"/api/v1/docker/{name}":     d.get,
			"/api/v1/docker":            d.get,
			"/api/v1/k8s/{name}":        k.get,
			"/api/v1/k8s":               k.get,
			"/debug":                    v.debug,
			"/api/v1/vm/{name}":         v.getVm,
			"/api/v1/vm":                v.getVm,
			"/api/v1/image/pull/{name}": i.pull,
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

func httpJsonDirect(w http.ResponseWriter, v interface{}) int {
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
	_, err := io.Copy(w, bytes.NewBuffer([]byte(text)))
	if err != nil {
		klog.Errorf("httpJson copy response: %s", err.Error())
	}
	return code
}
