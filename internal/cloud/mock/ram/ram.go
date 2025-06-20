package ram

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aoxn/meridian/internal/cloud"
	"github.com/aoxn/meridian/internal/cloud/mock/client"
)

// MockData stores mock RAM data
type MockData struct {
	IAMRoles map[string]*cloud.RamModel
	mu       sync.RWMutex
}

// Global mock data storage
var mockData = &MockData{
	IAMRoles: make(map[string]*cloud.RamModel),
}

// MockRAM implements RAM interface for mock provider
type MockRAM struct {
	clientMgr *client.MockClientManager
}

// NewRAM creates a new mock RAM module
func NewRAM(clientMgr *client.MockClientManager) cloud.IRamRole {
	return &MockRAM{
		clientMgr: clientMgr,
	}
}

// FindRAM finds a mock IAM role
func (m *MockRAM) FindRAM(ctx context.Context, id cloud.Id) (cloud.RamModel, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return cloud.RamModel{}, err
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	for _, role := range mockData.IAMRoles {
		if role.RamId == id.Id || role.RamName == id.Name {
			return *role, nil
		}
	}

	return cloud.RamModel{}, cloud.NotFound
}

// ListRAM lists mock IAM roles
func (m *MockRAM) ListRAM(ctx context.Context, id cloud.Id) ([]cloud.RamModel, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return nil, err
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	var roles []cloud.RamModel
	for _, role := range mockData.IAMRoles {
		roles = append(roles, *role)
	}

	return roles, nil
}

// CreateRAM creates a mock IAM role
func (m *MockRAM) CreateRAM(ctx context.Context, m2 cloud.RamModel) (string, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return "", err
	}

	// 参数校验
	if m2.RamName == "" {
		return "", fmt.Errorf("CreateRAM: RamName is required")
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	// 唯一性校验
	for _, exist := range mockData.IAMRoles {
		if exist.RamName == m2.RamName {
			return "", fmt.Errorf("CreateRAM: RAM with name '%s' already exists", m2.RamName)
		}
		if m2.RamId != "" && exist.RamId == m2.RamId {
			return "", fmt.Errorf("CreateRAM: RAM with id '%s' already exists", m2.RamId)
		}
	}

	roleID := fmt.Sprintf("role-mock-%d", time.Now().UnixNano())
	m2.RamId = roleID

	mockData.IAMRoles[roleID] = &m2

	return roleID, nil
}

// UpdateRAM updates a mock IAM role
func (m *MockRAM) UpdateRAM(ctx context.Context, m2 cloud.RamModel) error {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return err
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	if _, exists := mockData.IAMRoles[m2.RamId]; !exists {
		return cloud.NotFound
	}

	mockData.IAMRoles[m2.RamId] = &m2
	return nil
}

// DeleteRAM deletes a mock IAM role
func (m *MockRAM) DeleteRAM(ctx context.Context, id cloud.Id, policyName string) error {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return err
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	if _, exists := mockData.IAMRoles[id.Id]; exists {
		delete(mockData.IAMRoles, id.Id)
		return nil
	}

	return cloud.NotFound
}

// FindPolicy finds a mock policy
func (m *MockRAM) FindPolicy(ctx context.Context, m2 cloud.Id) (cloud.RamModel, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return cloud.RamModel{}, err
	}

	return cloud.RamModel{}, cloud.NotFound
}

// CreatePolicy creates a mock policy
func (m *MockRAM) CreatePolicy(ctx context.Context, m2 cloud.RamModel) (cloud.RamModel, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return cloud.RamModel{}, err
	}

	return cloud.RamModel{}, nil
}

// AttachPolicyToRole attaches a policy to a role
func (m *MockRAM) AttachPolicyToRole(ctx context.Context, m2 cloud.RamModel) (cloud.RamModel, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return cloud.RamModel{}, err
	}

	return cloud.RamModel{}, nil
}

// ListPoliciesForRole lists policies for a role
func (m *MockRAM) ListPoliciesForRole(ctx context.Context, m2 cloud.RamModel) (cloud.RamModel, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return cloud.RamModel{}, err
	}

	return cloud.RamModel{}, nil
}
