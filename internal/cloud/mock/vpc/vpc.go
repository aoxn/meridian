package vpc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aoxn/meridian/internal/cloud"
	"github.com/aoxn/meridian/internal/cloud/mock/client"
)

// MockData stores mock resource data
type MockData struct {
	VPCs      map[string]*cloud.VpcModel
	VSwitches map[string]*cloud.VSwitchModel
	EIPs      map[string]*cloud.EipModel
	mu        sync.RWMutex
}

// Global mock data storage
var mockData = &MockData{
	VPCs:      make(map[string]*cloud.VpcModel),
	VSwitches: make(map[string]*cloud.VSwitchModel),
	EIPs:      make(map[string]*cloud.EipModel),
}

// MockVPC implements VPC interface for mock provider
type MockVPC struct {
	clientMgr *client.MockClientManager
}

// NewVpc creates a new mock VPC module
func NewVpc(clientMgr *client.MockClientManager) cloud.IVpc {
	return &MockVPC{
		clientMgr: clientMgr,
	}
}

// FindVPC finds a mock VPC
func (m *MockVPC) FindVPC(ctx context.Context, id cloud.Id) (cloud.VpcModel, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return cloud.VpcModel{}, err
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	if id.Id != "" {
		if vpc, ok := mockData.VPCs[id.Id]; ok {
			return *vpc, nil
		}
	} else if id.Name != "" {
		for _, vpc := range mockData.VPCs {
			if vpc.VpcName == id.Name {
				return *vpc, nil
			}
		}
	} else {
		return cloud.VpcModel{}, fmt.Errorf("FindVPC: both id and name are empty")
	}
	return cloud.VpcModel{}, cloud.NotFound
}

// ListVPC lists mock VPCs
func (m *MockVPC) ListVPC(ctx context.Context, id cloud.Id) ([]cloud.VpcModel, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return nil, err
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	var vpcs []cloud.VpcModel
	for _, vpc := range mockData.VPCs {
		if id.Region != "" && vpc.Region != id.Region {
			continue
		}
		vpcs = append(vpcs, *vpc)
	}

	return vpcs, nil
}

// CreateVPC creates a mock VPC
func (m *MockVPC) CreateVPC(ctx context.Context, vpc cloud.VpcModel) (string, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return "", err
	}

	// 参数校验
	if vpc.VpcName == "" {
		return "", fmt.Errorf("CreateVPC: VpcName is required")
	}
	if vpc.Cidr == "" {
		return "", fmt.Errorf("CreateVPC: Cidr is required")
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	// 检查是否已存在同名或同ID的VPC
	for _, exist := range mockData.VPCs {
		if exist.VpcName == vpc.VpcName {
			return "", fmt.Errorf("CreateVPC: VPC with name '%s' already exists", vpc.VpcName)
		}
		if vpc.VpcId != "" && exist.VpcId == vpc.VpcId {
			return "", fmt.Errorf("CreateVPC: VPC with id '%s' already exists", vpc.VpcId)
		}
	}

	vpcID := fmt.Sprintf("vpc-mock-%d", time.Now().UnixNano())
	vpc.VpcId = vpcID
	vpc.Region = m.clientMgr.GetRegion()

	mockData.VPCs[vpcID] = &vpc

	return vpcID, nil
}

// UpdateVPC updates a mock VPC
func (m *MockVPC) UpdateVPC(ctx context.Context, vpc cloud.VpcModel) error {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return err
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	if _, exists := mockData.VPCs[vpc.VpcId]; !exists {
		return cloud.NotFound
	}

	mockData.VPCs[vpc.VpcId] = &vpc
	return nil
}

// DeleteVPC deletes a mock VPC
func (m *MockVPC) DeleteVPC(ctx context.Context, id string) error {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return err
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	if _, exists := mockData.VPCs[id]; !exists {
		return cloud.NotFound
	}

	delete(mockData.VPCs, id)
	return nil
}

// MockVSwitch implements VSwitch interface for mock provider
type MockVSwitch struct {
	clientMgr *client.MockClientManager
}

// NewVswitch creates a new mock VSwitch module
func NewVswitch(clientMgr *client.MockClientManager) cloud.IVSwitch {
	return &MockVSwitch{
		clientMgr: clientMgr,
	}
}

// FindVSwitch finds a mock VSwitch
func (m *MockVSwitch) FindVSwitch(ctx context.Context, vpcid string, id cloud.Id) (cloud.VSwitchModel, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return cloud.VSwitchModel{}, err
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	for _, vswitch := range mockData.VSwitches {
		if vswitch.VSwitchId == id.Id || vswitch.VSwitchName == id.Name {
			return *vswitch, nil
		}
	}

	return cloud.VSwitchModel{}, cloud.NotFound
}

// ListVSwitch lists mock VSwitches
func (m *MockVSwitch) ListVSwitch(ctx context.Context, vpcid string, id cloud.Id) ([]cloud.VSwitchModel, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return nil, err
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	var vswitches []cloud.VSwitchModel
	for _, vswitch := range mockData.VSwitches {
		vswitches = append(vswitches, *vswitch)
	}

	return vswitches, nil
}

