// Package files talks to S3: presigned uploads (the file goes straight from the app to S3)
// and the reads/writes the worker needs. Bucket names come from one prefix per environment.
package files

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// Logical buckets; the real name is "<prefix>-<name>", e.g. local-bussiness-staging-catalog.
const (
	Catalog        = "catalog"
	PrivateDocs    = "private-docs"
	RentalEvidence = "rental-evidence"
)

var ErrNoObject = errors.New("object not found")

type S3 struct {
	client  *s3.Client
	presign *s3.PresignClient
	prefix  string
}

// New uses the default AWS credential chain (AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY in Railway).
func New(ctx context.Context, region, bucketPrefix string) (*S3, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, err
	}
	c := s3.NewFromConfig(cfg)
	return &S3{client: c, presign: s3.NewPresignClient(c), prefix: bucketPrefix}, nil
}

func (s *S3) bucket(name string) *string { return aws.String(s.prefix + "-" + name) }

// Ping checks credentials and access to a bucket (logged at startup).
func (s *S3) Ping(ctx context.Context, name string) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: s.bucket(name)})
	return err
}

// PresignPut returns a URL valid for 10 minutes that only accepts this exact content type and size.
func (s *S3) PresignPut(ctx context.Context, name, key, contentType string, size int64) (string, error) {
	req, err := s.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        s.bucket(name),
		Key:           aws.String(key),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
	}, s3.WithPresignExpires(10*time.Minute))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

// Head returns size and content type, or ErrNoObject.
func (s *S3) Head(ctx context.Context, name, key string) (int64, string, error) {
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: s.bucket(name), Key: aws.String(key)})
	var nf *types.NotFound
	if errors.As(err, &nf) {
		return 0, "", ErrNoObject
	}
	if err != nil {
		return 0, "", err
	}
	return aws.ToInt64(out.ContentLength), aws.ToString(out.ContentType), nil
}

// Get reads an object, refusing anything bigger than maxBytes.
func (s *S3) Get(ctx context.Context, name, key string, maxBytes int64) ([]byte, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: s.bucket(name), Key: aws.String(key)})
	var nk *types.NoSuchKey
	if errors.As(err, &nk) {
		return nil, ErrNoObject
	}
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()
	data, err := io.ReadAll(io.LimitReader(out.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("object %s larger than %d bytes", key, maxBytes)
	}
	return data, nil
}

// PutImmutable writes a derived file whose key never changes content (cached for a year by the CDN).
func (s *S3) PutImmutable(ctx context.Context, name, key, contentType string, data []byte) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:       s.bucket(name),
		Key:          aws.String(key),
		Body:         bytes.NewReader(data),
		ContentType:  aws.String(contentType),
		CacheControl: aws.String("public, max-age=31536000, immutable"),
	})
	return err
}

func (s *S3) Delete(ctx context.Context, name, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: s.bucket(name), Key: aws.String(key)})
	return err
}
