package provider

import (
	"context"
	"io"
	"net/http"
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

type AccessResult struct {
	URL     string
	Header  http.Header
}

type StorageProvider interface {
	Name() string
	Upload(ctx context.Context, in UploadInput) (UploadResult, error)
	Delete(ctx context.Context, providerFileID string, bucketOrChat *string) error
	GetAccess(ctx context.Context, providerFileID string, bucketOrChat *string) (AccessResult, error)
}
