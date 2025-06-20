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

type eip struct {
	mgr *client.ClientMgr
}

func NewEIP(mgr *client.ClientMgr) cloud.IEip {
	return &eip{mgr: mgr}
}

func (e *eip) FindEIP(ctx context.Context, id cloud.Id) (cloud.EipModel, error) {
	if id.Id != "" {
		return e.findEIPByID(ctx, id.Id)
	}
	if id.Name != "" {
		return e.findEIPByName(ctx, id.Name)
	}
	return cloud.EipModel{}, cloud.NotFound
}

func (e *eip) findEIPByID(ctx context.Context, allocationID string) (cloud.EipModel, error) {
	input := &ec2.DescribeAddressesInput{
		AllocationIds: []string{allocationID},
	}

	result, err := e.mgr.EC2Client.DescribeAddresses(ctx, input)
	if err != nil {
		return cloud.EipModel{}, fmt.Errorf("failed to describe EIP: %w", err)
	}

	if len(result.Addresses) == 0 {
		return cloud.EipModel{}, cloud.NotFound
	}

	address := result.Addresses[0]
	return e.convertToEipModel(address), nil
}

func (e *eip) findEIPByName(ctx context.Context, name string) (cloud.EipModel, error) {
	input := &ec2.DescribeAddressesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:Name"),
				Values: []string{name},
			},
		},
	}

	result, err := e.mgr.EC2Client.DescribeAddresses(ctx, input)
	if err != nil {
		return cloud.EipModel{}, fmt.Errorf("failed to describe EIP: %w", err)
	}

	if len(result.Addresses) == 0 {
		return cloud.EipModel{}, cloud.NotFound
	}

	return e.convertToEipModel(result.Addresses[0]), nil
}

func (e *eip) ListEIP(ctx context.Context, id cloud.Id) ([]cloud.EipModel, error) {
	input := &ec2.DescribeAddressesInput{}

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

	result, err := e.mgr.EC2Client.DescribeAddresses(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list EIPs: %w", err)
	}

	var eips []cloud.EipModel
	for _, address := range result.Addresses {
		eips = append(eips, e.convertToEipModel(address))
	}

	return eips, nil
}

func (e *eip) CreateEIP(ctx context.Context, m cloud.EipModel) (string, error) {
	input := &ec2.AllocateAddressInput{
		Domain: types.DomainTypeVpc, // Default to VPC domain
	}

	// Add tags if provided
	if len(m.Tag) > 0 {
		input.TagSpecifications = []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeElasticIp,
				Tags:         e.mgr.CreateTags(m.Tag),
			},
		}
	}

	result, err := e.mgr.EC2Client.AllocateAddress(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to allocate EIP: %w", err)
	}

	klog.Infof("Created EIP: %s", *result.AllocationId)
	return *result.AllocationId, nil
}

func (e *eip) UpdateEIP(ctx context.Context, m cloud.EipModel) error {
	if m.EipId == "" {
		return fmt.Errorf("EIP ID is required for update")
	}

	// Update EIP attributes if needed
	if m.EipName != "" {
		// Create tags for EIP name
		input := &ec2.CreateTagsInput{
			Resources: []string{m.EipId},
			Tags: []types.Tag{
				{
					Key:   aws.String("Name"),
					Value: aws.String(m.EipName),
				},
			},
		}

		_, err := e.mgr.EC2Client.CreateTags(ctx, input)
		if err != nil {
			return fmt.Errorf("failed to update EIP name: %w", err)
		}
	}

	return nil
}

func (e *eip) DeleteEIP(ctx context.Context, id cloud.Id) error {
	if id.Id == "" {
		return fmt.Errorf("EIP ID is required for deletion")
	}

	input := &ec2.ReleaseAddressInput{
		AllocationId: aws.String(id.Id),
	}

	_, err := e.mgr.EC2Client.ReleaseAddress(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to release EIP: %w", err)
	}

	klog.Infof("Released EIP: %s", id.Id)
	return nil
}

func (e *eip) BindEIP(ctx context.Context, do cloud.EipModel) error {
	if do.EipId == "" {
		return fmt.Errorf("EIP ID is required for binding")
	}

	if do.InstanceId == "" {
		return fmt.Errorf("instance ID is required for binding")
	}

	input := &ec2.AssociateAddressInput{
		AllocationId: aws.String(do.EipId),
		InstanceId:   aws.String(do.InstanceId),
	}

	_, err := e.mgr.EC2Client.AssociateAddress(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to associate EIP: %w", err)
	}

	klog.Infof("Associated EIP %s with instance %s", do.EipId, do.InstanceId)
	return nil
}

func (e *eip) convertToEipModel(address types.Address) cloud.EipModel {
	model := cloud.EipModel{
		EipId:   *address.AllocationId,
		Address: *address.PublicIp,
		Region:  e.mgr.GetRegion(),
		Tag:     e.mgr.ConvertToCloudTags(address.Tags),
	}

	// Extract EIP name from tags
	for _, tag := range address.Tags {
		if tag.Key != nil && *tag.Key == "Name" && tag.Value != nil {
			model.EipName = *tag.Value
			break
		}
	}

	// Set instance information if associated
	if address.InstanceId != nil {
		model.InstanceId = *address.InstanceId
		model.InstanceType = "EC2Instance"
	}

	return model
}
