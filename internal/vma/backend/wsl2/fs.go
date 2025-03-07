// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package wsl2

import (
	"context"
	"errors"
	"fmt"
	"github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/vma/download"
	"k8s.io/klog/v2"
	"os"
	"path/filepath"

	"github.com/aoxn/meridian/internal/vma/backend"
)

// EnsureFs downloads the root fs.
func EnsureFs(ctx context.Context, driver *backend.BaseDriver) error {
	baseDisk := filepath.Join(driver.I.Dir, v1.BaseDisk)
	if _, err := os.Stat(baseDisk); errors.Is(err, os.ErrNotExist) {
		f := v1.FindImage(driver.Yaml.Spec.Image.Name)
		if f == nil {
			return fmt.Errorf("unexpected image name: [%s]", driver.Yaml.Spec.Image.Name)
		}
		if _, err := download.DownloadFile(ctx, baseDisk, *f, true, "the image", driver.Yaml.Spec.Arch); err != nil {
			return err
		}
	}
	klog.Info("Download succeeded")

	return nil
}
