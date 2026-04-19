package service

import (
	"encoding/hex"
	"testing"

	platformcrypto "github.com/meowmix1337/argus/backend/internal/platform/crypto"
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
