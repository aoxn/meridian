package mock

import (
	"context"
	"fmt"
	"testing"

	"github.com/aoxn/meridian/internal/cloud"
)

func TestNewMockProvider(t *testing.T) {
	cfg := cloud.Config{}
	provider, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create mock provider: %v", err)
	}

	if provider == nil {
		t.Fatal("Provider should not be nil")
	}

	// Just verify that GetConfig returns something
	_ = provider.GetConfig()
}

func TestMockVPCOperations(t *testing.T) {
	cfg := cloud.Config{}
	provider, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create mock provider: %v", err)
	}

	ctx := context.Background()

	// Test CreateVPC
	vpcModel := cloud.VpcModel{
		VpcName: "test-vpc",
		Cidr:    "10.0.0.0/16",
		Region:  "mock-region",
	}

	vpcID, err := provider.CreateVPC(ctx, vpcModel)
	if err != nil {
		t.Fatalf("Failed to create VPC: %v", err)
	}

	if vpcID == "" {
		t.Fatal("VPC ID should not be empty")
	}

	// Test FindVPC
	foundVPC, err := provider.FindVPC(ctx, cloud.Id{Id: vpcID})
	if err != nil {
		t.Fatalf("Failed to find VPC: %v", err)
	}

	if foundVPC.VpcId != vpcID {
		t.Errorf("Expected VPC ID %s, got %s", vpcID, foundVPC.VpcId)
	}

	if foundVPC.VpcName != "test-vpc" {
		t.Errorf("Expected VPC name 'test-vpc', got %s", foundVPC.VpcName)
	}

	// Test ListVPC
	vpcs, err := provider.ListVPC(ctx, cloud.Id{})
	if err != nil {
		t.Fatalf("Failed to list VPCs: %v", err)
	}

	if len(vpcs) == 0 {
		t.Fatal("Should have at least one VPC")
	}

	// Test DeleteVPC
	err = provider.DeleteVPC(ctx, vpcID)
	if err != nil {
		t.Fatalf("Failed to delete VPC: %v", err)
	}

	// Verify VPC is deleted
	_, err = provider.FindVPC(ctx, cloud.Id{Id: vpcID})
	if err != cloud.NotFound {
		t.Errorf("Expected NotFound error, got %v", err)
	}
}

func TestMockVSwitchOperations(t *testing.T) {
	cfg := cloud.Config{}
	provider, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create mock provider: %v", err)
	}

	ctx := context.Background()

	// Test CreateVSwitch
	vswitchModel := cloud.VSwitchModel{
		VSwitchName: "test-vswitch",
		CidrBlock:   "10.0.1.0/24",
		ZoneId:      "mock-zone-1",
	}

	vswitchID, err := provider.CreateVSwitch(ctx, "vpc-mock-1", vswitchModel)
	if err != nil {
		t.Fatalf("Failed to create VSwitch: %v", err)
	}

	if vswitchID == "" {
		t.Fatal("VSwitch ID should not be empty")
	}

	// Test FindVSwitch
	foundVSwitch, err := provider.FindVSwitch(ctx, "vpc-mock-1", cloud.Id{Id: vswitchID})
	if err != nil {
		t.Fatalf("Failed to find VSwitch: %v", err)
	}

	if foundVSwitch.VSwitchId != vswitchID {
		t.Errorf("Expected VSwitch ID %s, got %s", vswitchID, foundVSwitch.VSwitchId)
	}

	// Test ListVSwitch
	vswitches, err := provider.ListVSwitch(ctx, "vpc-mock-1", cloud.Id{})
	if err != nil {
		t.Fatalf("Failed to list VSwitches: %v", err)
	}

	if len(vswitches) == 0 {
		t.Fatal("Should have at least one VSwitch")
	}
}

