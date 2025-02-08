//go:build windows
// +build windows

package file

import (
	v1 "github.com/aoxn/meridian/api/v1"
	"path/filepath"
)

type action struct {
	files []Transfer
}

// NewAction returns a new action for kubeadm init
func NewAction(files []Transfer) actions.Action {
	return &action{files: files}
}

// Execute runs the action
func (a *action) Execute(ctx *v1.Request) error {

	return nil
}

func WgetPath(f Transfer) string { return filepath.Join(f.Cache, filepath.Base(f.URI())) }

func UntarPath(f Transfer) string { return filepath.Join(f.Cache, "untar") }
