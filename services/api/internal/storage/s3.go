package storage

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const MaxImageBytes int64 = 16 << 20

type Config struct{ Endpoint, Region, Bucket, AccessKey, SecretKey string }
type S3 struct {
	Client *s3.Client
	Bucket string
}
type Object struct {
	Key      string
	Modified time.Time
}

var safeKey = regexp.MustCompile(`^open-flow/[a-f0-9-]{36}/[a-f0-9-]{36}/[a-f0-9-]{36}$`)

func ValidKey(key string) bool { return safeKey.MatchString(key) }
func New(c Config) (*S3, error) {
	u, err := url.Parse(c.Endpoint)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Scheme != "https" && u.Scheme != "http" || c.Bucket == "" || c.AccessKey == "" || c.SecretKey == "" {
		return nil, errors.New("invalid private storage configuration")
	}
	if u.Scheme == "http" && u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" && u.Hostname() != "seaweedfs" {
		return nil, errors.New("remote storage requires HTTPS")
	}
	client := s3.NewFromConfig(aws.Config{Region: c.Region, Credentials: credentials.NewStaticCredentialsProvider(c.AccessKey, c.SecretKey, ""), HTTPClient: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, RetryMaxAttempts: 3}, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(c.Endpoint)
		o.UsePathStyle = true
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})
	return &S3{Client: client, Bucket: c.Bucket}, nil
}
func (s *S3) Ready(ctx context.Context) error {
	_, err := s.Client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.Bucket)})
	if err != nil {
		return errors.New("private storage unavailable")
	}
	return nil
}

// Streaming uploads set no public ACL. Caller checks bytes/MIME before this step.
func (s *S3) Put(ctx context.Context, key, mime string, size int64, body io.Reader) error {
	if !ValidKey(key) || size < 1 || size > MaxImageBytes || mime != "image/png" && mime != "image/jpeg" && mime != "image/webp" {
		return errors.New("invalid image upload")
	}
	_, err := s.Client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(s.Bucket), Key: aws.String(key), ContentType: aws.String(mime), ContentLength: aws.Int64(size), Body: body})
	if err != nil {
		return errors.New("private image upload failed")
	}
	return nil
}
func (s *S3) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	if !ValidKey(key) {
		return nil, errors.New("invalid artifact key")
	}
	result, err := s.Client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.Bucket), Key: aws.String(key)})
	if err != nil {
		return nil, errors.New("private image unavailable")
	}
	return result.Body, nil
}
func (s *S3) Delete(ctx context.Context, key string) error {
	if !ValidKey(key) {
		return errors.New("invalid artifact key")
	}
	_, err := s.Client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.Bucket), Key: aws.String(key)})
	if err != nil {
		return errors.New("private image deletion failed")
	}
	return nil
}

// Sweep deletes expired/tombstoned/orphan objects after a grace period. Dry run
// is the CLI default. The caller must fence uploads while apply runs.
func (s *S3) Sweep(ctx context.Context, refs map[string]bool, before time.Time, apply bool) (int, error) {
	count := 0
	pages := s3.NewListObjectsV2Paginator(s.Client, &s3.ListObjectsV2Input{Bucket: aws.String(s.Bucket), Prefix: aws.String("open-flow/")})
	for pages.HasMorePages() {
		p, err := pages.NextPage(ctx)
		if err != nil {
			return count, errors.New("storage inventory failed")
		}
		for _, o := range p.Contents {
			key := aws.ToString(o.Key)
			if !ValidKey(key) || refs[key] || o.LastModified == nil || !o.LastModified.Before(before) {
				continue
			}
			count++
			if apply {
				if err = s.Delete(ctx, key); err != nil {
					return count, err
				}
			}
		}
	}
	return count, nil
}
