package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type S3Storage struct {
	client     *s3.Client
	bucket     string
	region     string
	baseURL    string
	publicURLs bool
}

type S3Config struct {
	Bucket     string
	Region     string
	BaseURL    string
	PublicURLs bool
	AccessKey  string
	SecretKey  string
}

func NewS3Storage(ctx context.Context, cfg S3Config) (*S3Storage, error) {
	var awsCfg aws.Config
	var err error

	// If access key and secret are provided, use them
	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		awsCfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(cfg.Region),
			config.WithCredentialsProvider(aws.CredentialsProviderFunc(
				func(ctx context.Context) (aws.Credentials, error) {
					return aws.Credentials{
						AccessKeyID:     cfg.AccessKey,
						SecretAccessKey: cfg.SecretKey,
					}, nil
				},
			)),
		)
	} else {
		// Fall back to default credential chain
		awsCfg, err = config.LoadDefaultConfig(ctx, config.WithRegion(cfg.Region))
	}

	if err != nil {
		return nil, fmt.Errorf("unable to load AWS SDK config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg)

	storage := &S3Storage{
		client:     client,
		bucket:     cfg.Bucket,
		region:     cfg.Region,
		baseURL:    cfg.BaseURL,
		publicURLs: cfg.PublicURLs,
	}

	return storage, nil
}

func (s *S3Storage) Save(ctx context.Context, path string, contents io.Reader, options ...Option) error {
	opts := NewOptions(options...)

	putParams := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
		Body:   contents,
	}

	if opts.ContentType != "" {
		putParams.ContentType = aws.String(opts.ContentType)
	}

	if opts.ContentDisposition != "" {
		putParams.ContentDisposition = aws.String(opts.ContentDisposition)
	}

	if opts.CacheControl != "" {
		putParams.CacheControl = aws.String(opts.CacheControl)
	}

	if len(opts.Metadata) > 0 {
		putParams.Metadata = opts.Metadata
	}

	if opts.Visibility == "public" {
		putParams.ACL = types.ObjectCannedACLPublicRead
	}

	_, err := s.client.PutObject(ctx, putParams)
	if err != nil {
		return fmt.Errorf("failed to upload file to S3: %w", err)
	}

	return nil
}

func (s *S3Storage) SaveFromURL(ctx context.Context, path string, url string, options ...Option) error {

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request for URL: %w", err)
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download file from URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download file from URL, status: %s", resp.Status)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "" {
		hasContentType := false
		for _, opt := range options {
			testOpts := &Options{}
			opt(testOpts)
			if testOpts.ContentType != "" {
				hasContentType = true
				break
			}
		}

		if !hasContentType {
			options = append(options, WithContentType(contentType))
		}
	}

	return s.Save(ctx, path, resp.Body, options...)
}

func (s *S3Storage) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object from S3: %w", err)
	}

	return result.Body, nil
}

func (s *S3Storage) Exists(ctx context.Context, path string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	})

	if err != nil {

		var notFound *types.NotFound
		if ok := errors.As(err, &notFound); ok {
			return false, nil
		}
		return false, fmt.Errorf("failed to check if object exists: %w", err)
	}

	return true, nil
}

func (s *S3Storage) Delete(ctx context.Context, path string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return fmt.Errorf("failed to delete object from S3: %w", err)
	}

	return nil
}

func (s *S3Storage) URL(path string) string {
	path = filepath.Clean(path)

	if s.baseURL != "" {
		return fmt.Sprintf("%s/%s", s.baseURL, path)
	}

	if s.publicURLs {
		return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucket, s.region, path)
	}

	return ""
}

func (s *S3Storage) TemporaryURL(ctx context.Context, path string, expiry int64) (string, error) {
	presignClient := s3.NewPresignClient(s.client)

	request, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = time.Duration(expiry) * time.Second
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return request.URL, nil
}
