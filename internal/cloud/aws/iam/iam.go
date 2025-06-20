package iam

import (
	"context"
	"fmt"

	"github.com/aoxn/meridian/internal/cloud"
	"github.com/aoxn/meridian/internal/cloud/aws/client"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
	"k8s.io/klog/v2"
)

type iamRole struct {
	mgr *client.ClientMgr
}

func NewIAMRole(mgr *client.ClientMgr) cloud.IRamRole {
	return &iamRole{mgr: mgr}
}

func (i *iamRole) FindRAM(ctx context.Context, id cloud.Id) (cloud.RamModel, error) {
	if id.Id != "" {
		return i.findRoleByID(ctx, id.Id)
	}
	if id.Name != "" {
		return i.findRoleByName(ctx, id.Name)
	}
	return cloud.RamModel{}, cloud.NotFound
}

func (i *iamRole) findRoleByID(ctx context.Context, roleID string) (cloud.RamModel, error) {
	// AWS IAM roles don't have IDs, they have ARNs
	// This assumes the ID is actually the role name
	return i.findRoleByName(ctx, roleID)
}

func (i *iamRole) findRoleByName(ctx context.Context, name string) (cloud.RamModel, error) {
	input := &iam.GetRoleInput{
		RoleName: aws.String(name),
	}

	result, err := i.mgr.IAMClient.GetRole(ctx, input)
	if err != nil {
		return cloud.RamModel{}, fmt.Errorf("failed to get IAM role: %w", err)
	}

	return i.convertToRamModel(*result.Role), nil
}

func (i *iamRole) ListRAM(ctx context.Context, id cloud.Id) ([]cloud.RamModel, error) {
	input := &iam.ListRolesInput{}

	result, err := i.mgr.IAMClient.ListRoles(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list IAM roles: %w", err)
	}

	var roles []cloud.RamModel
	for _, role := range result.Roles {
		roles = append(roles, i.convertToRamModel(role))
	}

	return roles, nil
}

func (i *iamRole) CreateRAM(ctx context.Context, m cloud.RamModel) (string, error) {
	if m.RamName == "" {
		return "", fmt.Errorf("role name is required")
	}

	// Default assume role policy document for EC2 instances
	assumeRolePolicy := `{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Effect": "Allow",
				"Principal": {
					"Service": "ec2.amazonaws.com"
				},
				"Action": "sts:AssumeRole"
			}
		]
	}`

	input := &iam.CreateRoleInput{
		RoleName:                 aws.String(m.RamName),
		AssumeRolePolicyDocument: aws.String(assumeRolePolicy),
		Description:              aws.String("IAM role created by Meridian"),
	}

	result, err := i.mgr.IAMClient.CreateRole(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to create IAM role: %w", err)
	}

	klog.Infof("Created IAM role: %s", *result.Role.RoleName)
	return *result.Role.RoleName, nil
}

func (i *iamRole) UpdateRAM(ctx context.Context, m cloud.RamModel) error {
	if m.RamId == "" {
		return fmt.Errorf("role name is required for update")
	}

	// IAM roles can be updated by modifying their policies
	// This is a simplified implementation
	return fmt.Errorf("not implemented for AWS")
}

func (i *iamRole) DeleteRAM(ctx context.Context, id cloud.Id, policyName string) error {
	if id.Id == "" {
		return fmt.Errorf("role name is required for deletion")
	}

	// First detach all policies
	if policyName != "" {
		input := &iam.DetachRolePolicyInput{
			RoleName:  aws.String(id.Id),
			PolicyArn: aws.String(policyName),
		}

		_, err := i.mgr.IAMClient.DetachRolePolicy(ctx, input)
		if err != nil {
			return fmt.Errorf("failed to detach policy: %w", err)
		}
	}

	// Then delete the role
	deleteInput := &iam.DeleteRoleInput{
		RoleName: aws.String(id.Id),
	}

	_, err := i.mgr.IAMClient.DeleteRole(ctx, deleteInput)
	if err != nil {
		return fmt.Errorf("failed to delete IAM role: %w", err)
	}

	klog.Infof("Deleted IAM role: %s", id.Id)
	return nil
}

