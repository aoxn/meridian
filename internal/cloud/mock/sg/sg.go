package sg

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aoxn/meridian/internal/cloud"
	"github.com/aoxn/meridian/internal/cloud/mock/client"
)

// MockData stores mock security group data
type MockData struct {
	SecurityGroups map[string]*cloud.SecurityGroupModel
	mu             sync.RWMutex
}

// Global mock data storage
var mockData = &MockData{
	SecurityGroups: make(map[string]*cloud.SecurityGroupModel),
}

// MockSecurityGroup implements SecurityGroup interface for mock provider
type MockSecurityGroup struct {
	clientMgr *client.MockClientManager
}

// NewSecurityGrp creates a new mock SecurityGroup module
func NewSecurityGrp(clientMgr *client.MockClientManager) cloud.ISecurityGroup {
	return &MockSecurityGroup{
		clientMgr: clientMgr,
	}
}

// FindSecurityGroup finds a mock security group
func (m *MockSecurityGroup) FindSecurityGroup(ctx context.Context, vpcid string, id cloud.Id) (cloud.SecurityGroupModel, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return cloud.SecurityGroupModel{}, err
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	for _, sg := range mockData.SecurityGroups {
		if sg.SecurityGroupId == id.Id || sg.SecurityGroupName == id.Name {
			return *sg, nil
		}
	}

	return cloud.SecurityGroupModel{}, cloud.NotFound
}

// ListSecurityGroup lists mock security groups
func (m *MockSecurityGroup) ListSecurityGroup(ctx context.Context, vpcid string, id cloud.Id) ([]cloud.SecurityGroupModel, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return nil, err
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	var sgs []cloud.SecurityGroupModel
	for _, sg := range mockData.SecurityGroups {
		if id.Region != "" && sg.Region != id.Region {
			continue
		}
		sgs = append(sgs, *sg)
	}

	return sgs, nil
}

// CreateSecurityGroup creates a mock security group
func (m *MockSecurityGroup) CreateSecurityGroup(ctx context.Context, vpcid string, grp cloud.SecurityGroupModel) (string, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return "", err
	}

	// 参数校验
	if grp.SecurityGroupName == "" {
		return "", fmt.Errorf("CreateSecurityGroup: SecurityGroupName is required")
	}
	if grp.Region == "" {
		return "", fmt.Errorf("CreateSecurityGroup: Region is required")
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	// 唯一性校验
	for _, exist := range mockData.SecurityGroups {
		if exist.SecurityGroupName == grp.SecurityGroupName {
			return "", fmt.Errorf("CreateSecurityGroup: SecurityGroup with name '%s' already exists", grp.SecurityGroupName)
		}
		if grp.SecurityGroupId != "" && exist.SecurityGroupId == grp.SecurityGroupId {
			return "", fmt.Errorf("CreateSecurityGroup: SecurityGroup with id '%s' already exists", grp.SecurityGroupId)
		}
	}

	sgID := fmt.Sprintf("sg-mock-%d", time.Now().UnixNano())
	grp.SecurityGroupId = sgID
	grp.Region = m.clientMgr.GetRegion()

	mockData.SecurityGroups[sgID] = &grp

	return sgID, nil
}

// UpdateSecurityGroup updates a mock security group
func (m *MockSecurityGroup) UpdateSecurityGroup(ctx context.Context, grp cloud.SecurityGroupModel) error {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return err
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	if _, exists := mockData.SecurityGroups[grp.SecurityGroupId]; !exists {
		return cloud.NotFound
	}

	mockData.SecurityGroups[grp.SecurityGroupId] = &grp
	return nil
}

// DeleteSecurityGroup deletes a mock security group
func (m *MockSecurityGroup) DeleteSecurityGroup(ctx context.Context, id cloud.Id) error {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return err
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	if _, exists := mockData.SecurityGroups[id.Id]; exists {
		delete(mockData.SecurityGroups, id.Id)
		return nil
	}

	return cloud.NotFound
}
