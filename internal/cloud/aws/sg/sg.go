package sg

import (
	"context"
	"fmt"

	"github.com/aoxn/meridian/internal/cloud"
	"github.com/aoxn/meridian/internal/cloud/aws/client"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"k8s.io/klog/v2"
)

type securityGroup struct {
	mgr *client.ClientMgr
}

func NewSecurityGroup(mgr *client.ClientMgr) cloud.ISecurityGroup {
	return &securityGroup{mgr: mgr}
}

func (s *securityGroup) FindSecurityGroup(ctx context.Context, vpcid string, id cloud.Id) (cloud.SecurityGroupModel, error) {
	if id.Id != "" {
		return s.findSecurityGroupByID(ctx, id.Id)
	}
	if id.Name != "" {
		return s.findSecurityGroupByName(ctx, vpcid, id.Name)
	}
	return cloud.SecurityGroupModel{}, cloud.NotFound
}

func (s *securityGroup) findSecurityGroupByID(ctx context.Context, sgID string) (cloud.SecurityGroupModel, error) {
	input := &ec2.DescribeSecurityGroupsInput{
		GroupIds: []string{sgID},
	}

	result, err := s.mgr.EC2Client.DescribeSecurityGroups(ctx, input)
	if err != nil {
		return cloud.SecurityGroupModel{}, fmt.Errorf("failed to describe security group: %w", err)
	}

	if len(result.SecurityGroups) == 0 {
		return cloud.SecurityGroupModel{}, cloud.NotFound
	}

	sg := result.SecurityGroups[0]
	return s.convertToSecurityGroupModel(sg), nil
}

func (s *securityGroup) findSecurityGroupByName(ctx context.Context, vpcID, name string) (cloud.SecurityGroupModel, error) {
	input := &ec2.DescribeSecurityGroupsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{vpcID},
			},
			{
				Name:   aws.String("group-name"),
				Values: []string{name},
			},
		},
	}

	result, err := s.mgr.EC2Client.DescribeSecurityGroups(ctx, input)
	if err != nil {
		return cloud.SecurityGroupModel{}, fmt.Errorf("failed to describe security group: %w", err)
	}

	if len(result.SecurityGroups) == 0 {
		return cloud.SecurityGroupModel{}, cloud.NotFound
	}

	return s.convertToSecurityGroupModel(result.SecurityGroups[0]), nil
}

func (s *securityGroup) ListSecurityGroup(ctx context.Context, vpcid string, id cloud.Id) ([]cloud.SecurityGroupModel, error) {
	input := &ec2.DescribeSecurityGroupsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{vpcid},
			},
		},
	}

	result, err := s.mgr.EC2Client.DescribeSecurityGroups(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list security groups: %w", err)
	}

	var sgs []cloud.SecurityGroupModel
	for _, sg := range result.SecurityGroups {
		sgs = append(sgs, s.convertToSecurityGroupModel(sg))
	}

	return sgs, nil
}

func (s *securityGroup) CreateSecurityGroup(ctx context.Context, vpcid string, grp cloud.SecurityGroupModel) (string, error) {
	if grp.SecurityGroupName == "" {
		return "", fmt.Errorf("security group name is required")
	}

	input := &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(grp.SecurityGroupName),
		Description: aws.String("Security group created by Meridian"),
		VpcId:       aws.String(vpcid),
	}

	// Add tags
	if len(grp.Tag) > 0 {
		input.TagSpecifications = []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeSecurityGroup,
				Tags:         s.mgr.CreateTags(grp.Tag),
			},
		}
	}

	result, err := s.mgr.EC2Client.CreateSecurityGroup(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to create security group: %w", err)
	}

	klog.Infof("Created security group: %s", *result.GroupId)
	return *result.GroupId, nil
}

func (s *securityGroup) UpdateSecurityGroup(ctx context.Context, grp cloud.SecurityGroupModel) error {
	if grp.SecurityGroupId == "" {
		return fmt.Errorf("security group ID is required for update")
	}

	// Update security group attributes if needed
	if grp.SecurityGroupName != "" {
		// Create tags for security group name
		input := &ec2.CreateTagsInput{
			Resources: []string{grp.SecurityGroupId},
			Tags: []types.Tag{
				{
					Key:   aws.String("Name"),
					Value: aws.String(grp.SecurityGroupName),
				},
			},
		}

		_, err := s.mgr.EC2Client.CreateTags(ctx, input)
		if err != nil {
			return fmt.Errorf("failed to update security group name: %w", err)
		}
	}

	return nil
}

func (s *securityGroup) DeleteSecurityGroup(ctx context.Context, id cloud.Id) error {
	if id.Id == "" {
		return fmt.Errorf("security group ID is required for deletion")
	}

	input := &ec2.DeleteSecurityGroupInput{
		GroupId: aws.String(id.Id),
	}

	_, err := s.mgr.EC2Client.DeleteSecurityGroup(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete security group: %w", err)
	}

	klog.Infof("Deleted security group: %s", id.Id)
	return nil
}

func (s *securityGroup) convertToSecurityGroupModel(awsSG types.SecurityGroup) cloud.SecurityGroupModel {
	model := cloud.SecurityGroupModel{
		SecurityGroupId:   *awsSG.GroupId,
		SecurityGroupName: *awsSG.GroupName,
		Region:            s.mgr.GetRegion(),
		Tag:               s.mgr.ConvertToCloudTags(awsSG.Tags),
	}

	return model
}
