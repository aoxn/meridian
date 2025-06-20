package ess

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aoxn/meridian/internal/cloud"
	"github.com/aoxn/meridian/internal/cloud/mock/client"
	corev1 "k8s.io/api/core/v1"
)

// MockData stores mock ESS data
type MockData struct {
	ScalingGroups map[string]*cloud.ScalingGroupModel
	Instances     map[string]*cloud.InstanceModel
	mu            sync.RWMutex
}

// Global mock data storage
var mockData = &MockData{
	ScalingGroups: make(map[string]*cloud.ScalingGroupModel),
	Instances:     make(map[string]*cloud.InstanceModel),
}

// MockESS implements ESS interface for mock provider
type MockESS struct {
	clientMgr *client.MockClientManager
}

// NewESS creates a new mock ESS module
func NewESS(clientMgr *client.MockClientManager) cloud.IElasticScalingGroup {
	return &MockESS{
		clientMgr: clientMgr,
	}
}

// FindESSBy finds a mock scaling group
func (m *MockESS) FindESSBy(ctx context.Context, id cloud.Id) (cloud.ScalingGroupModel, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return cloud.ScalingGroupModel{}, err
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	for _, sg := range mockData.ScalingGroups {
		if sg.ScalingGroupId == id.Id || sg.ScalingGroupName == id.Name {
			return *sg, nil
		}
	}

	return cloud.ScalingGroupModel{}, cloud.NotFound
}

// ListESS lists mock scaling groups
func (m *MockESS) ListESS(ctx context.Context, id cloud.Id) ([]cloud.ScalingGroupModel, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return nil, err
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	var sgs []cloud.ScalingGroupModel
	for _, sg := range mockData.ScalingGroups {
		if id.Region != "" && sg.Region != id.Region {
			continue
		}
		sgs = append(sgs, *sg)
	}

	return sgs, nil
}

// CreateESS creates a mock scaling group
func (m *MockESS) CreateESS(ctx context.Context, id string, ess cloud.ScalingGroupModel) (string, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return "", err
	}

	// 参数校验
	if ess.ScalingGroupName == "" {
		return "", fmt.Errorf("CreateESS: ScalingGroupName is required")
	}
	if ess.Region == "" {
		return "", fmt.Errorf("CreateESS: Region is required")
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	// 唯一性校验
	for _, exist := range mockData.ScalingGroups {
		if exist.ScalingGroupName == ess.ScalingGroupName {
			return "", fmt.Errorf("CreateESS: ScalingGroup with name '%s' already exists", ess.ScalingGroupName)
		}
		if ess.ScalingGroupId != "" && exist.ScalingGroupId == ess.ScalingGroupId {
			return "", fmt.Errorf("CreateESS: ScalingGroup with id '%s' already exists", ess.ScalingGroupId)
		}
	}

	sgID := fmt.Sprintf("asg-mock-%d", time.Now().UnixNano())
	ess.ScalingGroupId = sgID
	ess.Region = m.clientMgr.GetRegion()

	mockData.ScalingGroups[sgID] = &ess

	return sgID, nil
}

// UpdateESS updates a mock scaling group
func (m *MockESS) UpdateESS(ctx context.Context, ess cloud.ScalingGroupModel) error {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return err
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	if _, exists := mockData.ScalingGroups[ess.ScalingGroupId]; !exists {
		return cloud.NotFound
	}

	mockData.ScalingGroups[ess.ScalingGroupId] = &ess
	return nil
}

// DeleteESS deletes a mock scaling group
func (m *MockESS) DeleteESS(ctx context.Context, essid cloud.ScalingGroupModel) error {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return err
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	if _, exists := mockData.ScalingGroups[essid.ScalingGroupId]; !exists {
		return cloud.NotFound
	}

	delete(mockData.ScalingGroups, essid.ScalingGroupId)
	return nil
}

// ScaleNodeGroup scales a mock scaling group
func (m *MockESS) ScaleNodeGroup(ctx context.Context, model cloud.ScalingGroupModel, desired uint) error {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return err
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	if sg, exists := mockData.ScalingGroups[model.ScalingGroupId]; exists {
		sg.DesiredCapacity = int(desired)
	}

	return nil
}

// FindScalingConfig finds a mock scaling config
func (m *MockESS) FindScalingConfig(ctx context.Context, id cloud.Id) (cloud.ScalingConfig, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return cloud.ScalingConfig{}, err
	}

	return cloud.ScalingConfig{}, cloud.NotFound
}

// FindScalingRule finds a mock scaling rule
func (m *MockESS) FindScalingRule(ctx context.Context, id cloud.Id) (cloud.ScalingRule, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return cloud.ScalingRule{}, err
	}

	return cloud.ScalingRule{}, cloud.NotFound
}

