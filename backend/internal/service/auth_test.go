package service

import (
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

// ---- NewAuthService ----

func TestNewAuthService_Constructor(t *testing.T) {
	// db can be nil — the constructor just stores it, no nil check.
	svc := NewAuthService(nil, "google-client-id", "google-client-secret", "http://localhost/callback")
	if svc == nil {
		t.Fatal("expected non-nil AuthService")
	}
}

// ---- AuthService.AuthCodeURL ----

func TestAuthService_AuthCodeURL_ContainsClientID(t *testing.T) {
	svc := &AuthService{
		oauthCfg: &oauth2.Config{
			ClientID:    "my-google-client-id",
			RedirectURL: "http://localhost/callback",
		},
	}
	url := svc.AuthCodeURL("csrf-state-token")
	if url == "" {
		t.Fatal("expected non-empty OAuth URL")
	}
	if !strings.Contains(url, "my-google-client-id") {
		t.Errorf("URL does not contain client_id: %q", url)
	}
}