func TestMockEIPOperations(t *testing.T) {
	cfg := cloud.Config{}
	provider, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create mock provider: %v", err)
	}

	ctx := context.Background()

	// Test CreateEIP
	eipModel := cloud.EipModel{
		EipName: "test-eip",
		Region:  "mock-region",
	}

	eipID, err := provider.CreateEIP(ctx, eipModel)
	if err != nil {
		t.Fatalf("Failed to create EIP: %v", err)
	}

	if eipID == "" {
		t.Fatal("EIP ID should not be empty")
	}

	// Test FindEIP
	foundEIP, err := provider.FindEIP(ctx, cloud.Id{Id: eipID})
	if err != nil {
		t.Fatalf("Failed to find EIP: %v", err)
	}

	if foundEIP.EipId != eipID {
		t.Errorf("Expected EIP ID %s, got %s", eipID, foundEIP.EipId)
	}

	// Test ListEIP
	eips, err := provider.ListEIP(ctx, cloud.Id{})
	if err != nil {
		t.Fatalf("Failed to list EIPs: %v", err)
	}

	if len(eips) == 0 {
		t.Fatal("Should have at least one EIP")
	}
}

func TestMockESSOperations(t *testing.T) {
	cfg := cloud.Config{}
	provider, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create mock provider: %v", err)
	}

	ctx := context.Background()

	// Test CreateESS
	essModel := cloud.ScalingGroupModel{
		ScalingGroupName: "test-asg",
		Region:           "mock-region",
		Min:              1,
		Max:              10,
		DesiredCapacity:  3,
	}

	essID, err := provider.CreateESS(ctx, "test-group", essModel)
	if err != nil {
		t.Fatalf("Failed to create ESS: %v", err)
	}

	if essID == "" {
		t.Fatal("ESS ID should not be empty")
	}

	// Test FindESSBy
	foundESS, err := provider.FindESSBy(ctx, cloud.Id{Id: essID})
	if err != nil {
		t.Fatalf("Failed to find ESS: %v", err)
	}

	if foundESS.ScalingGroupId != essID {
		t.Errorf("Expected ESS ID %s, got %s", essID, foundESS.ScalingGroupId)
	}

	// Test ScaleNodeGroup
	err = provider.ScaleNodeGroup(ctx, foundESS, 5)
	if err != nil {
		t.Fatalf("Failed to scale node group: %v", err)
	}
}

func TestMockInstanceOperations(t *testing.T) {
	cfg := cloud.Config{}
	provider, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create mock provider: %v", err)
	}

	ctx := context.Background()

	// Test CreateInstance
	instanceModel := cloud.InstanceModel{
		Tag: []cloud.Tag{
			{Key: "Name", Value: "test-instance"},
		},
	}

	instanceID, err := provider.CreateInstance(ctx, instanceModel)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}

	if instanceID == "" {
		t.Fatal("Instance ID should not be empty")
	}

	// Test ListInstance
	instances, err := provider.ListInstance(ctx, cloud.Id{})
	if err != nil {
		t.Fatalf("Failed to list instances: %v", err)
	}

	if len(instances) == 0 {
		t.Fatal("Should have at least one instance")
	}

	// Test RunCommand
	output, err := provider.RunCommand(ctx, cloud.Id{Id: instanceID}, "ls -la")
	if err != nil {
		t.Fatalf("Failed to run command: %v", err)
	}

	if output == "" {
		t.Fatal("Command output should not be empty")
	}
}

func TestMockSecurityGroupOperations(t *testing.T) {
	cfg := cloud.Config{}
	provider, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create mock provider: %v", err)
	}

	ctx := context.Background()

	// Test CreateSecurityGroup
	sgModel := cloud.SecurityGroupModel{
		SecurityGroupName: "test-sg",
		Region:            "mock-region",
	}

	sgID, err := provider.CreateSecurityGroup(ctx, "vpc-mock-1", sgModel)
	if err != nil {
		t.Fatalf("Failed to create security group: %v", err)
	}

	if sgID == "" {
		t.Fatal("Security group ID should not be empty")
	}

	// Test FindSecurityGroup
	foundSG, err := provider.FindSecurityGroup(ctx, "vpc-mock-1", cloud.Id{Id: sgID})
	if err != nil {
		t.Fatalf("Failed to find security group: %v", err)
	}

	if foundSG.SecurityGroupId != sgID {
		t.Errorf("Expected security group ID %s, got %s", sgID, foundSG.SecurityGroupId)
	}
}

