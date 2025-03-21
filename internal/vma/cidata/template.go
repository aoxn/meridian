// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package cidata

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os/user"
	"path"
	"text/template"
)

//go:embed cidata.TEMPLATE.d
var templateFS embed.FS

const templateFSRoot = "cidata.TEMPLATE.d"

type CACerts struct {
	RemoveDefaults bool
	Trusted        []Cert
}

type Cert struct {
	Lines []string
}

type Containerd struct {
	System bool
	User   bool
}
type Network struct {
	MACAddress string
	Interface  string
}
type Mount struct {
	Tag        string
	MountPoint string // abs path, accessible by the User
	Type       string
	Options    string
}
type BootCmds struct {
	Lines []string
}
type Disk struct {
	Name   string
	Device string
	Format bool
	FSType string
	FSArgs []string
}
type TemplateArgs struct {
	Name               string     // instance name
	IID                string     // instance id
	User               *user.User // user name
	Home               string     // home directory
	SSHPubKeys         []string
	Mounts             []Mount
	MountType          string
	Disks              []Disk
	Networks           []Network
	Env                map[string]string
	DNSAddresses       []string
	CACerts            CACerts
	HostHomeMountPoint string
	BootCmds           []BootCmds
	VMType             string
	VSockPort          int
	VirtioPort         string
	TimeZone           string
}

func ValidateTemplateArgs(args TemplateArgs) error {
	if args.User.Username == "root" {
		return errors.New("field User must not be \"root\"")
	}
	if args.User.Uid == "" {
		return errors.New("field UID must not be 0")
	}
	if args.Home == "" {
		return errors.New("field Home must be set")
	}
	if len(args.SSHPubKeys) == 0 {
		return errors.New("field SSHPubKeys must be set")
	}
	for i, m := range args.Mounts {
		f := m.MountPoint
		if !path.IsAbs(f) {
			return fmt.Errorf("field mounts[%d] must be absolute, got %q", i, f)
		}
	}
	return nil
}

func ExecuteTemplate(args TemplateArgs) ([]entry, error) {
	if err := ValidateTemplateArgs(args); err != nil {
		return nil, err
	}

	fsys, err := fs.Sub(templateFS, templateFSRoot)
	if err != nil {
		return nil, err
	}

	var layout []entry
	walkFn := func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("got non-regular file %q", path)
		}
		templateB, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		b, err := executeTemplate(string(templateB), args)
		if err != nil {
			return err
		}
		layout = append(layout, entry{
			path:   path,
			reader: bytes.NewReader(b),
		})
		return nil
	}

	if err := fs.WalkDir(fsys, ".", walkFn); err != nil {
		return nil, err
	}

	return layout, nil
}

// executeTemplate executes a text/template template.
func executeTemplate(tmpl string, args interface{}) ([]byte, error) {
	x, err := template.New("").Parse(tmpl)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	if err := x.Execute(&b, args); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
