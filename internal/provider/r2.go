package provider

import (
	"context"
	"fmt"
)

type R2Provider struct{}

func NewR2Provider() *R2Provider {
	return &R2Provider{}
}

func (p *R2Provider) Name() string {
	return "r2"
}

func (p *R2Provider) Upload(_ context.Context, _ UploadInput) (UploadResult, error) {
	return UploadResult{}, fmt.Errorf("r2 provider not enabled in MVP")
}

func (p *R2Provider) Delete(_ context.Context, _ string, _ *string) error {
	return nil
}

func (p *R2Provider) GetAccess(_ context.Context, _ string, _ *string) (AccessResult, error) {
	return AccessResult{}, fmt.Errorf("r2 provider not enabled in MVP")
}
