package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/aoxn/meridian/internal/cloud"
	"github.com/aoxn/meridian/internal/cloud/aws/client"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"k8s.io/klog/v2"
)

type s3Storage struct {
	mgr        *client.ClientMgr
	bucketName string
}

func NewS3(mgr *client.ClientMgr) cloud.IObjectStorage {
	return &s3Storage{
		mgr:        mgr,
		bucketName: "meridian-storage", // Default bucket name
	}
}

func (s *s3Storage) BucketName() string {
	return s.bucketName
}

func (s *s3Storage) EnsureBucket(name string) error {
	if name != "" {
		s.bucketName = name
	}

	// Check if bucket exists
	_, err := s.mgr.S3Client.HeadBucket(context.TODO(), &s3.HeadBucketInput{
		Bucket: aws.String(s.bucketName),
	})

	if err == nil {
		// Bucket exists
		return nil
	}

	// Create bucket if it doesn't exist
	input := &s3.CreateBucketInput{
		Bucket: aws.String(s.bucketName),
	}

	_, err = s.mgr.S3Client.CreateBucket(context.TODO(), input)
	if err != nil {
		return fmt.Errorf("failed to create S3 bucket: %w", err)
	}

	klog.Infof("Created S3 bucket: %s", s.bucketName)
	return nil
}

func (s *s3Storage) GetFile(src, dst string) error {
	// Download file from S3
	input := &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(src),
	}

	result, err := s.mgr.S3Client.GetObject(context.TODO(), input)
	if err != nil {
		return fmt.Errorf("failed to get object from S3: %w", err)
	}
	defer result.Body.Close()

	// Create destination directory if it doesn't exist
	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create destination file
	file, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Copy content
	_, err = io.Copy(file, result.Body)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	klog.Infof("Downloaded file from S3: %s -> %s", src, dst)
	return nil
}

func (s *s3Storage) PutFile(src, dst string) error {
	// Open source file
	file, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer file.Close()

	// Upload file to S3
	input := &s3.PutObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(dst),
		Body:   file,
	}

	_, err = s.mgr.S3Client.PutObject(context.TODO(), input)
	if err != nil {
		return fmt.Errorf("failed to put object to S3: %w", err)
	}

	klog.Infof("Uploaded file to S3: %s -> %s", src, dst)
	return nil
}

func (s *s3Storage) DeleteObject(f string) error {
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(f),
	}

	_, err := s.mgr.S3Client.DeleteObject(context.TODO(), input)
	if err != nil {
		return fmt.Errorf("failed to delete object from S3: %w", err)
	}

	klog.Infof("Deleted object from S3: %s", f)
	return nil
}

func (s *s3Storage) GetObject(src string) ([]byte, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(src),
	}

	result, err := s.mgr.S3Client.GetObject(context.TODO(), input)
	if err != nil {
		return nil, fmt.Errorf("failed to get object from S3: %w", err)
	}
	defer result.Body.Close()

	// Read all content
	content, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read object content: %w", err)
	}

	return content, nil
}

func (s *s3Storage) PutObject(b []byte, dst string) error {
	input := &s3.PutObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(dst),
		Body:   bytes.NewReader(b),
	}

	_, err := s.mgr.S3Client.PutObject(context.TODO(), input)
	if err != nil {
		return fmt.Errorf("failed to put object to S3: %w", err)
	}

	klog.Infof("Put object to S3: %s (%d bytes)", dst, len(b))
	return nil
}

func (s *s3Storage) ListObject(prefix string) ([][]byte, error) {
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucketName),
		Prefix: aws.String(prefix),
	}

	result, err := s.mgr.S3Client.ListObjectsV2(context.TODO(), input)
	if err != nil {
		return nil, fmt.Errorf("failed to list objects from S3: %w", err)
	}

	var objects [][]byte
	for _, obj := range result.Contents {
		if obj.Key != nil {
			content, err := s.GetObject(*obj.Key)
			if err != nil {
				klog.Warningf("Failed to get object %s: %v", *obj.Key, err)
				continue
			}
			objects = append(objects, content)
		}
	}

	return objects, nil
}
