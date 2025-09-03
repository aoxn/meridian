package meta

import "github.com/Code-Hex/vz/v3"

func GetLatestRestoreImageURL() (string, error) {
	return vz.GetLatestSupportedMacOSRestoreImageURL()
}
