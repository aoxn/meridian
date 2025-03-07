// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package download

import (
	"context"
	"errors"
	"fmt"
	"github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/tool/downloader"
	"k8s.io/klog/v2"
	"path"
)

// ErrSkipped is returned when the downloader did not attempt to download the specified file.
var ErrSkipped = errors.New("skipped to download")

// DownloadFile downloads a file to the cache, optionally copying it to the destination. Returns path in cache.
func DownloadFile(ctx context.Context, dest string, f v1.File, decompress bool, description string, expectedArch v1.Arch) (string, error) {
	if f.Arch != expectedArch {
		return "", fmt.Errorf("%w: %q: unsupported arch: %q, expected=%q", ErrSkipped, f.Location, f.Arch, expectedArch)
	}
	klog.Infof("Attempting to download [%s],[%s],[%s],[%s]", f.Arch, f.Digest, f.Location, description)
	res, err := downloader.Download(ctx, dest, f.Location,
		downloader.WithCache(),
		downloader.WithDecompress(decompress),
		downloader.WithDescription(fmt.Sprintf("%s (%s)", description, path.Base(f.Location))),
		downloader.WithExpectedDigest(f.Digest),
	)
	if err != nil {
		return "", fmt.Errorf("failed to download %q: %w", f.Location, err)
	}
	klog.V(5).Infof("res.ValidatedDigest=%v", res.ValidatedDigest)
	switch res.Status {
	case downloader.StatusDownloaded:
		klog.Infof("Downloaded %s from %q", description, f.Location)
	case downloader.StatusUsedCache:
		klog.Infof("Using cache %q", res.CachePath)
	default:
		klog.Infof("Unexpected result from downloader.Download(): %+v", res)
	}
	return res.CachePath, nil
}

// CachedFile checks if a file is in the cache, validating the digest if it is available. Returns path in cache.
func CachedFile(f v1.File) (string, error) {
	res, err := downloader.Cached(f.Location,
		downloader.WithCache(),
		downloader.WithExpectedDigest(f.Digest))
	if err != nil {
		return "", fmt.Errorf("cache did not contain %q: %w", f.Location, err)
	}
	return res.CachePath, nil
}

// Errors compose multiple into a single error.
// Errors filters out ErrSkipped.
func Errors(errs []error) error {
	var finalErr error
	for _, err := range errs {
		if errors.Is(err, ErrSkipped) {
			klog.V(1).Infof("with error: %s", err.Error())
		} else {
			finalErr = errors.Join(finalErr, err)
		}
	}
	if len(errs) > 0 && finalErr == nil {
		// errs only contains ErrSkipped
		finalErr = fmt.Errorf("%v", errs)
	}
	return finalErr
}
