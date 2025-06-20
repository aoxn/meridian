package ec2

import (
	"context"
	"fmt"

	"github.com/aoxn/meridian/internal/cloud"
	"github.com/aoxn/meridian/internal/cloud/aws/client"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"k8s.io/klog/v2"
)

type autoScalingGroup struct {
	mgr *client.ClientMgr
}

func NewAutoScalingGroup(mgr *client.ClientMgr) cloud.IElasticScalingGroup {
	return &autoScalingGroup{mgr: mgr}
}

func (a *autoScalingGroup) FindESSBy(ctx context.Context, id cloud.Id) (cloud.ScalingGroupModel, error) {
	if id.Id != "" {
		return a.findASGByID(ctx, id.Id)
	}
	if id.Name != "" {
		return a.findASGByName(ctx, id.Name)
	}
	return cloud.ScalingGroupModel{}, cloud.NotFound
}

func (a *autoScalingGroup) findASGByID(ctx context.Context, asgID string) (cloud.ScalingGroupModel, error) {
	input := &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []string{asgID},
	}

	result, err := a.mgr.AutoScalingClient.DescribeAutoScalingGroups(ctx, input)
	if err != nil {
		return cloud.ScalingGroupModel{}, fmt.Errorf("failed to describe ASG: %w", err)
	}

	if len(result.AutoScalingGroups) == 0 {
		return cloud.ScalingGroupModel{}, cloud.NotFound
	}

	asg := result.AutoScalingGroups[0]
	return a.convertToScalingGroupModel(asg), nil
}

func (a *autoScalingGroup) findASGByName(ctx context.Context, name string) (cloud.ScalingGroupModel, error) {
	input := &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []string{name},
	}

	result, err := a.mgr.AutoScalingClient.DescribeAutoScalingGroups(ctx, input)
	if err != nil {
		return cloud.ScalingGroupModel{}, fmt.Errorf("failed to describe ASG: %w", err)
	}

	if len(result.AutoScalingGroups) == 0 {
		return cloud.ScalingGroupModel{}, cloud.NotFound
	}

	return a.convertToScalingGroupModel(result.AutoScalingGroups[0]), nil
}

func (a *autoScalingGroup) ListESS(ctx context.Context, id cloud.Id) ([]cloud.ScalingGroupModel, error) {
	input := &autoscaling.DescribeAutoScalingGroupsInput{}

	result, err := a.mgr.AutoScalingClient.DescribeAutoScalingGroups(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list ASGs: %w", err)
	}

	var asgs []cloud.ScalingGroupModel
	for _, asg := range result.AutoScalingGroups {
		asgs = append(asgs, a.convertToScalingGroupModel(asg))
	}

	return asgs, nil
}

func (a *autoScalingGroup) CreateESS(ctx context.Context, id string, ess cloud.ScalingGroupModel) (string, error) {
	if len(ess.VSwitchId) == 0 {
		return "", fmt.Errorf("at least one subnet is required")
	}

	// Extract subnet IDs
	var subnetIDs []string
	for _, vswitch := range ess.VSwitchId {
		subnetIDs = append(subnetIDs, vswitch.VSwitchId)
	}

	input := &autoscaling.CreateAutoScalingGroupInput{
		AutoScalingGroupName: aws.String(ess.ScalingGroupName),
		MinSize:              aws.Int32(int32(ess.Min)),
		MaxSize:              aws.Int32(int32(ess.Max)),
		DesiredCapacity:      aws.Int32(int32(ess.DesiredCapacity)),
		VPCZoneIdentifier:    aws.String(subnetIDs[0]), // AWS only supports one subnet per ASG
	}

	// Add launch template if specified
	if ess.ScalingConfig.ImageId != "" {
		input.LaunchTemplate = &types.LaunchTemplateSpecification{
			LaunchTemplateName: aws.String(ess.ScalingConfig.ScalingCfgName),
			Version:            aws.String("$Latest"),
		}
	}

	// Add tags
	if len(ess.Tag) > 0 {
		var tags []types.Tag
		for _, tag := range ess.Tag {
			tags = append(tags, types.Tag{
				Key:               aws.String(tag.Key),
				Value:             aws.String(tag.Value),
				PropagateAtLaunch: aws.Bool(true),
			})
		}
		input.Tags = tags
	}

	_, err := a.mgr.AutoScalingClient.CreateAutoScalingGroup(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to create ASG: %w", err)
	}

	klog.Infof("Created ASG: %s", ess.ScalingGroupName)
	return ess.ScalingGroupName, nil
}

func (a *autoScalingGroup) UpdateESS(ctx context.Context, ess cloud.ScalingGroupModel) error {
	if ess.ScalingGroupId == "" {
		return fmt.Errorf("ASG ID is required for update")
	}

	input := &autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: aws.String(ess.ScalingGroupId),
	}

	if ess.Min > 0 {
		input.MinSize = aws.Int32(int32(ess.Min))
	}
	if ess.Max > 0 {
		input.MaxSize = aws.Int32(int32(ess.Max))
	}
	if ess.DesiredCapacity > 0 {
		input.DesiredCapacity = aws.Int32(int32(ess.DesiredCapacity))
	}

	_, err := a.mgr.AutoScalingClient.UpdateAutoScalingGroup(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to update ASG: %w", err)
	}

	return nil
}

