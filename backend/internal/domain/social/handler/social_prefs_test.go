package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/meowmix1337/argus/backend/internal/model"
	"github.com/meowmix1337/argus/backend/internal/platform/validate"
)

type fakeSocialPrefsService struct {
	prefs      model.SocialNotificationPrefs
	getErr     error
	upsertErr  error
	upsertedID string
}

func (f *fakeSocialPrefsService) GetPrefs(_ context.Context, userID string) (model.SocialNotificationPrefs, error) {
	if f.getErr != nil {
		return model.SocialNotificationPrefs{}, f.getErr
	}
	if f.prefs.UserID == "" {
		return model.SocialNotificationPrefs{UserID: userID}, nil
	}
	return f.prefs, nil
}

func (f *fakeSocialPrefsService) UpsertPrefs(_ context.Context, userID string, _, _ bool) error {
	f.upsertedID = userID
	return f.upsertErr
}

func TestSocialPrefsHandler_GetPrefs_DefaultsWhenNoRow(t *testing.T) {
	h := NewSocialPrefsHandler(&fakeSocialPrefsService{}, validate.New())
	r := chi.NewRouter()
	h.AddRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/social-notifications", nil)
	req = withSession(req, "user-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp SocialPrefsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.MutePosts || resp.MuteFollows {
		t.Errorf("expected default false prefs, got %+v", resp)
	}
}

func TestSocialPrefsHandler_GetPrefs_ReturnsStored(t *testing.T) {
	stored := model.SocialNotificationPrefs{UserID: "user-1", MutePosts: true, MuteFollows: false}
	h := NewSocialPrefsHandler(&fakeSocialPrefsService{prefs: stored}, validate.New())
	r := chi.NewRouter()
	h.AddRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/social-notifications", nil)
	req = withSession(req, "user-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp SocialPrefsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.MutePosts {
		t.Error("expected MutePosts=true in response")
	}
}

func TestSocialPrefsHandler_GetPrefs_Unauthorized(t *testing.T) {
	h := NewSocialPrefsHandler(&fakeSocialPrefsService{}, validate.New())
	r := chi.NewRouter()
	h.AddRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/social-notifications", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestSocialPrefsHandler_UpdatePrefs_Success(t *testing.T) {
	svc := &fakeSocialPrefsService{}
	h := NewSocialPrefsHandler(svc, validate.New())
	r := chi.NewRouter()
	h.AddRoutes(r)

	body, _ := json.Marshal(UpdateSocialPrefsRequest{MutePosts: true, MuteFollows: false})
	req := httptest.NewRequest(http.MethodPut, "/api/settings/social-notifications", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withSession(req, "user-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
	if svc.upsertedID != "user-1" {
		t.Errorf("upserted userID = %q, want user-1", svc.upsertedID)
	}
}

func TestSocialPrefsHandler_UpdatePrefs_InvalidJSON(t *testing.T) {
	h := NewSocialPrefsHandler(&fakeSocialPrefsService{}, validate.New())
	r := chi.NewRouter()
	h.AddRoutes(r)

	req := httptest.NewRequest(http.MethodPut, "/api/settings/social-notifications", bytes.NewReader([]byte(`not-json`)))
	req = withSession(req, "user-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestSocialPrefsHandler_UpdatePrefs_Unauthorized(t *testing.T) {
	h := NewSocialPrefsHandler(&fakeSocialPrefsService{}, validate.New())
	r := chi.NewRouter()
	h.AddRoutes(r)

	body, _ := json.Marshal(UpdateSocialPrefsRequest{})
	req := httptest.NewRequest(http.MethodPut, "/api/settings/social-notifications", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}
