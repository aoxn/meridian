package model

import (
	"errors"
	"fmt"
	"github.com/aoxn/meridian/api/v1"
	"github.com/containerd/containerd/identifiers"
	"os"
	"path/filepath"
	"strings"
)

// DotMeridian is a directory that appears under the home directory.
const DotMeridian = ".meridian"

// MdHOME returns the abstract path of `~/.meridian` (or $MERIDIAN_HOME, if set).
//
// We use `~/.meridian` so that we can have enough space for the length of the socket path,
// which can be only 104 characters on macOS.
func MdHOME() (string, error) {
	dir := os.Getenv("MERIDIAN_HOME")
	if dir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(homeDir, DotMeridian)
	}
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		return dir, os.MkdirAll(dir, 0755)
	}
	realdir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", fmt.Errorf("cannot evaluate symlinks in %q: %w", dir, err)
	}
	return realdir, nil
}

// MdConfigDir returns the path of the config directory, $MERIDIAN_HOME/_config.
func MdConfigDir() (string, error) {
	mDir, err := MdHOME()
	if err != nil {
		return "", err
	}
	return filepath.Join(mDir, v1.ConfigDir), nil
}

// MdNetworksDir returns the path of the networks log directory, $MERIDIAN_HOME/_networks.
func MdNetworksDir() (string, error) {
	mDir, err := MdHOME()
	if err != nil {
		return "", err
	}
	return filepath.Join(mDir, v1.NetworksDir), nil
}

// MdDisksDir returns the path of the disks directory, $MERIDIAN_HOME/_disks.
func MdDisksDir() (string, error) {
	mDir, err := MdHOME()
	if err != nil {
		return "", err
	}
	return filepath.Join(mDir, v1.DisksDir), nil
}

// MdImagesDir returns the path of the disks directory, $MERIDIAN_HOME/_disks.
func MdImagesDir() (string, error) {
	mDir, err := MdHOME()
	if err != nil {
		return "", err
	}
	return filepath.Join(mDir, v1.ImagesDir), nil
}

// MdCacheDir returns the path of the disks directory, $MERIDIAN_HOME/_cache.
func MdCacheDir() (string, error) {
	mDir, err := MdHOME()
	if err != nil {
		return "", err
	}
	return filepath.Join(mDir, v1.CacheDir), nil
}

// Directory returns the LimaDir.
func Directory() string {
	limaDir, err := MdHOME()
	if err != nil {
		return ""
	}
	return limaDir
}

// Validate checks the LimaDir.
func Validate() error {
	limaDir, err := MdHOME()
	if err != nil {
		return err
	}
	names, err := Instances()
	if err != nil {
		return err
	}
	for _, name := range names {
		// Each instance directory needs to have limayaml
		instDir := filepath.Join(limaDir, name)
		yamlPath := filepath.Join(instDir, v1.MeridianYAMLFile)
		if _, err := os.Stat(yamlPath); err != nil {
			return err
		}
	}
	return nil
}

// Instances returns the names of the instances under LimaDir.
func Instances() ([]string, error) {
	limaDir, err := MdHOME()
	if err != nil {
		return nil, err
	}
	limaDirList, err := os.ReadDir(limaDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, f := range limaDirList {
		if strings.HasPrefix(f.Name(), ".") || strings.HasPrefix(f.Name(), "_") {
			continue
		}
		if !f.IsDir() {
			continue
		}
		names = append(names, f.Name())
	}
	return names, nil
}

func Disks() ([]string, error) {
	limaDiskDir, err := MdDisksDir()
	if err != nil {
		return nil, err
	}
	limaDiskDirList, err := os.ReadDir(limaDiskDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, f := range limaDiskDirList {
		names = append(names, f.Name())
	}
	return names, nil
}

// InstanceDir returns the instance dir.
// InstanceDir does not check whether the instance exists.
func InstanceDir(name string) (string, error) {
	if err := identifiers.Validate(name); err != nil {
		return "", err
	}
	limaDir, err := MdHOME()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(limaDir, name)
	return dir, nil
}

func ImageId(arch, name string) (string, error) {
	if err := identifiers.Validate(name); err != nil {
		return "", err
	}
	limaDir, err := MdImagesDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(limaDir, arch, name)
	return dir, nil
}

func DiskDir(name string) (string, error) {
	if err := identifiers.Validate(name); err != nil {
		return "", err
	}
	limaDisksDir, err := MdDisksDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(limaDisksDir, name)
	return dir, nil
}

// UnixPathMax is the value of UNIX_PATH_MAX.
const UnixPathMax = 108
