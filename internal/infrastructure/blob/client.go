package blob

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	appcfg "cassyork.dev/platform/internal/config"
)

// Client uploads objects to the configured S3-compatible bucket (MinIO, Linode OS, etc.).
type Client struct {
	bucket string
	api    *s3.Client
}

// New builds an S3 API client from shared application settings.
func New(cfg appcfg.ObjectStorage) (*Client, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	bucket := strings.TrimSpace(cfg.Bucket)
	if endpoint == "" || bucket == "" {
		return nil, fmt.Errorf("blob: OBJECT_STORAGE_ENDPOINT and OBJECT_STORAGE_BUCKET are required")
	}
	endpoint = strings.TrimRight(endpoint, "/")

	ctx := context.Background()
	awscfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("blob: aws config: %w", err)
	}

	api := s3.NewFromConfig(awscfg, func(o *s3.Options) {
		o.UsePathStyle = cfg.UsePathStyle
		o.BaseEndpoint = aws.String(endpoint)
	})

	return &Client{bucket: bucket, api: api}, nil
}

// Put writes an object. Pass size >= 0 when known (recommended for stable uploads).
func (c *Client) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	key = strings.Trim(key, "/")
	if key == "" {
		return fmt.Errorf("blob: empty object key")
	}
	in := &s3.PutObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
		Body:   body,
	}
	if contentType != "" {
		in.ContentType = aws.String(contentType)
	}
	if size >= 0 {
		in.ContentLength = aws.Int64(size)
	}
	_, err := c.api.PutObject(ctx, in)
	if err != nil {
		return fmt.Errorf("blob: PutObject %s: %w", key, err)
	}
	return nil
}
