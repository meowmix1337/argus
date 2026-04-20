package service

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"

	platformcrypto "github.com/meowmix1337/argus/backend/internal/platform/crypto"
	"github.com/meowmix1337/argus/backend/internal/platform/httpclient"
)

// mustNewEncryptionService creates a test EncryptionService with a fixed 32-byte key.
func mustNewEncryptionService(t *testing.T) *platformcrypto.EncryptionService {
	t.Helper()
	key, _ := hex.DecodeString("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	svc, err := platformcrypto.NewEncryptionService(key)
	if err != nil {
		t.Fatalf("NewEncryptionService: %v", err)
	}
	return svc
}

// fakeHTTPClient is a minimal HTTPClient that marshals responseBody into the result parameter.
// Set rawBytes to return raw bytes from GetBytes (e.g. ICS content for calendar tests).
type fakeHTTPClient struct {
	responseBody any
	rawBytes     []byte
	err          error
}

func (f *fakeHTTPClient) Get(_ context.Context, _ string, result any, _ ...httpclient.RequestOption) error {
	if f.err != nil {
		return f.err
	}
	b, err := json.Marshal(f.responseBody)
	if err != nil {
		return fmt.Errorf("fakeHTTPClient: marshal: %w", err)
	}
	return json.Unmarshal(b, result)
}

func (f *fakeHTTPClient) Post(_ context.Context, _ string, _ any, _ any, _ ...httpclient.RequestOption) error {
	return f.err
}

func (f *fakeHTTPClient) Put(_ context.Context, _ string, _ any, _ any, _ ...httpclient.RequestOption) error {
	return f.err
}

func (f *fakeHTTPClient) Delete(_ context.Context, _ string, _ any, _ ...httpclient.RequestOption) error {
	return f.err
}

func (f *fakeHTTPClient) GetBytes(_ context.Context, _ string, _ ...httpclient.RequestOption) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.rawBytes != nil {
		return f.rawBytes, nil
	}
	return json.Marshal(f.responseBody)
}