func (i *iamRole) FindPolicy(ctx context.Context, m cloud.Id) (cloud.RamModel, error) {
	if m.Id == "" {
		return cloud.RamModel{}, fmt.Errorf("policy ARN is required")
	}

	input := &iam.GetPolicyInput{
		PolicyArn: aws.String(m.Id),
	}

	result, err := i.mgr.IAMClient.GetPolicy(ctx, input)
	if err != nil {
		return cloud.RamModel{}, fmt.Errorf("failed to get IAM policy: %w", err)
	}

	return cloud.RamModel{
		RamId:   *result.Policy.PolicyName,
		RamName: *result.Policy.PolicyName,
		Arn:     *result.Policy.Arn,
	}, nil
}

func (i *iamRole) CreatePolicy(ctx context.Context, m cloud.RamModel) (cloud.RamModel, error) {
	if m.RamName == "" {
		return cloud.RamModel{}, fmt.Errorf("policy name is required")
	}

	// Default policy document for basic EC2 permissions
	policyDocument := `{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Effect": "Allow",
				"Action": [
					"ec2:DescribeInstances",
					"ec2:DescribeRegions",
					"ec2:DescribeAvailabilityZones"
				],
				"Resource": "*"
			}
		]
	}`

	input := &iam.CreatePolicyInput{
		PolicyName:     aws.String(m.RamName),
		PolicyDocument: aws.String(policyDocument),
		Description:    aws.String("IAM policy created by Meridian"),
	}

	result, err := i.mgr.IAMClient.CreatePolicy(ctx, input)
	if err != nil {
		return cloud.RamModel{}, fmt.Errorf("failed to create IAM policy: %w", err)
	}

	klog.Infof("Created IAM policy: %s", *result.Policy.PolicyName)
	return cloud.RamModel{
		RamId:   *result.Policy.PolicyName,
		RamName: *result.Policy.PolicyName,
		Arn:     *result.Policy.Arn,
	}, nil
}

func (i *iamRole) AttachPolicyToRole(ctx context.Context, m cloud.RamModel) (cloud.RamModel, error) {
	if m.RamId == "" {
		return cloud.RamModel{}, fmt.Errorf("role name is required")
	}
	if m.Arn == "" {
		return cloud.RamModel{}, fmt.Errorf("policy ARN is required")
	}

	input := &iam.AttachRolePolicyInput{
		RoleName:  aws.String(m.RamId),
		PolicyArn: aws.String(m.Arn),
	}

	_, err := i.mgr.IAMClient.AttachRolePolicy(ctx, input)
	if err != nil {
		return cloud.RamModel{}, fmt.Errorf("failed to attach policy to role: %w", err)
	}

	klog.Infof("Attached policy %s to role %s", m.Arn, m.RamId)
	return m, nil
}

func (i *iamRole) ListPoliciesForRole(ctx context.Context, m cloud.RamModel) (cloud.RamModel, error) {
	if m.RamId == "" {
		return cloud.RamModel{}, fmt.Errorf("role name is required")
	}

	input := &iam.ListAttachedRolePoliciesInput{
		RoleName: aws.String(m.RamId),
	}

	result, err := i.mgr.IAMClient.ListAttachedRolePolicies(ctx, input)
	if err != nil {
		return cloud.RamModel{}, fmt.Errorf("failed to list policies for role: %w", err)
	}

	// Return the first policy found (simplified)
	if len(result.AttachedPolicies) > 0 {
		policy := result.AttachedPolicies[0]
		return cloud.RamModel{
			RamId:      m.RamId,
			RamName:    m.RamName,
			Arn:        *policy.PolicyArn,
			PolicyName: *policy.PolicyName,
		}, nil
	}

	return m, nil
}

func (i *iamRole) convertToRamModel(role types.Role) cloud.RamModel {
	return cloud.RamModel{
		RamId:   *role.RoleName,
		RamName: *role.RoleName,
		Arn:     *role.Arn,
	}
}
