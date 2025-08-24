//go:build darwin && arm64 && !no_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"fmt"
	"k8s.io/klog/v2"
	"os/exec"
	"strings"

	"github.com/Code-Hex/vz/v3"
	"github.com/coreos/go-semver/semver"
)

func createRosettaDirectoryShareConfiguration() (*vz.VirtioFileSystemDeviceConfiguration, error) {
	config, err := vz.NewVirtioFileSystemDeviceConfiguration("vz-rosetta")
	if err != nil {
		return nil, fmt.Errorf("failed to create a new virtio file system configuration: %w", err)
	}
	availability := vz.LinuxRosettaDirectoryShareAvailability()
	switch availability {
	case vz.LinuxRosettaAvailabilityNotSupported:
		return nil, errRosettaUnsupported
	case vz.LinuxRosettaAvailabilityNotInstalled:
		klog.Info("Installing rosetta...")
		klog.Info("Hint: try `softwareupdate --install-rosetta` if Lima gets stuck here")
		if err := vz.LinuxRosettaDirectoryShareInstallRosetta(); err != nil {
			return nil, fmt.Errorf("failed to install rosetta: %w", err)
		}
		klog.Info("Rosetta installation complete.")
	case vz.LinuxRosettaAvailabilityInstalled:
		// nothing to do
	}

	rosettaShare, err := vz.NewLinuxRosettaDirectoryShare()
	if err != nil {
		return nil, fmt.Errorf("failed to create a new rosetta directory share: %w", err)
	}
	macOSProductVersion, err := ProductVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get macOS product version: %w", err)
	}
	if !macOSProductVersion.LessThan(*semver.New("14.0.0")) {
		cachingOption, err := vz.NewLinuxRosettaAbstractSocketCachingOptions("rosetta")
		if err != nil {
			return nil, fmt.Errorf("failed to create a new rosetta directory share caching option: %w", err)
		}
		rosettaShare.SetOptions(cachingOption)
	}
	config.SetDirectoryShare(rosettaShare)

	return config, nil
}

// ProductVersion returns the macOS product version like "12.3.1".
func ProductVersion() (*semver.Version, error) {
	cmd := exec.Command("sw_vers", "-productVersion")
	// output is like "12.3.1\n"
	b, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute %v: %w", cmd.Args, err)
	}
	verTrimmed := strings.TrimSpace(string(b))
	// macOS 12.4 returns just "12.4\n"
	for strings.Count(verTrimmed, ".") < 2 {
		verTrimmed += ".0"
	}
	verSem, err := semver.NewVersion(verTrimmed)
	if err != nil {
		return nil, fmt.Errorf("failed to parse macOS version %q: %w", verTrimmed, err)
	}
	return verSem, nil
}
