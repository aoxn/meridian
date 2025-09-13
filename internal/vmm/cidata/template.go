// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package cidata

import (
	"bytes"
	"embed"
	"fmt"
	"github.com/aoxn/meridian/internal/vmm/meta"
	"github.com/aoxn/meridian/internal/vmm/sshutil"
	"github.com/pkg/errors"
	"io"
	"io/fs"
	"k8s.io/klog/v2"
	"os/user"
	"text/template"
	"time"
)

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
	IpAddress  string
	IpGateway  string
}
type Mount struct {
	Tag        string
	MountPoint string // abs Path, accessible by the User
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

type ValidateFn func(tpl *TemplateArgs) error

func NewTpl(ii *meta.Machine, pub []sshutil.PubKey) (*TemplateArgs, error) {
	u := &user.User{
		Uid:      "1000",
		Username: ii.Name,
		HomeDir:  fmt.Sprintf("/home/%s", ii.Name),
	}
	var pubs []string
	for _, p := range pub {
		pubs = append(pubs, p.Content)
	}

	vmInfo := ii.Spec
	tplModel := TemplateArgs{
		Name:       ii.Name,
		User:       u,
		VMType:     string(vmInfo.VMType),
		TimeZone:   vmInfo.TimeZone,
		SSHPubKeys: pubs,
		MountType:  "virtiofs",
		CACerts: CACerts{
			RemoveDefaults: false,
		},
		Home: u.HomeDir,
	}
	for k, n := range vmInfo.Mounts {
		mount := Mount{
			MountPoint: n.MountPoint,
			Type:       "virtiofs",
			Tag:        fmt.Sprintf("mount%d", k),
		}
		if vmInfo.VMType == "vz" {
			mount.Type = "virtiofs"
		}
		tplModel.Mounts = append(tplModel.Mounts, mount)
	}

	for _, n := range vmInfo.Networks {
		network := Network{
			Interface:  "enp0s1",
			MACAddress: n.MACAddress,
			IpAddress:  n.Address,
			IpGateway:  n.IpGateway,
		}
		tplModel.Networks = append(tplModel.Networks, network)
	}
	klog.Infof("network addresses: %+v", tplModel.Networks[0])
	// change instance id on every boot so network config will be processed again
	tplModel.IID = fmt.Sprintf("iid-%d", time.Now().Unix())
	return &tplModel, nil
}

func (tpl *TemplateArgs) Build(top *embed.FS, root string, validateFn ValidateFn) ([]*Entry, error) {
	err := validateFn(tpl)
	if err != nil {
		return nil, err
	}

	fsys, err := fs.Sub(top, root)
	if err != nil {
		return nil, errors.Wrapf(err, "unexpected root %q", root)
	}

	render := func(tmpl string, args interface{}) ([]byte, error) {
		tp, err := template.
			New("").
			Parse(tmpl)
		if err != nil {
			return nil, err
		}
		var b bytes.Buffer
		err = tp.Execute(&b, args)
		if err != nil {
			return nil, err
		}
		return b.Bytes(), nil
	}
	var layout []*Entry
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
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		b, err := render(string(data), tpl)
		if err != nil {
			return err
		}
		layout = append(layout, &Entry{
			Path:   path,
			reader: bytes.NewReader(b),
		})
		return nil
	}

	err = fs.WalkDir(fsys, ".", walkFn)
	if err != nil {
		return nil, errors.Wrapf(err, "walk embed fs")
	}

	return layout, nil
}

type Entry struct {
	Path   string
	reader io.Reader
	closer io.Closer
}