func (a *autoScalingGroup) DeleteESS(ctx context.Context, ess cloud.ScalingGroupModel) error {
	if ess.ScalingGroupId == "" {
		return fmt.Errorf("ASG ID is required for deletion")
	}

	input := &autoscaling.DeleteAutoScalingGroupInput{
		AutoScalingGroupName: aws.String(ess.ScalingGroupId),
		ForceDelete:          aws.Bool(true),
	}

	_, err := a.mgr.AutoScalingClient.DeleteAutoScalingGroup(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete ASG: %w", err)
	}

	klog.Infof("Deleted ASG: %s", ess.ScalingGroupId)
	return nil
}

func (a *autoScalingGroup) ScaleNodeGroup(ctx context.Context, model cloud.ScalingGroupModel, desired uint) error {
	if model.ScalingGroupId == "" {
		return fmt.Errorf("ASG ID is required for scaling")
	}

	input := &autoscaling.SetDesiredCapacityInput{
		AutoScalingGroupName: aws.String(model.ScalingGroupId),
		DesiredCapacity:      aws.Int32(int32(desired)),
	}

	_, err := a.mgr.AutoScalingClient.SetDesiredCapacity(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to scale ASG: %w", err)
	}

	klog.Infof("Scaled ASG %s to %d instances", model.ScalingGroupId, desired)
	return nil
}

func (a *autoScalingGroup) FindScalingConfig(ctx context.Context, id cloud.Id) (cloud.ScalingConfig, error) {
	// AWS uses Launch Templates instead of scaling configurations
	// This would need to be implemented using EC2 Launch Templates
	return cloud.ScalingConfig{}, fmt.Errorf("not implemented for AWS")
}

func (a *autoScalingGroup) FindScalingRule(ctx context.Context, id cloud.Id) (cloud.ScalingRule, error) {
	// AWS uses CloudWatch alarms and scaling policies
	// This would need to be implemented using Auto Scaling policies
	return cloud.ScalingRule{}, fmt.Errorf("not implemented for AWS")
}

func (a *autoScalingGroup) CreateScalingConfig(ctx context.Context, id string, cfg cloud.ScalingConfig) (string, error) {
	// AWS uses Launch Templates instead of scaling configurations
	// This would need to be implemented using EC2 Launch Templates
	return "", fmt.Errorf("not implemented for AWS")
}

func (a *autoScalingGroup) CreateScalingRule(ctx context.Context, id string, rule cloud.ScalingRule) (cloud.ScalingRule, error) {
	// AWS uses CloudWatch alarms and scaling policies
	// This would need to be implemented using Auto Scaling policies
	return cloud.ScalingRule{}, fmt.Errorf("not implemented for AWS")
}

func (a *autoScalingGroup) ExecuteScalingRule(ctx context.Context, id string) (string, error) {
	// AWS uses CloudWatch alarms and scaling policies
	// This would need to be implemented using Auto Scaling policies
	return "", fmt.Errorf("not implemented for AWS")
}

func (a *autoScalingGroup) DeleteScalingConfig(ctx context.Context, cfgId string) error {
	// AWS uses Launch Templates instead of scaling configurations
	return fmt.Errorf("not implemented for AWS")
}

func (a *autoScalingGroup) DeleteScalingRule(ctx context.Context, ruleId string) error {
	// AWS uses CloudWatch alarms and scaling policies
	return fmt.Errorf("not implemented for AWS")
}

func (a *autoScalingGroup) EnableScalingGroup(ctx context.Context, gid, sid string) error {
	// AWS ASGs are enabled by default when created
	return nil
}

func (a *autoScalingGroup) convertToScalingGroupModel(asg types.AutoScalingGroup) cloud.ScalingGroupModel {
	model := cloud.ScalingGroupModel{
		ScalingGroupId:   *asg.AutoScalingGroupName,
		ScalingGroupName: *asg.AutoScalingGroupName,
		Min:              int(*asg.MinSize),
		Max:              int(*asg.MaxSize),
		DesiredCapacity:  int(*asg.DesiredCapacity),
		Region:           a.mgr.GetRegion(),
	}

	// Convert tags
	for _, tag := range asg.Tags {
		if tag.Key != nil && tag.Value != nil {
			model.Tag = append(model.Tag, cloud.Tag{
				Key:   *tag.Key,
				Value: *tag.Value,
			})
		}
	}

	// Convert VPC zone identifier to VSwitch models
	if asg.VPCZoneIdentifier != nil {
		// AWS VPCZoneIdentifier is a comma-separated list of subnet IDs
		// For simplicity, we'll create a single VSwitch model
		model.VSwitchId = []cloud.VSwitchModel{
			{
				VSwitchId: *asg.VPCZoneIdentifier,
			},
		}
	}

	return model
}
