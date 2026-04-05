package service

import (
	"strings"
	"testing"
)

func TestProvideEncryptionService_EmptyKey(t *testing.T) {
	_, err := ProvideEncryptionService("")
	if err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestProvideEncryptionService_InvalidHex(t *testing.T) {
	_, err := ProvideEncryptionService("not-valid-hex!!")
	if err == nil {
		t.Fatal("expected error for non-hex input")
	}
}

func TestProvideEncryptionService_WrongKeyLength(t *testing.T) {
	// 16 bytes = 32 hex chars — too short for AES-256 (requires 32 bytes)
	_, err := ProvideEncryptionService("deadbeefdeadbeefdeadbeefdeadbeef")
	if err == nil {
		t.Fatal("expected error for 16-byte key (AES-256 requires 32 bytes)")
	}
}

func TestProvideEncryptionService_Valid(t *testing.T) {
	key := strings.Repeat("ab", 32) // 64 hex chars = 32 bytes
	svc, err := ProvideEncryptionService(key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

// TestEncryptDecrypt_RoundTrip is the core correctness test: whatever is
// encrypted must decrypt back to the original plaintext.
func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	svc := mustNewEncryptionService(t)
	plaintext := "https://calendar.example.com/feed.ics?token=secret"

	ciphertext, err := svc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if !strings.HasPrefix(ciphertext, "enc:") {
		t.Errorf("ciphertext should start with 'enc:', got: %q", ciphertext)
	}

	got, err := svc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if got != plaintext {
		t.Errorf("Decrypt = %q, want %q", got, plaintext)
	}
}

// TestEncrypt_ProducesUniqueNonces ensures AES-GCM random nonces are working:
// two encryptions of the same plaintext must produce different ciphertexts.
func TestEncrypt_ProducesUniqueNonces(t *testing.T) {
	svc := mustNewEncryptionService(t)

	c1, err := svc.Encrypt("same plaintext")
	if err != nil {
		t.Fatalf("first Encrypt: %v", err)
	}
	c2, err := svc.Encrypt("same plaintext")
	if err != nil {
		t.Fatalf("second Encrypt: %v", err)
	}
	if c1 == c2 {
		t.Error("two encryptions of the same plaintext produced identical ciphertexts — nonces are not random")
	}
}

// TestDecrypt_PlaintextPassthrough verifies backward-compatibility: values that
// don't have the "enc:" prefix are returned unchanged (legacy unencrypted data).
func TestDecrypt_PlaintextPassthrough(t *testing.T) {
	svc := mustNewEncryptionService(t)
	raw := "https://example.com/calendar.ics"

	got, err := svc.Decrypt(raw)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if got != raw {
		t.Errorf("Decrypt = %q, want %q (should pass through unencrypted values)", got, raw)
	}
}

// TestDecrypt_TamperedCiphertext ensures authentication is enforced: modifying
// the ciphertext must cause decryption to fail, not silently return garbage.
func TestDecrypt_TamperedCiphertext(t *testing.T) {
	svc := mustNewEncryptionService(t)
	// Valid base64 but not a valid AES-GCM ciphertext produced by this key.
	_, err := svc.Decrypt("enc:dGhpcyBpcyBub3QgYSB2YWxpZCBjaXBoZXJ0ZXh0")
	if err == nil {
		t.Error("expected error when decrypting tampered ciphertext")
	}
}

// TestDecrypt_EmptyEncPayload ensures we handle "enc:" with no payload gracefully.
func TestDecrypt_EmptyEncPayload(t *testing.T) {
	svc := mustNewEncryptionService(t)
	_, err := svc.Decrypt("enc:")
	if err == nil {
		t.Error("expected error for 'enc:' with no payload")
	}
}

// TestNewEncryptionService_InvalidKeySize_ReturnsError verifies that passing a key
// that is not a valid AES key size (16, 24, or 32 bytes) returns an error.
func TestNewEncryptionService_InvalidKeySize_ReturnsError(t *testing.T) {
	_, err := NewEncryptionService([]byte("tooshort")) // 8 bytes — invalid for AES
	if err == nil {
		t.Fatal("expected error for 8-byte key (AES requires 16, 24, or 32 bytes)")
	}
}

// mustNewEncryptionService creates a test EncryptionService with a fixed 32-byte key.
// Never use an all-zero key in production.
func mustNewEncryptionService(t *testing.T) *EncryptionService {
	t.Helper()
	key := make([]byte, 32) // all zeros — test-only
	svc, err := NewEncryptionService(key)
	if err != nil {
		t.Fatalf("NewEncryptionService: %v", err)
	}
	return svc
}
