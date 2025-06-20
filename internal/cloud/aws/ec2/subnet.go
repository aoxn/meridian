package ec2

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

type subnet struct {
	mgr *client.ClientMgr
}

func NewSubnet(mgr *client.ClientMgr) cloud.IVSwitch {
	return &subnet{mgr: mgr}
}

func (s *subnet) FindVSwitch(ctx context.Context, vpcid string, id cloud.Id) (cloud.VSwitchModel, error) {
	if id.Id != "" {
		return s.findSubnetByID(ctx, id.Id)
	}
	if id.Name != "" {
		return s.findSubnetByName(ctx, vpcid, id.Name)
	}
	return cloud.VSwitchModel{}, cloud.NotFound
}

func (s *subnet) findSubnetByID(ctx context.Context, subnetID string) (cloud.VSwitchModel, error) {
	input := &ec2.DescribeSubnetsInput{
		SubnetIds: []string{subnetID},
	}

	result, err := s.mgr.EC2Client.DescribeSubnets(ctx, input)
	if err != nil {
		return cloud.VSwitchModel{}, fmt.Errorf("failed to describe subnet: %w", err)
	}

	if len(result.Subnets) == 0 {
		return cloud.VSwitchModel{}, cloud.NotFound
	}

	subnet := result.Subnets[0]
	return s.convertToVSwitchModel(subnet), nil
}

func (s *subnet) findSubnetByName(ctx context.Context, vpcID, name string) (cloud.VSwitchModel, error) {
	input := &ec2.DescribeSubnetsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{vpcID},
			},
			{
				Name:   aws.String("tag:Name"),
				Values: []string{name},
			},
		},
	}

	result, err := s.mgr.EC2Client.DescribeSubnets(ctx, input)
	if err != nil {
		return cloud.VSwitchModel{}, fmt.Errorf("failed to describe subnet: %w", err)
	}

	if len(result.Subnets) == 0 {
		return cloud.VSwitchModel{}, cloud.NotFound
	}

	return s.convertToVSwitchModel(result.Subnets[0]), nil
}

func (s *subnet) ListVSwitch(ctx context.Context, vpcid string, id cloud.Id) ([]cloud.VSwitchModel, error) {
	input := &ec2.DescribeSubnetsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{vpcid},
			},
		},
	}

	// Add additional filters if provided
	if id.Region != "" {
		input.Filters = append(input.Filters, types.Filter{
			Name:   aws.String("availability-zone"),
			Values: []string{id.Region},
		})
	}

	result, err := s.mgr.EC2Client.DescribeSubnets(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list subnets: %w", err)
	}

	var subnets []cloud.VSwitchModel
	for _, subnet := range result.Subnets {
		subnets = append(subnets, s.convertToVSwitchModel(subnet))
	}

	return subnets, nil
}

func (s *subnet) CreateVSwitch(ctx context.Context, vpcid string, model cloud.VSwitchModel) (string, error) {
	if model.ZoneId == "" {
		return "", fmt.Errorf("availability zone is required")
	}

	input := &ec2.CreateSubnetInput{
		VpcId:            aws.String(vpcid),
		CidrBlock:        aws.String(model.CidrBlock),
		AvailabilityZone: aws.String(model.ZoneId),
	}

	// Add tags
	if len(model.Tag) > 0 {
		input.TagSpecifications = []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeSubnet,
				Tags:         s.mgr.CreateTags(model.Tag),
			},
		}
	}

	result, err := s.mgr.EC2Client.CreateSubnet(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to create subnet: %w", err)
	}

	klog.Infof("Created subnet: %s", *result.Subnet.SubnetId)
	return *result.Subnet.SubnetId, nil
}

func (s *subnet) UpdateVSwitch(ctx context.Context, vpcid string, model cloud.VSwitchModel) error {
	if model.VSwitchId == "" {
		return fmt.Errorf("subnet ID is required for update")
	}

	// Update subnet attributes if needed
	if model.VSwitchName != "" {
		// Create tags for subnet name
		input := &ec2.CreateTagsInput{
			Resources: []string{model.VSwitchId},
			Tags: []types.Tag{
				{
					Key:   aws.String("Name"),
					Value: aws.String(model.VSwitchName),
				},
			},
		}

		_, err := s.mgr.EC2Client.CreateTags(ctx, input)
		if err != nil {
			return fmt.Errorf("failed to update subnet name: %w", err)
		}
	}

	return nil
}

func (s *subnet) DeleteVSwitch(ctx context.Context, vpcId string, id cloud.Id) error {
	if id.Id == "" {
		return fmt.Errorf("subnet ID is required for deletion")
	}

	input := &ec2.DeleteSubnetInput{
		SubnetId: aws.String(id.Id),
	}

	_, err := s.mgr.EC2Client.DeleteSubnet(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete subnet: %w", err)
	}

	klog.Infof("Deleted subnet: %s", id.Id)
	return nil
}

func (s *subnet) convertToVSwitchModel(awsSubnet types.Subnet) cloud.VSwitchModel {
	model := cloud.VSwitchModel{
		VSwitchId: *awsSubnet.SubnetId,
		CidrBlock: *awsSubnet.CidrBlock,
		ZoneId:    *awsSubnet.AvailabilityZone,
		Tag:       s.mgr.ConvertToCloudTags(awsSubnet.Tags),
	}

	// Extract subnet name from tags
	for _, tag := range awsSubnet.Tags {
		if tag.Key != nil && *tag.Key == "Name" && tag.Value != nil {
			model.VSwitchName = *tag.Value
			break
		}
	}

	return model
}
