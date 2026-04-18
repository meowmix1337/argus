package session

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const CookieName = "session"

type Data struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url,omitempty"`
	ExpiresAt int64  `json:"exp"` // Unix timestamp; token is invalid after this time
}

// Encode serialises d as JSON, base64url-encodes it, appends an HMAC-SHA256 sig.
// Format: <base64payload>.<hex_sig>
func Encode(secret []byte, d Data) (string, error) {
	payload, err := json.Marshal(d)
	if err != nil {
		return "", err
	}
	b64 := base64.RawURLEncoding.EncodeToString(payload)
	return b64 + "." + sign(secret, b64), nil
}

// Decode verifies the HMAC, deserialises the session data, and rejects expired tokens.
func Decode(secret []byte, value string) (Data, error) {
	idx := strings.LastIndex(value, ".")
	if idx < 0 {
		return Data{}, errors.New("invalid session cookie format")
	}
	b64, gotSig := value[:idx], value[idx+1:]
	if !hmac.Equal([]byte(sign(secret, b64)), []byte(gotSig)) {
		return Data{}, errors.New("invalid session cookie signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(b64)
	if err != nil {
		return Data{}, err
	}
	var d Data
	if err := json.Unmarshal(payload, &d); err != nil {
		return Data{}, err
	}
	if d.ExpiresAt > 0 && time.Now().Unix() > d.ExpiresAt {
		return Data{}, errors.New("session expired")
	}
	return d, nil
}

func sign(secret []byte, data string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}
