package core

import (
	v1 "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/tool"
	"github.com/aoxn/meridian/internal/vmm/meta"
	"testing"
)

func TestAlloc(t *testing.T) {
	mchs, err := meta.Local.Machine().List()
	if err != nil {
		t.Fatalf("list machines: %s", err)
	}
	var vm = meta.Machine{
		Name: "abc",
		Spec: &v1.VirtualMachineSpec{
			Networks: []v1.Network{
				{
					VZNAT: true,
				},
			},
		},
	}
	err = allocateAddress(&vm, mchs)
	if err != nil {
		t.Fatalf("allocate: %s", err)
	}
	t.Logf("address: %s", tool.PrettyJson(vm))
}
