# AWS Cloud Provider for Meridian

This document describes the AWS cloud provider implementation for the Meridian project.

## Overview

The AWS cloud provider enables Meridian to manage AWS resources including:
- VPC and Subnets
- Security Groups
- Auto Scaling Groups
- Elastic IPs
- IAM Roles and Policies
- Application Load Balancers
- S3 Object Storage
- EC2 Instances

## Features

### Core Infrastructure
- **VPC Management**: Create, update, and delete VPCs with custom CIDR blocks
- **Subnet Management**: Manage subnets across multiple availability zones
- **Security Groups**: Configure network security with ingress/egress rules

### Compute Resources
- **Auto Scaling Groups**: Automatically scale EC2 instances based on demand
- **EC2 Instances**: Launch and manage individual EC2 instances
- **Launch Templates**: Define instance configurations for consistent deployments

### Networking
- **Elastic IPs**: Allocate and associate static IP addresses
- **Load Balancers**: Create Application Load Balancers (ALB) and Network Load Balancers (NLB)
- **Target Groups**: Configure load balancer target groups for traffic routing

### Security & Access
- **IAM Roles**: Create and manage IAM roles for EC2 instances
- **IAM Policies**: Define permissions and attach policies to roles
- **Instance Profiles**: Associate IAM roles with EC2 instances

### Storage
- **S3 Buckets**: Create and manage S3 buckets for object storage
- **Object Operations**: Upload, download, and manage objects in S3

## Configuration

### Provider Configuration

```yaml
apiVersion: v1
kind: Provider
metadata:
  name: aws-provider
spec:
  type: aws
  authInfo:
    region: us-west-2
    accessKey: "your-access-key-id"
    accessSecret: "your-secret-access-key"
```

### Authentication Methods

1. **Access Keys**: Provide AWS access key ID and secret access key
2. **IAM Roles**: Use IAM roles attached to EC2 instances or EKS pods
3. **Environment Variables**: Set `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`
4. **AWS CLI Configuration**: Use credentials from `~/.aws/credentials`

### Cluster Configuration

```yaml
apiVersion: v1
kind: Cluster
metadata:
  name: aws-cluster
spec:
  provider: aws-provider
  region: us-west-2
  vpc:
    cidr: "10.0.0.0/16"
    vpcName: "meridian-vpc"
  subnets:
    - zoneId: "us-west-2a"
      cidrBlock: "10.0.1.0/24"
      vswitchName: "meridian-subnet-1"
  nodeGroups:
    - name: "worker-nodes"
      minSize: 1
      maxSize: 5
      desiredCapacity: 2
      instanceTypes: ["t3.medium"]
      amiId: "ami-12345678"
```

## Usage Examples

### Creating a VPC

```go
vpc := cloud.VpcModel{
    Cidr:    "10.0.0.0/16",
    VpcName: "my-vpc",
    Tag: []cloud.Tag{
        {Key: "Environment", Value: "production"},
        {Key: "Project", Value: "meridian"},
    },
}

vpcID, err := awsProvider.CreateVPC(ctx, vpc)
```

### Creating an Auto Scaling Group

```go
asg := cloud.ScalingGroupModel{
    ScalingGroupName: "worker-asg",
    Min:              1,
    Max:              5,
    DesiredCapacity:  2,
    VSwitchId: []cloud.VSwitchModel{
        {VSwitchId: "subnet-12345678"},
    },
    ScalingConfig: cloud.ScalingConfig{
        ImageId:       "ami-12345678",
        InstanceType:  "t3.medium",
        SecurityGrpId: "sg-12345678",
    },
}

asgID, err := awsProvider.CreateESS(ctx, "", asg)
```

### Managing Security Groups

```go
sg := cloud.SecurityGroupModel{
    SecurityGroupName: "web-sg",
    Region:           "us-west-2",
    Tag: []cloud.Tag{
        {Key: "Name", Value: "web-security-group"},
    },
}

sgID, err := awsProvider.CreateSecurityGroup(ctx, "vpc-12345678", sg)
```

### Working with S3

```go
// Upload a file
err := awsProvider.PutFile("/local/path/file.txt", "remote/path/file.txt")

// Download a file
err := awsProvider.GetFile("remote/path/file.txt", "/local/path/file.txt")

// Get object content
content, err := awsProvider.GetObject("remote/path/file.txt")
```

## Best Practices

### Security
1. **Use IAM Roles**: Prefer IAM roles over access keys when possible
2. **Least Privilege**: Grant minimal required permissions to IAM roles
3. **Security Groups**: Restrict network access using security group rules
4. **Encryption**: Enable encryption for EBS volumes and S3 buckets

### Networking
1. **Multi-AZ Deployment**: Use multiple availability zones for high availability
2. **Private Subnets**: Place worker nodes in private subnets
3. **NAT Gateways**: Use NAT gateways for outbound internet access from private subnets
4. **VPC Endpoints**: Use VPC endpoints for AWS service access

### Cost Optimization
1. **Spot Instances**: Use spot instances for non-critical workloads
2. **Right Sizing**: Choose appropriate instance types
3. **Auto Scaling**: Implement proper auto scaling policies
4. **Resource Tagging**: Tag resources for cost allocation

### Monitoring
1. **CloudWatch**: Enable CloudWatch monitoring for EC2 instances
2. **Logging**: Configure logging for load balancers and S3 access
3. **Metrics**: Monitor key metrics like CPU, memory, and network usage

## Limitations

1. **Region Specific**: Resources are created in the specified AWS region
2. **VPC Limits**: AWS has limits on VPCs, subnets, and security groups per region
3. **Instance Types**: Some instance types may not be available in all regions
4. **AMI Compatibility**: Ensure AMIs are compatible with your target region

## Troubleshooting

### Common Issues

1. **Authentication Errors**
   - Verify access keys are correct
   - Check IAM permissions
   - Ensure credentials are not expired

2. **VPC Creation Failures**
   - Verify CIDR block is valid
   - Check VPC limits in the region
   - Ensure no conflicting VPCs exist

3. **Instance Launch Failures**
   - Verify AMI exists in the region
   - Check instance type availability
   - Ensure security groups allow required traffic

4. **Auto Scaling Issues**
   - Verify launch template configuration
   - Check subnet availability
   - Ensure IAM roles have required permissions

### Debugging

Enable debug logging by setting the log level:

```go
klog.SetLevel(klog.LevelDebug)
```

Check AWS CloudTrail for API call logs and error details.

## API Reference

### Cloud Interface Methods

The AWS provider implements all methods defined in the `cloud.Cloud` interface:

- `IVpc`: VPC management operations
- `IVSwitch`: Subnet management operations
- `ISecurityGroup`: Security group operations
- `IElasticScalingGroup`: Auto Scaling Group operations
- `IInstance`: EC2 instance operations
- `IEip`: Elastic IP operations
- `IRamRole`: IAM role and policy operations
- `ISlb`: Load balancer operations
- `IObjectStorage`: S3 operations

### Error Handling

The provider returns standard errors:
- `cloud.NotFound`: Resource not found
- `UnexpectedResponse`: Unexpected API response
- Custom error messages for specific failures

## Contributing

When contributing to the AWS provider:

1. Follow AWS best practices
2. Add proper error handling
3. Include unit tests
4. Update documentation
5. Test with multiple AWS regions
6. Consider cost implications of new features 