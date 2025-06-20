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

type vpc struct {
	mgr *client.ClientMgr
}

func NewVpc(mgr *client.ClientMgr) cloud.IVpc {
	return &vpc{mgr: mgr}
}

func (v *vpc) FindVPC(ctx context.Context, id cloud.Id) (cloud.VpcModel, error) {
	if id.Id != "" {
		return v.findVPCByID(ctx, id.Id)
	}
	if id.Name != "" {
		return v.findVPCByName(ctx, id.Name)
	}
	return cloud.VpcModel{}, cloud.NotFound
}

func (v *vpc) findVPCByID(ctx context.Context, vpcID string) (cloud.VpcModel, error) {
	input := &ec2.DescribeVpcsInput{
		VpcIds: []string{vpcID},
	}

	result, err := v.mgr.EC2Client.DescribeVpcs(ctx, input)
	if err != nil {
		return cloud.VpcModel{}, fmt.Errorf("failed to describe VPC: %w", err)
	}

	if len(result.Vpcs) == 0 {
		return cloud.VpcModel{}, cloud.NotFound
	}

	vpc := result.Vpcs[0]
	return v.convertToVpcModel(vpc), nil
}

func (v *vpc) findVPCByName(ctx context.Context, name string) (cloud.VpcModel, error) {
	input := &ec2.DescribeVpcsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:Name"),
				Values: []string{name},
			},
		},
	}

	result, err := v.mgr.EC2Client.DescribeVpcs(ctx, input)
	if err != nil {
		return cloud.VpcModel{}, fmt.Errorf("failed to describe VPC: %w", err)
	}

	if len(result.Vpcs) == 0 {
		return cloud.VpcModel{}, cloud.NotFound
	}

	return v.convertToVpcModel(result.Vpcs[0]), nil
}

func (v *vpc) ListVPC(ctx context.Context, id cloud.Id) ([]cloud.VpcModel, error) {
	input := &ec2.DescribeVpcsInput{}

	// Add filters if provided
	var filters []types.Filter
	if id.Region != "" {
		filters = append(filters, types.Filter{
			Name:   aws.String("region"),
			Values: []string{id.Region},
		})
	}

	if len(filters) > 0 {
		input.Filters = filters
	}

	result, err := v.mgr.EC2Client.DescribeVpcs(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list VPCs: %w", err)
	}

	var vpcs []cloud.VpcModel
	for _, vpc := range result.Vpcs {
		vpcs = append(vpcs, v.convertToVpcModel(vpc))
	}

	return vpcs, nil
}

func (v *vpc) CreateVPC(ctx context.Context, vpc cloud.VpcModel) (string, error) {
	input := &ec2.CreateVpcInput{
		CidrBlock: aws.String(vpc.Cidr),
	}

	// Add tags
	if len(vpc.Tag) > 0 {
		input.TagSpecifications = []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeVpc,
				Tags:         v.mgr.CreateTags(vpc.Tag),
			},
		}
	}

	result, err := v.mgr.EC2Client.CreateVpc(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to create VPC: %w", err)
	}

	klog.Infof("Created VPC: %s", *result.Vpc.VpcId)
	return *result.Vpc.VpcId, nil
}

func (v *vpc) UpdateVPC(ctx context.Context, vpc cloud.VpcModel) error {
	if vpc.VpcId == "" {
		return fmt.Errorf("VPC ID is required for update")
	}

	// Update VPC attributes if needed
	if vpc.VpcName != "" {
		// Create tags for VPC name
		input := &ec2.CreateTagsInput{
			Resources: []string{vpc.VpcId},
			Tags: []types.Tag{
				{
					Key:   aws.String("Name"),
					Value: aws.String(vpc.VpcName),
				},
			},
		}

		_, err := v.mgr.EC2Client.CreateTags(ctx, input)
		if err != nil {
			return fmt.Errorf("failed to update VPC name: %w", err)
		}
	}

	return nil
}

func (v *vpc) DeleteVPC(ctx context.Context, id string) error {
	input := &ec2.DeleteVpcInput{
		VpcId: aws.String(id),
	}

	_, err := v.mgr.EC2Client.DeleteVpc(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete VPC: %w", err)
	}

	klog.Infof("Deleted VPC: %s", id)
	return nil
}

func (v *vpc) convertToVpcModel(awsVpc types.Vpc) cloud.VpcModel {
	model := cloud.VpcModel{
		VpcId:  *awsVpc.VpcId,
		Cidr:   *awsVpc.CidrBlock,
		Region: v.mgr.GetRegion(),
		Tag:    v.mgr.ConvertToCloudTags(awsVpc.Tags),
	}

	// Extract VPC name from tags
	for _, tag := range awsVpc.Tags {
		if tag.Key != nil && *tag.Key == "Name" && tag.Value != nil {
			model.VpcName = *tag.Value
			break
		}
	}

	return model
}
