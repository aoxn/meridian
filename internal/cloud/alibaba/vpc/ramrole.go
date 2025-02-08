package vpc

import (
	"context"
	"fmt"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ram"
	"github.com/aoxn/meridian/internal/cloud"
	"github.com/aoxn/meridian/internal/cloud/alibaba/client"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"
	"strings"
)

var _ cloud.IRamRole = &ramrole{}

func NewRamrole(mgr *client.ClientMgr) cloud.IRamRole {
	return &ramrole{ClientMgr: mgr}
}

type ramrole struct {
	*client.ClientMgr
}

func (n *ramrole) FindRAM(ctx context.Context, id cloud.Id) (cloud.RamModel, error) {
	model := cloud.RamModel{}
	if id.Name == "" {
		return model, fmt.Errorf("ram role name must not be empty")
	}
	req := ram.CreateGetRoleRequest()
	req.RoleName = id.Name
	r, err := n.RAM.GetRole(req)
	if err != nil {
		if strings.Contains(err.Error(), "EntityNotExist.Role") {
			return model, cloud.NotFound
		}
		return model, err
	}
	if r.Role.RoleId == "" {
		return model, cloud.NotFound
	}
	klog.Infof("[service]debug ramrole %+v", r)
	model.RamId = r.Role.RoleId
	model.Arn = r.Role.Arn
	return model, nil
}

func (n *ramrole) ListRAM(ctx context.Context, id cloud.Id) ([]cloud.RamModel, error) {
	//TODO implement me
	panic("implement me")
}

func (n *ramrole) CreateRAM(ctx context.Context, m cloud.RamModel) (string, error) {
	if m.RamName == "" {
		return "", fmt.Errorf("create with empty ram name")
	}
	req := ram.CreateCreateRoleRequest()
	req.RoleName = m.RamName
	req.AssumeRolePolicyDocument = policy
	req.Description = "ecs worker node role"

	r, err := n.RAM.CreateRole(req)
	if err != nil {
		return "", errors.Wrapf(err, "create ram role")
	}

	return r.Role.RoleId, nil
}

func (n *ramrole) UpdateRAM(ctx context.Context, m cloud.RamModel) error {
	//TODO implement me
	panic("implement me")
}

func (n *ramrole) DeleteRAM(ctx context.Context, id cloud.Id, policyName string) error {
	if id.Name == "" {
		return fmt.Errorf("unexpected empty ramrole id")
	}
	dreq := ram.CreateDetachPolicyFromRoleRequest()
	dreq.PolicyName = policyName
	dreq.RoleName = id.Name
	dreq.PolicyType = "Custom"
	klog.Infof("detach policy from role: %s", id.Name)
	_, err := n.RAM.DetachPolicyFromRole(dreq)
	if err != nil {
		if !strings.Contains(err.Error(), "EntityNotExist") {
			return err
		}
		klog.Infof("detach policy from role: %s, already detached", id.Name)
	}
	klog.Infof("delete ram role: [%s]", id.Name)
	req := ram.CreateDeleteRoleRequest()
	req.RoleName = id.Name
	_, err = n.RAM.DeleteRole(req)
	if err != nil {
		if !strings.Contains(err.Error(), "EntityNotExist") {
			return err
		}
		klog.Infof("delete role: [%s] already not exists", id.Name)
	}
	klog.Infof("delete policy: [%s]", policyName)
	preq := ram.CreateDeletePolicyRequest()
	preq.PolicyName = policyName
	_, err = n.RAM.DeletePolicy(preq)
	if err != nil {
		if strings.Contains(err.Error(), "EntityNotExist") {
			klog.Infof("delete policy: policy [%s] already not exist", policyName)
			return nil
		}
	}
	return err
}

