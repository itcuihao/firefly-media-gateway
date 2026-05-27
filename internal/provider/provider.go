package provider

import (
	"context"
	"io"
)

type UploadInput struct {
	FileName string
	MIMEType string
	Reader   io.Reader
}

type UploadResult struct {
	ProviderFileID       string
	ProviderBucketOrChat *string
}

type StorageProvider interface {
	Name() string
	Upload(ctx context.Context, in UploadInput) (UploadResult, error)
	Delete(ctx context.Context, providerFileID string, bucketOrChat *string) error
	GetAccessURL(ctx context.Context, providerFileID string, bucketOrChat *string) (string, error)
}
