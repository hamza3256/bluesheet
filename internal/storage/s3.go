package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/hamza3256/bluesheet/internal/config"
)

// Uploader abstracts object storage uploads so the worker doesn't depend on S3 directly.
type Uploader interface {
	Upload(ctx context.Context, bucket, key string, body io.Reader) (etag string, err error)
}

type S3Uploader struct {
	client *s3.Client
}

func NewS3Uploader(ctx context.Context, cfg *config.Config) (*S3Uploader, error) {
	resolver := aws.EndpointResolverWithOptionsFunc(
		func(service, region string, opts ...interface{}) (aws.Endpoint, error) {
			if cfg.S3Endpoint != "" {
				return aws.Endpoint{
					URL:               cfg.S3Endpoint,
					HostnameImmutable: true,
				}, nil
			}
			return aws.Endpoint{}, &aws.EndpointNotFoundError{}
		},
	)

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.S3Region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.S3AccessKey, cfg.S3SecretKey, ""),
		),
		awsconfig.WithEndpointResolverWithOptions(resolver),
	)
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	return &S3Uploader{client: client}, nil
}

func (u *S3Uploader) Upload(ctx context.Context, bucket, key string, body io.Reader) (string, error) {
	out, err := u.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   body,
	})
	if err != nil {
		return "", fmt.Errorf("s3 put: %w", err)
	}
	etag := ""
	if out.ETag != nil {
		etag = *out.ETag
	}
	return etag, nil
}