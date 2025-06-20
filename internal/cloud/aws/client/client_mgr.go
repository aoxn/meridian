package client

import (
	"context"
	"fmt"
	"time"

	api "github.com/aoxn/meridian/api/v1"
	"github.com/aoxn/meridian/internal/cloud"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

const (
	CloudAWS        = "cloud.aws"
	TokenSyncPeriod = 10 * time.Minute
)

// ClientMgr client manager for AWS provider
type ClientMgr struct {
	auth api.AuthInfo
	stop <-chan struct{}

	// AWS SDK v2 clients
	EC2Client         *ec2.Client
	AutoScalingClient *autoscaling.Client
	ELBClient         *elasticloadbalancingv2.Client
	IAMClient         *iam.Client
	S3Client          *s3.Client
	SSMClient         *ssm.Client

	// AWS configuration
	Config aws.Config
	Region string
}

// NewClientMgr return a new client manager
func NewClientMgr(auth api.AuthInfo) (*ClientMgr, error) {
	// Create AWS configuration
	var cfg aws.Config
	var err error

	if auth.AccessKey != "" && auth.AccessSecret != "" {
		// Use static credentials
		cfg, err = config.LoadDefaultConfig(context.TODO(),
			config.WithRegion(auth.Region),
			config.WithCredentialsProvider(aws.CredentialsProviderFunc(
				func(ctx context.Context) (aws.Credentials, error) {
					return aws.Credentials{
						AccessKeyID:     auth.AccessKey,
						SecretAccessKey: auth.AccessSecret,
					}, nil
				},
			)),
		)
	} else {
		// Use default credential chain (IAM roles, environment variables, etc.)
		cfg, err = config.LoadDefaultConfig(context.TODO(),
			config.WithRegion(auth.Region),
		)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create clients
	ec2Client := ec2.NewFromConfig(cfg)
	asgClient := autoscaling.NewFromConfig(cfg)
	elbClient := elasticloadbalancingv2.NewFromConfig(cfg)
	iamClient := iam.NewFromConfig(cfg)
	s3Client := s3.NewFromConfig(cfg)
	ssmClient := ssm.NewFromConfig(cfg)

	mgr := &ClientMgr{
		auth:              auth,
		EC2Client:         ec2Client,
		AutoScalingClient: asgClient,
		ELBClient:         elbClient,
		IAMClient:         iamClient,
		S3Client:          s3Client,
		SSMClient:         ssmClient,
		Config:            cfg,
		Region:            auth.Region,
	}

	return mgr, nil
}

// GetRegion returns the region
func (m *ClientMgr) GetRegion() string {
	return m.Region
}

// GetAccessKey returns the access key
func (m *ClientMgr) GetAccessKey() string {
	return m.auth.AccessKey
}

// GetAccessSecret returns the access secret
func (m *ClientMgr) GetAccessSecret() string {
	return m.auth.AccessSecret
}

// GetAccountID returns the AWS account ID
func (m *ClientMgr) GetAccountID() (string, error) {
	// This would typically call STS GetCallerIdentity
	// For now, return empty string
	return "", nil
}

// ValidateCredentials validates AWS credentials
func (m *ClientMgr) ValidateCredentials(ctx context.Context) error {
	// Test credentials by calling a simple API
	_, err := m.EC2Client.DescribeRegions(ctx, &ec2.DescribeRegionsInput{})
	return err
}

// CreateTags creates AWS tags from cloud.Tag slice
func (m *ClientMgr) CreateTags(tags []cloud.Tag) []ec2types.Tag {
	var awsTags []ec2types.Tag
	for _, tag := range tags {
		awsTags = append(awsTags, ec2types.Tag{
			Key:   aws.String(tag.Key),
			Value: aws.String(tag.Value),
		})
	}
	return awsTags
}

// ConvertToCloudTags converts AWS tags to cloud.Tag slice
func (m *ClientMgr) ConvertToCloudTags(awsTags []ec2types.Tag) []cloud.Tag {
	var tags []cloud.Tag
	for _, tag := range awsTags {
		if tag.Key != nil && tag.Value != nil {
			tags = append(tags, cloud.Tag{
				Key:   *tag.Key,
				Value: *tag.Value,
			})
		}
	}
	return tags
}
