package ec2

import (
	"context"
	"fmt"

	"github.com/aoxn/meridian/internal/cloud"
	"github.com/aoxn/meridian/internal/cloud/aws/client"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

type instance struct {
	mgr *client.ClientMgr
}

func NewInstance(mgr *client.ClientMgr) cloud.IInstance {
	return &instance{mgr: mgr}
}

func (i *instance) GetInstanceId(node *corev1.Node) string {
	// Extract instance ID from node provider ID
	// AWS format: aws:///us-west-2a/i-1234567890abcdef0
	if node.Spec.ProviderID != "" {
		// Parse provider ID to extract instance ID
		// This is a simplified implementation
		return node.Spec.ProviderID
	}
	return ""
}

func (i *instance) FindInstance(ctx context.Context, id cloud.Id) (cloud.InstanceModel, error) {
	if id.Id != "" {
		return i.findInstanceByID(ctx, id.Id)
	}
	if id.Name != "" {
		return i.findInstanceByName(ctx, id.Name)
	}
	return cloud.InstanceModel{}, cloud.NotFound
}

func (i *instance) findInstanceByID(ctx context.Context, instanceID string) (cloud.InstanceModel, error) {
	input := &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	}

	result, err := i.mgr.EC2Client.DescribeInstances(ctx, input)
	if err != nil {
		return cloud.InstanceModel{}, fmt.Errorf("failed to describe instance: %w", err)
	}

	if len(result.Reservations) == 0 || len(result.Reservations[0].Instances) == 0 {
		return cloud.InstanceModel{}, cloud.NotFound
	}

	instance := result.Reservations[0].Instances[0]
	return i.convertToInstanceModel(instance), nil
}

func (i *instance) findInstanceByName(ctx context.Context, name string) (cloud.InstanceModel, error) {
	input := &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:Name"),
				Values: []string{name},
			},
		},
	}

	result, err := i.mgr.EC2Client.DescribeInstances(ctx, input)
	if err != nil {
		return cloud.InstanceModel{}, fmt.Errorf("failed to describe instance: %w", err)
	}

	if len(result.Reservations) == 0 || len(result.Reservations[0].Instances) == 0 {
		return cloud.InstanceModel{}, cloud.NotFound
	}

	return i.convertToInstanceModel(result.Reservations[0].Instances[0]), nil
}

func (i *instance) ListInstance(ctx context.Context, id cloud.Id) ([]cloud.InstanceModel, error) {
	input := &ec2.DescribeInstancesInput{}

	// Add filters if provided
	var filters []types.Filter
	if id.Region != "" {
		filters = append(filters, types.Filter{
			Name:   aws.String("availability-zone"),
			Values: []string{id.Region},
		})
	}

	if len(filters) > 0 {
		input.Filters = filters
	}

	result, err := i.mgr.EC2Client.DescribeInstances(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list instances: %w", err)
	}

	var instances []cloud.InstanceModel
	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			instances = append(instances, i.convertToInstanceModel(instance))
		}
	}

	return instances, nil
}

func (i *instance) CreateInstance(ctx context.Context, inst cloud.InstanceModel) (string, error) {
	// This is a simplified implementation
	// In practice, you would need to specify more parameters like AMI, instance type, etc.
	return "", fmt.Errorf("not implemented for AWS - use Auto Scaling Groups instead")
}

func (i *instance) UpdateInstance(ctx context.Context, inst cloud.InstanceModel) error {
	// AWS instances cannot be updated directly
	// You would need to stop, modify, and start the instance
	return fmt.Errorf("not implemented for AWS")
}

func (i *instance) DeleteInstance(ctx context.Context, id cloud.Id) error {
	if id.Id == "" {
		return fmt.Errorf("instance ID is required for deletion")
	}

	input := &ec2.TerminateInstancesInput{
		InstanceIds: []string{id.Id},
	}

	_, err := i.mgr.EC2Client.TerminateInstances(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to terminate instance: %w", err)
	}

	klog.Infof("Terminated instance: %s", id.Id)
	return nil
}

func (i *instance) RunCommand(ctx context.Context, id cloud.Id, command string) (string, error) {
	if id.Id == "" {
		return "", fmt.Errorf("instance ID is required for running command")
	}

	// Use AWS Systems Manager to run commands on instances
	input := &ssm.SendCommandInput{
		DocumentName: aws.String("AWS-RunShellScript"),
		InstanceIds:  []string{id.Id},
		Parameters: map[string][]string{
			"commands": {command},
		},
	}

	result, err := i.mgr.SSMClient.SendCommand(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to send command: %w", err)
	}

	// Wait for command completion and get results
	// This is a simplified implementation
	// In practice, you would poll for command completion
	klog.Infof("Sent command to instance %s: %s", id.Id, *result.Command.CommandId)
	return *result.Command.CommandId, nil
}

func (i *instance) convertToInstanceModel(awsInstance types.Instance) cloud.InstanceModel {
	model := cloud.InstanceModel{
		Tag: i.mgr.ConvertToCloudTags(awsInstance.Tags),
	}

	// Extract instance name from tags
	for _, tag := range awsInstance.Tags {
		if tag.Key != nil && *tag.Key == "Name" && tag.Value != nil {
			// Instance name is stored in tags
			break
		}
	}

	return model
}