// CreateVSwitch creates a mock VSwitch
func (m *MockVSwitch) CreateVSwitch(ctx context.Context, vpcid string, model cloud.VSwitchModel) (string, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return "", err
	}

	// 参数校验
	if vpcid == "" {
		return "", fmt.Errorf("CreateVSwitch: vpcid is required")
	}
	if model.VSwitchName == "" {
		return "", fmt.Errorf("CreateVSwitch: VSwitchName is required")
	}
	if model.CidrBlock == "" {
		return "", fmt.Errorf("CreateVSwitch: CidrBlock is required")
	}
	if model.ZoneId == "" {
		return "", fmt.Errorf("CreateVSwitch: ZoneId is required")
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	// 唯一性校验
	for _, exist := range mockData.VSwitches {
		if exist.VSwitchName == model.VSwitchName {
			return "", fmt.Errorf("CreateVSwitch: VSwitch with name '%s' already exists", model.VSwitchName)
		}
		if model.VSwitchId != "" && exist.VSwitchId == model.VSwitchId {
			return "", fmt.Errorf("CreateVSwitch: VSwitch with id '%s' already exists", model.VSwitchId)
		}
	}

	vswitchID := fmt.Sprintf("vsw-mock-%d", time.Now().UnixNano())
	model.VSwitchId = vswitchID

	mockData.VSwitches[vswitchID] = &model

	return vswitchID, nil
}

// UpdateVSwitch updates a mock VSwitch
func (m *MockVSwitch) UpdateVSwitch(ctx context.Context, vpcid string, model cloud.VSwitchModel) error {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return err
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	if _, exists := mockData.VSwitches[model.VSwitchId]; !exists {
		return cloud.NotFound
	}

	mockData.VSwitches[model.VSwitchId] = &model
	return nil
}

// DeleteVSwitch deletes a mock VSwitch
func (m *MockVSwitch) DeleteVSwitch(ctx context.Context, vpcId string, id cloud.Id) error {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return err
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	if _, exists := mockData.VSwitches[id.Id]; !exists {
		return cloud.NotFound
	}

	delete(mockData.VSwitches, id.Id)
	return nil
}

// MockEIP implements EIP interface for mock provider
type MockEIP struct {
	clientMgr *client.MockClientManager
}

// NewEip creates a new mock EIP module
func NewEip(clientMgr *client.MockClientManager) cloud.IEip {
	return &MockEIP{
		clientMgr: clientMgr,
	}
}

// FindEIP finds a mock EIP
func (m *MockEIP) FindEIP(ctx context.Context, id cloud.Id) (cloud.EipModel, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return cloud.EipModel{}, err
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	for _, eip := range mockData.EIPs {
		if eip.EipId == id.Id || eip.EipName == id.Name {
			return *eip, nil
		}
	}

	return cloud.EipModel{}, cloud.NotFound
}

// ListEIP lists mock EIPs
func (m *MockEIP) ListEIP(ctx context.Context, id cloud.Id) ([]cloud.EipModel, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return nil, err
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	var eips []cloud.EipModel
	for _, eip := range mockData.EIPs {
		if id.Region != "" && eip.Region != id.Region {
			continue
		}
		eips = append(eips, *eip)
	}

	return eips, nil
}

// CreateEIP creates a mock EIP
func (m *MockEIP) CreateEIP(ctx context.Context, eip cloud.EipModel) (string, error) {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return "", err
	}

	// 参数校验
	if eip.EipName == "" {
		return "", fmt.Errorf("CreateEIP: EipName is required")
	}
	if eip.Region == "" {
		return "", fmt.Errorf("CreateEIP: Region is required")
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	// 唯一性校验
	for _, exist := range mockData.EIPs {
		if exist.EipName == eip.EipName {
			return "", fmt.Errorf("CreateEIP: EIP with name '%s' already exists", eip.EipName)
		}
		if eip.EipId != "" && exist.EipId == eip.EipId {
			return "", fmt.Errorf("CreateEIP: EIP with id '%s' already exists", eip.EipId)
		}
	}

	eipID := fmt.Sprintf("eip-mock-%d", time.Now().UnixNano())
	eip.EipId = eipID
	eip.Region = m.clientMgr.GetRegion()

	mockData.EIPs[eipID] = &eip

	return eipID, nil
}

// UpdateEIP updates a mock EIP
func (m *MockEIP) UpdateEIP(ctx context.Context, eip cloud.EipModel) error {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return err
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	if _, exists := mockData.EIPs[eip.EipId]; !exists {
		return cloud.NotFound
	}

	mockData.EIPs[eip.EipId] = &eip
	return nil
}

// DeleteEIP deletes a mock EIP
func (m *MockEIP) DeleteEIP(ctx context.Context, id cloud.Id) error {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return err
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	if _, exists := mockData.EIPs[id.Id]; !exists {
		return cloud.NotFound
	}

	delete(mockData.EIPs, id.Id)
	return nil
}

// BindEIP binds a mock EIP
func (m *MockEIP) BindEIP(ctx context.Context, eip cloud.EipModel) error {
	m.clientMgr.SimulateDelay()

	if err := m.clientMgr.SimulateFailure(); err != nil {
		return err
	}

	mockData.mu.Lock()
	defer mockData.mu.Unlock()

	if _, exists := mockData.EIPs[eip.EipId]; !exists {
		return cloud.NotFound
	}

	mockData.EIPs[eip.EipId] = &eip
	return nil
}
