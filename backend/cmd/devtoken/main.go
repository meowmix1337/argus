// devtoken is a dev-only CLI tool that mints a signed session cookie for local testing.
// Usage: SESSION_SECRET=<hex> go run ./cmd/devtoken
// NOT intended for production use.
package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/meowmix1337/argus/backend/internal/platform/config"
	"github.com/meowmix1337/argus/backend/internal/platform/session"
)

func main() {
	cfg := config.Load()

	secret, err := hex.DecodeString(cfg.SessionSecret)
	if err != nil || len(secret) == 0 {
		fmt.Fprintln(os.Stderr, "SESSION_SECRET missing or invalid hex in .env")
		os.Exit(1)
	}

	data := session.Data{
		UserID:    "dev-test-user-00000000",
		Email:     "dev@test.local",
		Name:      "Dev User",
		ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
	}

	token, err := session.Encode(secret, data)
	if err != nil {
		fmt.Fprintln(os.Stderr, "encode error:", err)
		os.Exit(1)
	}
	fmt.Println(token)
}
