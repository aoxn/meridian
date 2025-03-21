// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package cidata

import (
	"io"
	"os/user"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestTemplate(t *testing.T) {
	args := TemplateArgs{
		Name: "default",
		User: &user.User{
			Uid:      "501",
			Username: "foo",
		},
		Home: "/home/foo.linux",
		SSHPubKeys: []string{
			"ssh-rsa dummy foo@example.com",
		},
		Mounts: []Mount{
			{MountPoint: "/Users/dummy"},
			{MountPoint: "/Users/dummy/lima"},
		},
		MountType: "reverse-sshfs",
		CACerts: CACerts{
			Trusted: []Cert{},
		},
	}
	layout, err := ExecuteTemplate(args)
	assert.NilError(t, err)
	for _, f := range layout {
		t.Logf("=== %q ===", f.path)
		b, err := io.ReadAll(f.reader)
		assert.NilError(t, err)
		t.Log(string(b))
		if f.path == "user-data" {
			// mounted later
			assert.Assert(t, !strings.Contains(string(b), "mounts:"))
			// ca_certs:
			assert.Assert(t, !strings.Contains(string(b), "trusted:"))
		}
	}
}

func TestTemplate9p(t *testing.T) {
	args := TemplateArgs{
		Name: "default",
		User: &user.User{
			Username: "foo",
			Uid:      "501",
		},
		Home: "/home/foo.linux",
		SSHPubKeys: []string{
			"ssh-rsa dummy foo@example.com",
		},
		Mounts: []Mount{
			{Tag: "mount0", MountPoint: "/Users/dummy", Type: "9p", Options: "ro,trans=virtio"},
			{Tag: "mount1", MountPoint: "/Users/dummy/lima", Type: "9p", Options: "rw,trans=virtio"},
		},
		MountType: "9p",
	}
	layout, err := ExecuteTemplate(args)
	assert.NilError(t, err)
	for _, f := range layout {
		t.Logf("=== %q ===", f.path)
		b, err := io.ReadAll(f.reader)
		assert.NilError(t, err)
		t.Log(string(b))
		if f.path == "user-data" {
			// mounted at boot
			assert.Assert(t, strings.Contains(string(b), "mounts:"))
		}
	}
}
