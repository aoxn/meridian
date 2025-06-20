package slb

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aoxn/meridian/internal/cloud"
	"github.com/aoxn/meridian/internal/cloud/mock/client"
)

// MockData stores mock SLB data
type MockData struct {
	LoadBalancers map[string]*cloud.SlbModel
	mu            sync.RWMutex
}

// Global mock data storage
var mockData = &MockData{
	LoadBalancers: make(map[string]*cloud.SlbModel),
}

// MockSLB implements SLB interface for mock provider
type MockSLB struct {
	clientMgr *client.MockClientManager
}

// NewSLB creates a new mock SLB module
func NewSLB(clientMgr *client.MockClientManager) cloud.ISlb {
	return &MockSLB{
		clientMgr: clientMgr,
	}
}

// FindSLB finds a mock load balancer
func (m *MockSLB) FindSLB(ctx context.Context, id cloud.Id) (cloud.SlbModel, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return cloud.SlbModel{}, err
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	for _, lb := range mockData.LoadBalancers {
		// Mock implementation - return first one found
		return *lb, nil
	}

	return cloud.SlbModel{}, cloud.NotFound
}

// ListSLB lists mock load balancers
func (m *MockSLB) ListSLB(ctx context.Context, id cloud.Id) ([]cloud.SlbModel, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return nil, err
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	var lbs []cloud.SlbModel
	for _, lb := range mockData.LoadBalancers {
		lbs = append(lbs, *lb)
	}

	return lbs, nil
}

// CreateSLB creates a mock load balancer
func (m *MockSLB) CreateSLB(ctx context.Context, b cloud.SlbModel) (string, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return "", err
	}

	// 参数校验
	if b.VSwitchId == "" {
		return "", fmt.Errorf("CreateSLB: VSwitchId is required")
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	// 唯一性校验（只用VSwitchId）
	for _, exist := range mockData.LoadBalancers {
		if exist.VSwitchId == b.VSwitchId {
			return "", fmt.Errorf("CreateSLB: SLB with VSwitchId '%s' already exists", b.VSwitchId)
		}
	}

	lbID := fmt.Sprintf("lb-mock-%d", time.Now().UnixNano())

	mockData.LoadBalancers[lbID] = &b

	return lbID, nil
}

// UpdateSLB updates a mock load balancer
func (m *MockSLB) UpdateSLB(ctx context.Context, b cloud.SlbModel) error {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return err
	}

	// Mock implementation - just return success
	return nil
}

// DeleteSLB deletes a mock load balancer
func (m *MockSLB) DeleteSLB(ctx context.Context, id cloud.Id) error {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return err
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	if _, exists := mockData.LoadBalancers[id.Id]; exists {
		delete(mockData.LoadBalancers, id.Id)
		return nil
	}

	return cloud.NotFound
}

// FindListener finds a mock listener
func (m *MockSLB) FindListener(ctx context.Context, id cloud.Id) (cloud.SlbModel, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return cloud.SlbModel{}, err
	}

	return cloud.SlbModel{}, cloud.NotFound
}

// CreateListener creates a mock listener
func (m *MockSLB) CreateListener(ctx context.Context, b cloud.SlbModel) (string, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return "", err
	}

	return fmt.Sprintf("listener-mock-%d", time.Now().Unix()), nil
}

// UpdateListener updates a mock listener
func (m *MockSLB) UpdateListener(ctx context.Context, b cloud.SlbModel) error {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return err
	}

	return nil
}

// DeleteListener deletes a mock listener
func (m *MockSLB) DeleteListener(ctx context.Context, id cloud.Id) error {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return err
	}

	return nil
}
