package files

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// R2Config reaches a Cloudflare R2 bucket (or any S3-compatible store).
type R2Config struct {
	Endpoint        string // https://<account id>.r2.cloudflarestorage.com
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	PublicURL       string // where the bucket is served, for example https://media.staykey.direct
}

// R2 stores files in an R2 bucket.
type R2 struct {
	client  *s3.Client
	presign *s3.PresignClient
	cfg     R2Config
	TTL     time.Duration
}

var _ Store = (*R2)(nil)

// NewR2 returns a store for the bucket in cfg.
func NewR2(cfg R2Config) (*R2, error) {
	if cfg.Endpoint == "" || cfg.Bucket == "" || cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" || cfg.PublicURL == "" {
		return nil, errors.New("R2 needs an endpoint, bucket, access key, secret and public URL")
	}
	client := s3.New(s3.Options{
		Region:       "auto",
		BaseEndpoint: aws.String(cfg.Endpoint),
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		UsePathStyle: true,
	})
	return &R2{client: client, presign: s3.NewPresignClient(client), cfg: cfg, TTL: 15 * time.Minute}, nil
}

// PresignUpload signs a PUT for exactly this content type and size.
func (r *R2) PresignUpload(ctx context.Context, key, contentType string, size int64) (Upload, error) {
	req, err := r.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(r.cfg.Bucket),
		Key:           aws.String(key),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
	}, s3.WithPresignExpires(r.TTL))
	if err != nil {
		return Upload{}, fmt.Errorf("presign upload: %w", err)
	}
	return Upload{
		Key:       key,
		URL:       req.URL,
		Method:    http.MethodPut,
		Headers:   map[string]string{"Content-Type": contentType},
		ExpiresAt: time.Now().Add(r.TTL),
		PublicURL: r.URL(ctx, key),
	}, nil
}

// Exists checks the object with a HEAD request.
func (r *R2) Exists(ctx context.Context, key string) (bool, error) {
	_, err := r.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(r.cfg.Bucket), Key: aws.String(key)})
	if err == nil {
		return true, nil
	}
	var resp *smithyhttp.ResponseError
	if errors.As(err, &resp) && resp.HTTPStatusCode() == http.StatusNotFound {
		return false, nil
	}
	return false, fmt.Errorf("check upload: %w", err)
}

// URL is the public address of the object.
func (r *R2) URL(_ context.Context, key string) string {
	return strings.TrimRight(r.cfg.PublicURL, "/") + "/" + key
}