func (n *ramrole) FindPolicy(ctx context.Context, id cloud.Id) (cloud.RamModel, error) {
	var m cloud.RamModel
	policyName := id.Name
	if policyName == "" {
		return m, fmt.Errorf("ram policy name must not be empty")
	}
	req := ram.CreateGetPolicyRequest()
	req.PolicyName = policyName
	req.PolicyType = "Custom"
	r, err := n.RAM.GetPolicy(req)
	if err != nil {
		if strings.Contains(err.Error(), "EntityNotExist") {
			return m, cloud.NotFound
		}
		return m, err
	}
	if r.DefaultPolicyVersion.PolicyDocument == "" {
		return m, cloud.NotFound
	}
	klog.Infof("[service]found ram policy %s", policyName)
	return m, nil
}

func (n *ramrole) CreatePolicy(ctx context.Context, m cloud.RamModel) (cloud.RamModel, error) {
	if m.PolicyName == "" {
		return m, fmt.Errorf("create with empty policy name")
	}
	req := ram.CreateCreatePolicyRequest()
	req.PolicyName = m.PolicyName
	//req.PolicyType =     ram.Custom
	req.Description = "meridian ram policy"
	req.PolicyDocument = policyDoc
	klog.Infof("create ram policy: %s", m.PolicyName)
	_, err := n.RAM.CreatePolicy(req)
	if err != nil {
		return m, err
	}
	return n.AttachPolicyToRole(ctx, m)
}

func (n *ramrole) AttachPolicyToRole(ctx context.Context, m cloud.RamModel) (cloud.RamModel, error) {
	if m.RamName == "" {
		return m, fmt.Errorf("create with empty policy name")
	}
	req := ram.CreateAttachPolicyToRoleRequest()
	req.RoleName = m.RamName
	req.PolicyType = "Custom"
	req.PolicyName = m.PolicyName
	klog.Infof("attach policy [%s] to ram role[%s]", m.PolicyName, m.RamName)
	_, err := n.RAM.AttachPolicyToRole(req)

	return m, err
}

func (n *ramrole) ListPoliciesForRole(ctx context.Context, m cloud.RamModel) (cloud.RamModel, error) {
	if m.RamName == "" {
		return m, fmt.Errorf("list policies with empty role name")
	}
	req := ram.CreateListPoliciesForRoleRequest()
	req.RoleName = m.RamName
	r, err := n.RAM.ListPoliciesForRole(req)
	if err != nil {
		return m, err
	}
	for _, v := range r.Policies.Policy {
		if v.PolicyName == m.PolicyName {
			return m, nil
		}
	}
	return m, cloud.NotFound
}

var policy = `
{
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Effect": "Allow",
      "Principal": {
        "Service": [
          "ecs.aliyuncs.com"
        ]
      }
    }
  ],
  "Version": "1"
}
`

var policyDoc = `
{
  "Version": "1",
  "Statement": [
    {
      "Action": [
        "ecs:AttachDisk",
        "ecs:DetachDisk",
        "ecs:DescribeDisks",
        "ecs:CreateDisk",
        "ecs:CreateSnapshot",
        "ecs:DeleteDisk",
        "ecs:CreateNetworkInterface",
        "ecs:DescribeNetworkInterfaces",
        "ecs:AttachNetworkInterface",
        "ecs:AssignPrivateIpAddresses",
        "ecs:DetachNetworkInterface",
        "ecs:DeleteNetworkInterface",
        "ecs:DescribeInstanceAttribute"
      ],
      "Resource": [
        "*"
      ],
      "Effect": "Allow"
    },
    {
      "Action": [
        "cr:Get*",
        "cr:List*",
        "cr:PullRepository"
      ],
      "Resource": [
        "*"
      ],
      "Effect": "Allow"
    },
    {
      "Action": [
        "eci:CreateContainerGroup",
        "eci:DeleteContainerGroup",
        "eci:DescribeContainerGroups",
        "eci:DescribeContainerLog"
      ],
      "Resource": ["*"],
      "Effect": "Allow"
    },
    { "Action": [ "log:*" ], "Resource": [ "*" ], "Effect": "Allow" },
    { "Action": [ "cms:*" ], "Resource": [ "*" ], "Effect": "Allow" },
    { "Action": [ "vpc:*" ], "Resource": [ "*" ], "Effect": "Allow" }
  ]
}
`
