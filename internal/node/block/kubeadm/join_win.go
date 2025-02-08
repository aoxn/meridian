//go:build windows
// +build windows

/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package kubeadmin implements the kubeadm join joinBlock
package kubeadm

import v1 "github.com/aoxn/meridian/api/v1"

type ActionJoin struct{}

// NewJoinBlock returns a new joinBlock for kubeadm init
func NewActionJoin() actions.Action {
	return &ActionJoin{}
}

// Execute runs the joinBlock
func (a *ActionJoin) Execute(ctx *v1.Request) error {
	return nil
}
