package provider

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestLiveWorkerProvider(t *testing.T) {
	baseURL := "https://firefly-media-gateway.itcuihao.workers.dev"
	authToken := "chfirefly"

	p := NewWorkerProvider(baseURL, authToken)

	t.Run("Test Connection Verify Endpoint", func(t *testing.T) {
		ctx := context.Background()
		res, err := p.GetAccess(ctx, "test_file_id_not_found", nil)
		if err != nil {
			// If it returned 401/403, auth failed.
			if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "403") {
				t.Fatalf("Authentication failed on Cloudflare Worker: %v", err)
			}
			t.Logf("Authentication OK (returned expected non-auth error: %v)", err)
		} else {
			t.Logf("GetAccess success: %+v", res)
		}
	})

	t.Run("Test Upload Endpoint", func(t *testing.T) {
		ctx := context.Background()
		testContent := "hello firefly gateway worker integration test content"
		in := UploadInput{
			FileName: "test_integration_upload.png",
			MIMEType: "image/png",
			Reader:   bytes.NewReader([]byte(testContent)),
		}

		res, err := p.Upload(ctx, in)
		if err != nil {
			t.Fatalf("Upload to Cloudflare Worker failed: %v", err)
		}

		t.Logf("Upload Success! ProviderFileID: %s", res.ProviderFileID)
		if res.ProviderBucketOrChat != nil {
			t.Logf("Group/Chat ID: %s", *res.ProviderBucketOrChat)
		}

		// Retrieve Access
		access, err := p.GetAccess(ctx, res.ProviderFileID, res.ProviderBucketOrChat)
		if err != nil {
			t.Fatalf("GetAccess failed: %v", err)
		}
		t.Logf("Retrieve Stream URL Success: %s", access.URL)
	})
}
