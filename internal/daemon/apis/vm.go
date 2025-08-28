package apis

import (
	"fmt"
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/daemon/core"
	"github.com/aoxn/meridian/internal/tool/server"
	"github.com/aoxn/meridian/internal/vmm/meta"
	"github.com/gorilla/mux"
	"k8s.io/klog/v2"
	"net/http"
	"path"
)

func newVmHandler(ctx *core.Context) *vmhandler {
	return &vmhandler{ctx: ctx}
}

type vmhandler struct {
	ctx *core.Context
}

func (h *vmhandler) debug(r *http.Request, w http.ResponseWriter) int {
	return 0
}

func (h *vmhandler) createVm(r *http.Request, w http.ResponseWriter) int {
	name := mux.Vars(r)["name"]
	backend := h.ctx.Backend().Machine()
	switch name {
	case "":
		return httpJson(w, fmt.Errorf("unexpected empty name"))
	default:
	}
	_, err := backend.Get(name)
	if err == nil {
		return httpJson(w, fmt.Errorf("vm %s already exists", name))
	}
	var spec v1.VirtualMachineSpec
	err = server.DecodeBody(r.Body, &spec)
	if err != nil {
		return httpJson(w, err)
	}
	vm := &meta.Machine{
		Name:   name,
		Spec:   &spec,
		AbsDir: path.Join(backend.Dir(), name),
	}
	err = h.ctx.VMMgr().Create(r.Context(), vm)
	if err != nil {
		return httpJson(w, err)
	}
	return httpJsonCode(w, vm, http.StatusAccepted)
}

func (h *vmhandler) runVm(r *http.Request, w http.ResponseWriter) int {
	name := mux.Vars(r)["name"]
	backend := h.ctx.Backend().Machine()
	switch name {
	case "":
		return httpJson(w, fmt.Errorf("unexpected empty name"))
	default:
	}
	_, err := backend.Get(name)
	if err == nil {
		return httpJson(w, fmt.Errorf("vm %s already exists", name))
	}
	var spec v1.VirtualMachineSpec
	err = server.DecodeBody(r.Body, &spec)
	if err != nil {
		return httpJson(w, err)
	}
	vm := &meta.Machine{Name: name, Spec: &spec}
	err = h.ctx.VMMgr().Run(r.Context(), name, vm)
	if err != nil {
		return httpJson(w, err)
	}
	return httpJsonCode(w, vm, http.StatusAccepted)
}

func (h *vmhandler) startVm(r *http.Request, w http.ResponseWriter) int {
	name := mux.Vars(r)["name"]
	backend := h.ctx.Backend().Machine()
	switch name {
	case "":
		return httpJson(w, fmt.Errorf("unexpected empty name"))
	default:
	}
	vm, err := backend.Get(name)
	if err != nil {
		return httpJson(w, fmt.Errorf("start vm %s error: ", name))
	}
	err = h.ctx.VMMgr().Start(r.Context(), name)
	if err != nil {
		return httpJson(w, err)
	}
	return httpJsonCode(w, vm, http.StatusAccepted)
}

func (h *vmhandler) stopVm(r *http.Request, w http.ResponseWriter) int {
	name := mux.Vars(r)["name"]
	backend := h.ctx.Backend().Machine()
	switch name {
	case "":
		return httpJson(w, fmt.Errorf("unexpected empty name"))
	default:
	}
	vm, err := backend.Get(name)
	if err != nil {
		return httpJson(w, fmt.Errorf("stop vm %s error: ", name))
	}
	err = h.ctx.VMMgr().Stop(r.Context(), name)
	if err != nil {
		return httpJson(w, err)
	}
	return httpJsonCode(w, vm, http.StatusAccepted)
}

func (h *vmhandler) getVm(r *http.Request, w http.ResponseWriter) int {
	name := mux.Vars(r)["name"]
	machine := h.ctx.Backend().Machine()
	switch name {
	case "":
		vms, err := machine.List()
		if err != nil {
			return httpJson(w, err)
		}
		klog.Infof("handler: list vm, return count [%d]", len(vms))
		return httpJson(w, vms)
	default:
	}
	vm, err := machine.Get(name)
	if err != nil {
		return httpJson(w, err)
	}
	return httpJson(w, vm)
}

func (h *vmhandler) deleteVm(r *http.Request, w http.ResponseWriter) int {
	name := mux.Vars(r)["name"]
	machine := h.ctx.Backend().Machine()
	switch name {
	case "":
		return httpJson(w, fmt.Errorf("unexpected empty name"))
	default:
	}
	klog.Infof("delete vm [%s]", name)
	mch, err := machine.Get(name)
	if err != nil {
		return httpJson(w, fmt.Errorf("find vm %s failed: %s", name, err.Error()))
	}
	err = h.ctx.VMMgr().Destroy(r.Context(), mch.Name)
	if err != nil {
		return httpJson(w, err)
	}
	return httpJsonCode(w, mch, http.StatusOK)
}

func newDockerHandler(ctx *core.Context) *dockerHandler {
	return &dockerHandler{ctx: ctx}
}

type dockerHandler struct {
	ctx *core.Context
}

func (h *dockerHandler) debug(r *http.Request, w http.ResponseWriter) int {
	return 0
}

func (h *dockerHandler) create(r *http.Request, w http.ResponseWriter) int {
	name := mux.Vars(r)["name"]
	switch name {
	case "":
		return httpJson(w, fmt.Errorf("unexpected empty name"))
	default:
	}
	err := h.ctx.DockerMgr().Create(r.Context(), name)
	if err != nil {
		return httpJson(w, err)
	}
	d, err := h.ctx.Backend().Docker().Get(name)
	if err != nil {
		return httpJson(w, err)
	}
	return httpJsonCode(w, d, http.StatusAccepted)
}

func (h *dockerHandler) destroy(r *http.Request, w http.ResponseWriter) int {
	name := mux.Vars(r)["name"]
	switch name {
	case "":
		return httpJson(w, fmt.Errorf("unexpected empty name"))
	default:
	}
	d := meta.Docker{Name: name}
	docker := h.ctx.Backend().Docker()
	err := h.ctx.DockerMgr().Destroy(r.Context(), name)
	if err != nil {
		return httpJson(w, err)
	}
	err = docker.Remove(&d)
	if err != nil {
		return httpJson(w, err)
	}
	return httpJsonCode(w, d, http.StatusAccepted)
}

func (h *dockerHandler) get(r *http.Request, w http.ResponseWriter) int {
	name := mux.Vars(r)["name"]
	machine := h.ctx.Backend().Docker()
	switch name {
	case "":
		vms, err := machine.List()
		if err != nil {
			return httpJson(w, err)
		}
		klog.Infof("handler: list docker, return count [%d]", len(vms))
		return httpJson(w, vms)
	default:
	}
	vm, err := machine.Get(name)
	if err != nil {
		return httpJson(w, err)
	}
	return httpJson(w, vm)
}
