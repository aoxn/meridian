package vz

import (
	"context"
	"errors"
	"fmt"
	"github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/tool/iso9660util"
	"github.com/aoxn/meridian/internal/vma/backend"
	"github.com/aoxn/meridian/internal/vma/download"
	nativeimg "github.com/aoxn/meridian/internal/vma/nativeimg"
	"os"
	"path/filepath"

	"github.com/docker/go-units"
)

func EnsureDisk(ctx context.Context, driver *backend.BaseDriver) error {
	diffDisk := filepath.Join(driver.I.Dir, v1.DiffDisk)
	if _, err := os.Stat(diffDisk); err == nil || !errors.Is(err, os.ErrNotExist) {
		// disk is already ensured
		return err
	}

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
	diskSize, _ := units.RAMInBytes(driver.Yaml.Spec.Disk)
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