func TestMockSLBOperations(t *testing.T) {
	cfg := cloud.Config{}
	provider, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create mock provider: %v", err)
	}

	ctx := context.Background()

	// Test CreateSLB
	slbModel := cloud.SlbModel{
		VSwitchId: "vsw-mock-1",
	}

	slbID, err := provider.CreateSLB(ctx, slbModel)
	if err != nil {
		t.Fatalf("Failed to create SLB: %v", err)
	}

	if slbID == "" {
		t.Fatal("SLB ID should not be empty")
	}

	// Test ListSLB
	slbs, err := provider.ListSLB(ctx, cloud.Id{})
	if err != nil {
		t.Fatalf("Failed to list SLBs: %v", err)
	}

	if len(slbs) == 0 {
		t.Fatal("Should have at least one SLB")
	}
}

func TestMockRAMOperations(t *testing.T) {
	cfg := cloud.Config{}
	provider, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create mock provider: %v", err)
	}

	ctx := context.Background()

	// Test CreateRAM
	ramModel := cloud.RamModel{
		RamName: "test-role",
		Arn:     "arn:mock:iam::123456789012:role/TestRole",
	}

	roleID, err := provider.CreateRAM(ctx, ramModel)
	if err != nil {
		t.Fatalf("Failed to create RAM role: %v", err)
	}

	if roleID == "" {
		t.Fatal("Role ID should not be empty")
	}

	// Test FindRAM
	foundRAM, err := provider.FindRAM(ctx, cloud.Id{Id: roleID})
	if err != nil {
		t.Fatalf("Failed to find RAM role: %v", err)
	}

	if foundRAM.RamId != roleID {
		t.Errorf("Expected role ID %s, got %s", roleID, foundRAM.RamId)
	}
}

func TestMockOSSOperations(t *testing.T) {
	cfg := cloud.Config{}
	provider, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create mock provider: %v", err)
	}

	// Test BucketName
	bucketName := provider.BucketName()
	if bucketName == "" {
		t.Fatal("Bucket name should not be empty")
	}

	// Test EnsureBucket
	err = provider.EnsureBucket("test-bucket")
	if err != nil {
		t.Fatalf("Failed to ensure bucket: %v", err)
	}

	// Test PutObject
	testData := []byte("test object data")
	err = provider.PutObject(testData, "test-object")
	if err != nil {
		t.Fatalf("Failed to put object: %v", err)
	}

	// Test GetObject
	retrievedData, err := provider.GetObject("test-object")
	if err != nil {
		t.Fatalf("Failed to get object: %v", err)
	}

	if len(retrievedData) == 0 {
		t.Fatal("Retrieved data should not be empty")
	}

	// Test ListObject
	objects, err := provider.ListObject("test")
	if err != nil {
		t.Fatalf("Failed to list objects: %v", err)
	}

	if len(objects) == 0 {
		t.Fatal("Should have at least one object")
	}

	// Test DeleteObject
	err = provider.DeleteObject("test-object")
	if err != nil {
		t.Fatalf("Failed to delete object: %v", err)
	}
}

func TestMockProviderConcurrency(t *testing.T) {
	cfg := cloud.Config{}
	provider, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create mock provider: %v", err)
	}

	ctx := context.Background()

	// Test concurrent operations
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()

			vpcModel := cloud.VpcModel{
				VpcName: fmt.Sprintf("concurrent-vpc-%d", id),
				Cidr:    fmt.Sprintf("10.%d.0.0/16", id),
				Region:  "mock-region",
			}

			vpcID, err := provider.CreateVPC(ctx, vpcModel)
			if err != nil {
				t.Errorf("Failed to create VPC %d: %v", id, err)
				return
			}

			// Verify the VPC was created by ID
			foundVPC, err := provider.FindVPC(ctx, cloud.Id{Id: vpcID})
			if err != nil {
				t.Errorf("Failed to find VPC %d: %v", id, err)
				return
			}

			if foundVPC.VpcId != vpcID {
				t.Errorf("Expected VPC ID %s, got %s", vpcID, foundVPC.VpcId)
			}

			// Verify the VPC was created by name
			foundVPCByName, err := provider.FindVPC(ctx, cloud.Id{Name: fmt.Sprintf("concurrent-vpc-%d", id)})
			if err != nil {
				t.Errorf("Failed to find VPC %d by name %s: %v", id, fmt.Sprintf("concurrent-vpc-%d", id), err)
				return
			}

			if foundVPCByName.VpcName != fmt.Sprintf("concurrent-vpc-%d", id) {
				t.Errorf("Expected VPC name 'concurrent-vpc-%d', got %s", id, foundVPCByName.VpcName)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}
