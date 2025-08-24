//go:build !windows || no_wsl

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package wsl2

import (
	"context"
	"errors"
	"github.com/aoxn/meridian/internal/vmm/backend"
)

var ErrUnsupported = errors.New("vm driver 'wsl2' requires Windows 10 build 19041 or later (Hint: try recompiling Lima if you are seeing this error on Windows 10+)")

const Enabled = false

type MdWslDriver struct {
	*backend.BaseDriver
}

func New(driver *backend.BaseDriver) *MdWslDriver {
	return &MdWslDriver{
		BaseDriver: driver,
	}
}

func (l *MdWslDriver) Validate() error {
	return ErrUnsupported
}

func (l *MdWslDriver) CreateDisk(_ context.Context) error {
	return ErrUnsupported
}

func (l *MdWslDriver) Start(_ context.Context) (chan error, error) {
	return nil, ErrUnsupported
}

func (l *MdWslDriver) Stop(_ context.Context) error {
	return ErrUnsupported
}
