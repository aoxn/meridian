package apis

import (
	"encoding/json"
	"github.com/aoxn/meridian/internal/vmm/meta"
	"github.com/gorilla/mux"
	"io"
	"k8s.io/klog/v2"
	"net/http"
)

func debug(r *http.Request, w http.ResponseWriter) int {
	return 0
}

func createVm(r *http.Request, w http.ResponseWriter) int {
	return 0
}

func getVm(r *http.Request, w http.ResponseWriter) int {
	name := mux.Vars(r)["name"]
	machine := meta.Local.Machine()
	switch name {
	case "":
		klog.Infof("handler: list vm")
		vms, err := machine.List()
		if err != nil {
			return httpJson(w, err)
		}
		return httpJson(w, vms)
	default:
	}
	vm, err := machine.Get(name)
	if err != nil {
		return httpJson(w, err)
	}
	return httpJson(w, vm)
}

func decodeBody(body io.ReadCloser, v interface{}) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, v)
}
