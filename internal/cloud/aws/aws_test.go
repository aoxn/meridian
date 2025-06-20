package aws

import (
	"testing"

	api "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/cloud"
	"github.com/aoxn/meridian/internal/cloud/aws/client"
)

func TestNewAWSProvider(t *testing.T) {
	// Test with access keys
	cfg := cloud.Config{
		AuthInfo: api.AuthInfo{
			Region:       "us-west-2",
			AccessKey:    "test-access-key",
			AccessSecret: "test-secret-key",
		},
	}

	provider, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create AWS provider: %v", err)
	}

	if provider == nil {
		t.Fatal("Provider should not be nil")
	}

	// Test GetConfig
	config := provider.GetConfig()
	if config.AuthInfo.Region != "us-west-2" {
		t.Errorf("Expected region us-west-2, got %s", config.AuthInfo.Region)
	}
}

func TestAWSProviderInterfaces(t *testing.T) {
	cfg := cloud.Config{
		AuthInfo: api.AuthInfo{
			Region:       "us-west-2",
			AccessKey:    "test-access-key",
			AccessSecret: "test-secret-key",
		},
	}

	provider, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create AWS provider: %v", err)
	}

	// Test that provider implements all required interfaces
	var _ cloud.Cloud = provider
	var _ cloud.IVpc = provider
	var _ cloud.IVSwitch = provider
	var _ cloud.ISecurityGroup = provider
	var _ cloud.IElasticScalingGroup = provider
	var _ cloud.IInstance = provider
	var _ cloud.IEip = provider
	var _ cloud.IRamRole = provider
	var _ cloud.ISlb = provider
	var _ cloud.IObjectStorage = provider
}

func TestClientManager(t *testing.T) {
	auth := api.AuthInfo{
		Region:       "us-west-2",
		AccessKey:    "test-access-key",
		AccessSecret: "test-secret-key",
	}

	mgr, err := client.NewClientMgr(auth)
	if err != nil {
		t.Fatalf("Failed to create client manager: %v", err)
	}

	if mgr == nil {
		t.Fatal("Client manager should not be nil")
	}

	if mgr.GetRegion() != "us-west-2" {
		t.Errorf("Expected region us-west-2, got %s", mgr.GetRegion())
	}

	if mgr.GetAccessKey() != "test-access-key" {
		t.Errorf("Expected access key test-access-key, got %s", mgr.GetAccessKey())
	}
}

func TestTagConversion(t *testing.T) {
	auth := api.AuthInfo{
		Region:       "us-west-2",
		AccessKey:    "test-access-key",
		AccessSecret: "test-secret-key",
	}

	mgr, err := client.NewClientMgr(auth)
	if err != nil {
		t.Fatalf("Failed to create client manager: %v", err)
	}

	// Test cloud tags to AWS tags conversion
	cloudTags := []cloud.Tag{
		{Key: "Name", Value: "test-resource"},
		{Key: "Environment", Value: "test"},
	}

	awsTags := mgr.CreateTags(cloudTags)
	if len(awsTags) != 2 {
		t.Errorf("Expected 2 AWS tags, got %d", len(awsTags))
	}

	// Test AWS tags to cloud tags conversion
	convertedTags := mgr.ConvertToCloudTags(awsTags)
	if len(convertedTags) != 2 {
		t.Errorf("Expected 2 cloud tags, got %d", len(convertedTags))
	}

	// Verify tag values
	for i, tag := range convertedTags {
		if tag.Key != cloudTags[i].Key {
			t.Errorf("Expected key %s, got %s", cloudTags[i].Key, tag.Key)
		}
		if tag.Value != cloudTags[i].Value {
			t.Errorf("Expected value %s, got %s", cloudTags[i].Value, tag.Value)
		}
	}
}

func TestAWSNodeGroup(t *testing.T) {
	nodeGroup := AWSNodeGroup{
		Name:             "test-nodegroup",
		Region:           "us-west-2",
		SubnetIDs:        []string{"subnet-12345678"},
		SecurityGroupIDs: []string{"sg-12345678"},
		AMIID:            "ami-12345678",
		InstanceTypes:    []string{"t3.medium"},
		CapacityType:     "on-demand",
		MinSize:          1,
		MaxSize:          5,
		DesiredCapacity:  2,
		Tags: map[string]string{
			"Environment": "test",
			"Project":     "meridian",
		},
	}

	if nodeGroup.Name != "test-nodegroup" {
		t.Errorf("Expected name test-nodegroup, got %s", nodeGroup.Name)
	}

	if nodeGroup.Region != "us-west-2" {
		t.Errorf("Expected region us-west-2, got %s", nodeGroup.Region)
	}

	if len(nodeGroup.SubnetIDs) != 1 {
		t.Errorf("Expected 1 subnet ID, got %d", len(nodeGroup.SubnetIDs))
	}

	if nodeGroup.CapacityType != "on-demand" {
		t.Errorf("Expected capacity type on-demand, got %s", nodeGroup.CapacityType)
	}
}

func TestAWSLoadBalancer(t *testing.T) {
	lb := AWSLoadBalancer{
		Name:           "test-alb",
		Type:           "application",
		Scheme:         "internet-facing",
		Subnets:        []string{"subnet-12345678", "subnet-87654321"},
		SecurityGroups: []string{"sg-12345678"},
		Tags: map[string]string{
			"Environment": "test",
			"Project":     "meridian",
		},
	}

	if lb.Name != "test-alb" {
		t.Errorf("Expected name test-alb, got %s", lb.Name)
	}

	if lb.Type != "application" {
		t.Errorf("Expected type application, got %s", lb.Type)
	}

	if lb.Scheme != "internet-facing" {
		t.Errorf("Expected scheme internet-facing, got %s", lb.Scheme)
	}

	if len(lb.Subnets) != 2 {
		t.Errorf("Expected 2 subnets, got %d", len(lb.Subnets))
	}
}