// CreateScalingConfig creates a mock scaling config
func (m *MockESS) CreateScalingConfig(ctx context.Context, id string, cfg cloud.ScalingConfig) (string, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return "", err
	}

	return fmt.Sprintf("cfg-mock-%d", time.Now().Unix()), nil
}

// CreateScalingRule creates a mock scaling rule
func (m *MockESS) CreateScalingRule(ctx context.Context, id string, rule cloud.ScalingRule) (cloud.ScalingRule, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return cloud.ScalingRule{}, err
	}

	return cloud.ScalingRule{}, nil
}

// ExecuteScalingRule executes a mock scaling rule
func (m *MockESS) ExecuteScalingRule(ctx context.Context, id string) (string, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return "", err
	}

	return fmt.Sprintf("exec-mock-%d", time.Now().Unix()), nil
}

// DeleteScalingConfig deletes a mock scaling config
func (m *MockESS) DeleteScalingConfig(ctx context.Context, cfgId string) error {
	m.clientMgr.SimulateDelay()

	return m.clientMgr.SimulateFailure()
}

// DeleteScalingRule deletes a mock scaling rule
func (m *MockESS) DeleteScalingRule(ctx context.Context, ruleId string) error {
	m.clientMgr.SimulateDelay()

	return m.clientMgr.SimulateFailure()
}

// EnableScalingGroup enables a mock scaling group
func (m *MockESS) EnableScalingGroup(ctx context.Context, gid, sid string) error {
	m.clientMgr.SimulateDelay()

	return m.clientMgr.SimulateFailure()
}

// MockInstance implements Instance interface for mock provider
type MockInstance struct {
	clientMgr *client.MockClientManager
}

// NewInstance creates a new mock Instance module
func NewInstance(clientMgr *client.MockClientManager) cloud.IInstance {
	return &MockInstance{
		clientMgr: clientMgr,
	}
}

// GetInstanceId gets instance ID from node
func (m *MockInstance) GetInstanceId(node *corev1.Node) string {
	return fmt.Sprintf("i-mock-%d", time.Now().Unix())
}

// FindInstance finds a mock instance
func (m *MockInstance) FindInstance(ctx context.Context, id cloud.Id) (cloud.InstanceModel, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return cloud.InstanceModel{}, err
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	for _, instance := range mockData.Instances {
		// Check tags for instance ID
		for _, tag := range instance.Tag {
			if tag.Key == "InstanceId" && tag.Value == id.Id {
				return *instance, nil
			}
		}
	}

	return cloud.InstanceModel{}, cloud.NotFound
}

// ListInstance lists mock instances
func (m *MockInstance) ListInstance(ctx context.Context, i cloud.Id) ([]cloud.InstanceModel, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return nil, err
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	var instances []cloud.InstanceModel
	for _, instance := range mockData.Instances {
		instances = append(instances, *instance)
	}

	return instances, nil
}

// CreateInstance creates a mock instance
func (m *MockInstance) CreateInstance(ctx context.Context, i cloud.InstanceModel) (string, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return "", err
	}

	// 参数校验
	var name string
	for _, tag := range i.Tag {
		if tag.Key == "Name" {
			name = tag.Value
			break
		}
	}
	if name == "" {
		return "", fmt.Errorf("CreateInstance: Name tag is required")
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	// 唯一性校验（只用Name）
	for _, exist := range mockData.Instances {
		for _, tag := range exist.Tag {
			if tag.Key == "Name" && tag.Value == name {
				return "", fmt.Errorf("CreateInstance: instance with name '%s' already exists", name)
			}
		}
	}

	instanceID := fmt.Sprintf("i-mock-%d", time.Now().UnixNano())
	i.Tag = append(i.Tag, cloud.Tag{Key: "InstanceId", Value: instanceID})

	mockData.Instances[instanceID] = &i

	return instanceID, nil
}

// UpdateInstance updates a mock instance
func (m *MockInstance) UpdateInstance(ctx context.Context, i cloud.InstanceModel) error {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return err
	}

	// Find instance by ID in tags
	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	for _, tag := range i.Tag {
		if tag.Key == "InstanceId" {
			if _, exists := mockData.Instances[tag.Value]; exists {
				mockData.Instances[tag.Value] = &i
				return nil
			}
		}
	}

	return cloud.NotFound
}

// DeleteInstance deletes a mock instance
func (m *MockInstance) DeleteInstance(ctx context.Context, id cloud.Id) error {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return err
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	if _, exists := mockData.Instances[id.Id]; exists {
		delete(mockData.Instances, id.Id)
		return nil
	}

	return cloud.NotFound
}

// RunCommand runs a command on a mock instance
func (m *MockInstance) RunCommand(ctx context.Context, id cloud.Id, command string) (string, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return "", err
	}

	return "mock command output", nil
}
