package oss

import (
	"fmt"
	"sync"

	"github.com/aoxn/meridian/internal/cloud"
	"github.com/aoxn/meridian/internal/cloud/mock/client"
)

// MockData stores mock OSS data
type MockData struct {
	Buckets map[string]string
	Objects map[string][]byte
	mu      sync.RWMutex
}

// Global mock data storage
var mockData = &MockData{
	Buckets: make(map[string]string),
	Objects: make(map[string][]byte),
}

// MockOSS implements OSS interface for mock provider
type MockOSS struct {
	clientMgr *client.MockClientManager
}

// NewOSS creates a new mock OSS module
func NewOSS(clientMgr *client.MockClientManager) cloud.IObjectStorage {
	return &MockOSS{
		clientMgr: clientMgr,
	}
}

// BucketName returns the bucket name
func (m *MockOSS) BucketName() string {
	return "mock-bucket"
}

// EnsureBucket ensures a bucket exists
func (m *MockOSS) EnsureBucket(name string) error {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return err
	}

	if name == "" {
		return fmt.Errorf("EnsureBucket: name is required")
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	if _, exists := mockData.Buckets[name]; exists {
		return fmt.Errorf("EnsureBucket: bucket '%s' already exists", name)
	}

	mockData.Buckets[name] = name

	return nil
}

// GetFile gets a file from mock storage
func (m *MockOSS) GetFile(src, dst string) error {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return err
	}

	// Mock implementation - just return success
	return nil
}

// PutFile puts a file to mock storage
func (m *MockOSS) PutFile(src, dst string) error {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return err
	}

	// Mock implementation - just return success
	return nil
}

// DeleteObject deletes an object from mock storage
func (m *MockOSS) DeleteObject(f string) error {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return err
	}

	mockData.mu.Lock()
	delete(mockData.Objects, f)
	mockData.mu.Unlock()

	return nil
}

// GetObject gets an object from mock storage
func (m *MockOSS) GetObject(src string) ([]byte, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return nil, err
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	if data, exists := mockData.Objects[src]; exists {
		return data, nil
	}

	return []byte("mock object data"), nil
}

// PutObject puts an object to mock storage
func (m *MockOSS) PutObject(b []byte, dst string) error {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return err
	}

	mockData.mu.Lock()
	mockData.Objects[dst] = b
	mockData.mu.Unlock()

	return nil
}

// ListObject lists objects in mock storage
func (m *MockOSS) ListObject(prefix string) ([][]byte, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return nil, err
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	var objects [][]byte
	for key, data := range mockData.Objects {
		if prefix == "" || len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			objects = append(objects, data)
		}
	}

	return objects, nil
}
