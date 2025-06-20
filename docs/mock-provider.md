# Mock Cloud Provider

Mock cloud provider is a testing and development tool that simulates cloud provider behavior without requiring actual cloud credentials or resources.

## Features

- **No Cloud Credentials Required**: Perfect for local development and testing
- **Simulated Delays**: Mimics real network latency
- **Configurable Failures**: Can simulate API failures for testing error handling
- **In-Memory Storage**: Uses maps and slices to store mock data
- **Thread-Safe**: Supports concurrent operations
- **Full Interface Compliance**: Implements all cloud provider interfaces

## Configuration

### Basic Configuration

```yaml
apiVersion: v1
kind: Config
metadata:
  name: mock-provider-config
spec:
  provider: mock
  region: mock-region
  authInfo:
    accessKeyId: mock-access-key
    accessKeySecret: mock-secret-key
```

### Advanced Configuration

```yaml
apiVersion: v1
kind: Config
metadata:
  name: mock-provider-config
spec:
  provider: mock
  region: mock-region
  authInfo:
    accessKeyId: mock-access-key
    accessKeySecret: mock-secret-key
  mockConfig:
    region: mock-region
    mockDelay: 100ms          # Simulated network delay
    enableFailures: false     # Enable/disable failure simulation
    failureRate: 0.1          # Failure rate (0.0 to 1.0)
    mockResources:
      vpc: 5                  # Number of mock VPCs
      vswitch: 10             # Number of mock VSwitches
      eip: 20                 # Number of mock EIPs
      scalingGroup: 3         # Number of mock scaling groups
      instance: 15            # Number of mock instances
      securityGroup: 8        # Number of mock security groups
      loadBalancer: 2         # Number of mock load balancers
      iamRole: 5              # Number of mock IAM roles
      s3Bucket: 3             # Number of mock S3 buckets
```

## Supported Resources

### VPC
- Create, find, list, update, delete VPCs
- Simulates VPC lifecycle management

### VSwitch
- Create, find, list, update, delete VSwitches
- Associated with VPCs

### EIP (Elastic IP)
- Create, find, list, update, delete EIPs
- Bind/unbind operations

### Auto Scaling Group (ESS)
- Create, find, list, update, delete scaling groups
- Scale operations
- Scaling configurations and rules

### EC2 Instances
- Create, find, list, update, delete instances
- Command execution simulation

### Security Groups
- Create, find, list, update, delete security groups
- Associated with VPCs

### Load Balancer (SLB)
- Create, find, list, update, delete load balancers
- Listener management

### IAM Roles (RAM)
- Create, find, list, update, delete IAM roles
- Policy management

### Object Storage (OSS/S3)
- Bucket operations
- Object upload/download
- File operations

## Usage Examples

### Creating a VPC

```go
ctx := context.Background()
vpcModel := cloud.VpcModel{
    VpcName: "test-vpc",
    Cidr:    "10.0.0.0/16",
    Region:  "mock-region",
}

vpcID, err := provider.CreateVPC(ctx, vpcModel)
if err != nil {
    log.Fatalf("Failed to create VPC: %v", err)
}
```

### Finding Resources

```go
// Find by ID
vpc, err := provider.FindVPC(ctx, cloud.Id{Id: "vpc-mock-123"})

// Find by name
vpc, err := provider.FindVPC(ctx, cloud.Id{Name: "test-vpc"})
```

### Listing Resources

```go
// List all VPCs
vpcs, err := provider.ListVPC(ctx, cloud.Id{})

// List VPCs in specific region
vpcs, err := provider.ListVPC(ctx, cloud.Id{Region: "mock-region"})
```

## Testing

### Running Tests

```bash
# Run all mock provider tests
go test ./internal/cloud/mock/...

# Run specific test
go test ./internal/cloud/mock/ -run TestMockVPCOperations

# Run with verbose output
go test ./internal/cloud/mock/ -v
```

### Test Coverage

```bash
# Generate coverage report
go test ./internal/cloud/mock/ -cover

# Generate detailed coverage report
go test ./internal/cloud/mock/ -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Development

### Adding New Resources

1. Create a new module in `internal/cloud/mock/`
2. Implement the corresponding cloud interface
3. Add the module to the main mock provider
4. Add tests for the new module

### Extending Mock Data

Each module has its own mock data storage:

```go
type MockData struct {
    Resources map[string]*ResourceModel
    mu        sync.RWMutex
}

var mockData = &MockData{
    Resources: make(map[string]*ResourceModel),
}
```

### Simulating Failures

Enable failure simulation in the client manager:

```go
config := &MockConfig{
    EnableFailures: true,
    FailureRate:    0.2, // 20% failure rate
}
```

## Best Practices

1. **Use for Testing**: Mock provider is designed for testing, not production
2. **Configure Delays**: Set appropriate delays to simulate real network conditions
3. **Test Error Handling**: Enable failures to test error scenarios
4. **Clean Up**: Mock data persists in memory, clean up after tests
5. **Thread Safety**: All operations are thread-safe for concurrent testing

## Limitations

- **In-Memory Only**: Data is not persisted between runs
- **No Real Cloud**: Cannot test actual cloud provider behavior
- **Limited Validation**: Basic validation only, not comprehensive
- **No Network**: Cannot test actual network connectivity

## Troubleshooting

### Common Issues

1. **Import Cycle**: Ensure no circular dependencies between modules
2. **Interface Compliance**: Verify all interface methods are implemented
3. **Thread Safety**: Use proper locking for concurrent access
4. **Test Isolation**: Clean up mock data between tests

### Debug Mode

Enable debug logging to see mock operations:

```go
// Set log level to debug
log.SetLevel(log.DebugLevel)
```

## Contributing

When contributing to the mock provider:

1. Follow the existing code structure
2. Add comprehensive tests
3. Update documentation
4. Ensure thread safety
5. Maintain interface compliance 