// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"context"
	"errors"
	"fmt"
	"github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/tool/downloader"
	"github.com/aoxn/meridian/internal/tool/iso9660util"
	"github.com/aoxn/meridian/internal/vmm/backend"
	nativeimg "github.com/aoxn/meridian/internal/vmm/nativeimg"
	"k8s.io/klog/v2"
	"os"
	"path"
	"path/filepath"

	"github.com/docker/go-units"
)

func EnsureDisk(ctx context.Context, driver *backend.BaseDriver) error {
	diffDisk := filepath.Join(driver.I.Dir(), v1.DiffDisk)
	if _, err := os.Stat(diffDisk); err == nil || !errors.Is(err, os.ErrNotExist) {
		// disk is already ensured
		return err
	}
	vmInfo := driver.I.Spec
	baseDisk := filepath.Join(driver.I.Dir(), v1.BaseDisk)
	if _, err := os.Stat(baseDisk); errors.Is(err, os.ErrNotExist) {
		f := v1.FindImage(driver.I.Spec.Image.Name)
		if f == nil {
			return fmt.Errorf("unexpected image name: [%s]", driver.I.Spec.Image.Name)
		}
		if f.Arch != vmInfo.Arch {
			return fmt.Errorf("%q: unsupported arch: %q, expected=%q", f.Location, f.Arch, vmInfo.Arch)
		}
		res, err := downloader.Download(ctx, baseDisk, f.Location,
			downloader.WithCache(),
			downloader.WithDecompress(true),
			downloader.WithDescription(fmt.Sprintf("%s (%s)", "guest vm image", path.Base(f.Location))),
			downloader.WithExpectedDigest(f.Digest),
		)
		if err != nil {
			return fmt.Errorf("failed to download %q: %w", f.Location, err)
		}
		klog.Infof("download base disk for image: %s, from %s, [%s]", vmInfo.Image.Name, f.Location, res.Status)
	}
	diskSize, _ := units.RAMInBytes(driver.I.Spec.Disk)
	if diskSize == 0 {
		return nil
	}
	isBaseDiskISO, err := iso9660util.IsISO9660(baseDisk)
	if err != nil {
		return err
	}
	if isBaseDiskISO {
		// Create an empty data volume (sparse)
		diffDiskF, err := os.Create(diffDisk)
		if err != nil {
			return err
		}
		if err = nativeimg.MakeSparse(diffDiskF, diskSize); err != nil {
			diffDiskF.Close()
			return err
		}
		return diffDiskF.Close()
	}
	if err = nativeimg.ConvertToRaw(baseDisk, diffDisk, &diskSize, false); err != nil {
		return fmt.Errorf("failed to convert %q to a raw disk %q: %w", baseDisk, diffDisk, err)
	}
	return err
}
