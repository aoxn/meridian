package elb

import (
	"context"
	"fmt"

	"github.com/aoxn/meridian/internal/cloud"
	"github.com/aoxn/meridian/internal/cloud/aws/client"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"k8s.io/klog/v2"
)

type elb struct {
	mgr *client.ClientMgr
}

func NewELB(mgr *client.ClientMgr) cloud.ISlb {
	return &elb{mgr: mgr}
}

func (e *elb) FindSLB(ctx context.Context, id cloud.Id) (cloud.SlbModel, error) {
	if id.Id != "" {
		return e.findLoadBalancerByID(ctx, id.Id)
	}
	if id.Name != "" {
		return e.findLoadBalancerByName(ctx, id.Name)
	}
	return cloud.SlbModel{}, cloud.NotFound
}

func (e *elb) findLoadBalancerByID(ctx context.Context, lbID string) (cloud.SlbModel, error) {
	input := &elasticloadbalancingv2.DescribeLoadBalancersInput{
		LoadBalancerArns: []string{lbID},
	}

	result, err := e.mgr.ELBClient.DescribeLoadBalancers(ctx, input)
	if err != nil {
		return cloud.SlbModel{}, fmt.Errorf("failed to describe load balancer: %w", err)
	}

	if len(result.LoadBalancers) == 0 {
		return cloud.SlbModel{}, cloud.NotFound
	}

	lb := result.LoadBalancers[0]
	return e.convertToSlbModel(lb), nil
}

func (e *elb) findLoadBalancerByName(ctx context.Context, name string) (cloud.SlbModel, error) {
	input := &elasticloadbalancingv2.DescribeLoadBalancersInput{
		Names: []string{name},
	}

	result, err := e.mgr.ELBClient.DescribeLoadBalancers(ctx, input)
	if err != nil {
		return cloud.SlbModel{}, fmt.Errorf("failed to describe load balancer: %w", err)
	}

	if len(result.LoadBalancers) == 0 {
		return cloud.SlbModel{}, cloud.NotFound
	}

	return e.convertToSlbModel(result.LoadBalancers[0]), nil
}

func (e *elb) ListSLB(ctx context.Context, id cloud.Id) ([]cloud.SlbModel, error) {
	input := &elasticloadbalancingv2.DescribeLoadBalancersInput{}

	result, err := e.mgr.ELBClient.DescribeLoadBalancers(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list load balancers: %w", err)
	}

	var lbs []cloud.SlbModel
	for _, lb := range result.LoadBalancers {
		lbs = append(lbs, e.convertToSlbModel(lb))
	}

	return lbs, nil
}

func (e *elb) CreateSLB(ctx context.Context, b cloud.SlbModel) (string, error) {
	// This is a simplified implementation
	// In practice, you would need to specify more parameters like subnets, security groups, etc.
	return "", fmt.Errorf("not implemented for AWS - use Application Load Balancer or Network Load Balancer")
}

func (e *elb) UpdateSLB(ctx context.Context, b cloud.SlbModel) error {
	// AWS load balancers cannot be updated directly
	// You would need to create a new one and migrate traffic
	return fmt.Errorf("not implemented for AWS")
}

func (e *elb) DeleteSLB(ctx context.Context, id cloud.Id) error {
	if id.Id == "" {
		return fmt.Errorf("load balancer ID is required for deletion")
	}

	input := &elasticloadbalancingv2.DeleteLoadBalancerInput{
		LoadBalancerArn: aws.String(id.Id),
	}

	_, err := e.mgr.ELBClient.DeleteLoadBalancer(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete load balancer: %w", err)
	}

	klog.Infof("Deleted load balancer: %s", id.Id)
	return nil
}

func (e *elb) FindListener(ctx context.Context, id cloud.Id) (cloud.SlbModel, error) {
	if id.Id == "" {
		return cloud.SlbModel{}, fmt.Errorf("listener ID is required")
	}

	input := &elasticloadbalancingv2.DescribeListenersInput{
		ListenerArns: []string{id.Id},
	}

	result, err := e.mgr.ELBClient.DescribeListeners(ctx, input)
	if err != nil {
		return cloud.SlbModel{}, fmt.Errorf("failed to describe listener: %w", err)
	}

	if len(result.Listeners) == 0 {
		return cloud.SlbModel{}, cloud.NotFound
	}

	listener := result.Listeners[0]
	return e.convertListenerToSlbModel(listener), nil
}

func (e *elb) CreateListener(ctx context.Context, b cloud.SlbModel) (string, error) {
	// This is a simplified implementation
	// In practice, you would need to specify more parameters like load balancer ARN, target group ARN, etc.
	return "", fmt.Errorf("not implemented for AWS")
}

func (e *elb) UpdateListener(ctx context.Context, b cloud.SlbModel) error {
	// AWS listeners can be updated but this is a simplified implementation
	return fmt.Errorf("not implemented for AWS")
}

func (e *elb) DeleteListener(ctx context.Context, id cloud.Id) error {
	if id.Id == "" {
		return fmt.Errorf("listener ID is required for deletion")
	}

	input := &elasticloadbalancingv2.DeleteListenerInput{
		ListenerArn: aws.String(id.Id),
	}

	_, err := e.mgr.ELBClient.DeleteListener(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete listener: %w", err)
	}

	klog.Infof("Deleted listener: %s", id.Id)
	return nil
}

func (e *elb) convertToSlbModel(lb types.LoadBalancer) cloud.SlbModel {
	model := cloud.SlbModel{
		Tag: []cloud.Tag{
			{
				Key:   "LoadBalancerArn",
				Value: *lb.LoadBalancerArn,
			},
			{
				Key:   "DNSName",
				Value: *lb.DNSName,
			},
		},
	}

	// Extract VSwitch ID from availability zones
	if len(lb.AvailabilityZones) > 0 {
		model.VSwitchId = *lb.AvailabilityZones[0].SubnetId
	}

	return model
}

func (e *elb) convertListenerToSlbModel(listener types.Listener) cloud.SlbModel {
	model := cloud.SlbModel{
		Tag: []cloud.Tag{
			{
				Key:   "ListenerArn",
				Value: *listener.ListenerArn,
			},
			{
				Key:   "Port",
				Value: fmt.Sprintf("%d", *listener.Port),
			},
			{
				Key:   "Protocol",
				Value: string(listener.Protocol),
			},
		},
	}

	return model
}
