package client

import (
	"context"
	"fmt"
	"time"
)

// MockConfig holds configuration for mock provider
type MockConfig struct {
	Region         string         `yaml:"region"`
	MockDelay      time.Duration  `yaml:"mock_delay"`
	EnableFailures bool           `yaml:"enable_failures"`
	FailureRate    float64        `yaml:"failure_rate"`
	MockResources  map[string]int `yaml:"mock_resources"`
}

// MockClientManager manages mock cloud clients
type MockClientManager struct {
	config *MockConfig
	ctx    context.Context
}

// NewClientMgr creates a new mock client manager
func NewClientMgr() *MockClientManager {
	config := &MockConfig{
		Region:         "mock-region",
		MockDelay:      100 * time.Millisecond,
		EnableFailures: false,
		FailureRate:    0.1,
		MockResources: map[string]int{
			"vpc":            5,
			"vswitch":        10,
			"eip":            20,
			"scaling_group":  3,
			"instance":       15,
			"security_group": 8,
			"load_balancer":  2,
			"iam_role":       5,
			"s3_bucket":      3,
		},
	}

	return &MockClientManager{
		config: config,
		ctx:    context.Background(),
	}
}

// GetContext returns the context
func (m *MockClientManager) GetContext() context.Context {
	return m.ctx
}

// SetContext sets the context
func (m *MockClientManager) SetContext(ctx context.Context) {
	m.ctx = ctx
}

// GetRegion returns the region
func (m *MockClientManager) GetRegion() string {
	return m.config.Region
}

// GetConfig returns the configuration
func (m *MockClientManager) GetConfig() *MockConfig {
	return m.config
}

// TestConnection tests the connection to mock cloud
func (m *MockClientManager) TestConnection() error {
	// Simulate connection test delay
	time.Sleep(m.config.MockDelay)

	// Simulate potential failure
	if m.config.EnableFailures && time.Now().UnixNano()%100 < int64(m.config.FailureRate*100) {
		return fmt.Errorf("mock connection test failed")
	}

	return nil
}

// Close closes the client manager
func (m *MockClientManager) Close() error {
	time.Sleep(m.config.MockDelay)
	return nil
}

// SimulateFailure simulates a failure based on configuration
func (m *MockClientManager) SimulateFailure() error {
	if !m.config.EnableFailures {
		return nil
	}

	// Simple random failure simulation
	if time.Now().UnixNano()%100 < int64(m.config.FailureRate*100) {
		return fmt.Errorf("mock provider simulated failure")
	}
	return nil
}

// SimulateDelay simulates network delay
func (m *MockClientManager) SimulateDelay() {
	time.Sleep(m.config.MockDelay)
}
