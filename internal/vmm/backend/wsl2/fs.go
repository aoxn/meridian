// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package wsl2

import (
	"context"
	"errors"
	"fmt"
	"github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/tool/downloader"
	"k8s.io/klog/v2"
	"os"
	"path"
	"path/filepath"

	"github.com/aoxn/meridian/internal/vmm/backend"
)

// EnsureFs downloads the root fs.
func EnsureFs(ctx context.Context, driver *backend.BaseDriver) error {
	baseDisk := filepath.Join(driver.I.Dir(), v1.BaseDisk)
	if _, err := os.Stat(baseDisk); errors.Is(err, os.ErrNotExist) {
		f := v1.FindImage(driver.I.Spec.Image.Name)
		if f == nil {
			return fmt.Errorf("unexpected image name: [%s]", driver.I.Spec.Image.Name)
		}
		_, err := downloader.Download(ctx, baseDisk, f.Location,
			downloader.WithCache(),
			downloader.WithDecompress(true),
			downloader.WithDescription(fmt.Sprintf("%s (%s)", "the image", path.Base(f.Location))),
			downloader.WithExpectedDigest(f.Digest),
		)
		if err != nil {
			return fmt.Errorf("failed to download %q: %w", f.Location, err)
		}
	}
	klog.Info("Download succeeded")

	return nil
}
