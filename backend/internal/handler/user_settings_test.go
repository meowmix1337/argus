package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-playground/validator/v10"

	apperrors "github.com/meowmix1337/argus/backend/internal/errors"
	"github.com/meowmix1337/argus/backend/internal/model"
	"github.com/meowmix1337/argus/backend/internal/service"
)

// fakeUserSettingsStore is an in-memory UserSettingsStore for handler tests.
type fakeUserSettingsStore struct {
	settings               *model.UserSettings // nil means return ErrSettingsNotFound
	upsertResult           model.UserSettings
	allCategories          []model.NewsCategoryType
	selectedCategories     []model.NewsCategoryType
	err                    error
	listAllCategoriesErr   error
	listSelectedCategories error
}

func (f *fakeUserSettingsStore) Get(_ context.Context, _ string) (model.UserSettings, error) {
	if f.err != nil {
		return model.UserSettings{}, f.err
	}
	if f.settings == nil {
		return model.UserSettings{}, apperrors.ErrSettingsNotFound
	}
	return *f.settings, nil
}

func (f *fakeUserSettingsStore) Upsert(_ context.Context, _ string, _ model.UserSettingsUpsert) (model.UserSettings, error) {
	if f.err != nil {
		return model.UserSettings{}, f.err
	}
	return f.upsertResult, nil
}

func (f *fakeUserSettingsStore) ListAllCategories(_ context.Context) ([]model.NewsCategoryType, error) {
	if f.listAllCategoriesErr != nil {
		return nil, f.listAllCategoriesErr
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.allCategories, nil
}

func (f *fakeUserSettingsStore) ListSelectedCategories(_ context.Context, _ string) ([]model.NewsCategoryType, error) {
	if f.listSelectedCategories != nil {
		return nil, f.listSelectedCategories
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.selectedCategories, nil
}

func (f *fakeUserSettingsStore) SetSelectedCategories(_ context.Context, _ string, _ []string) error {
	return nil
}

func newTestUserSettingsHandler(store service.UserSettingsStore) *UserSettingsHandler {
	svc := service.NewUserSettingsService(store, nil)
	return NewUserSettingsHandler(svc, validator.New())
}

func TestGetUserSettings(t *testing.T) {
	lat := 37.77
	lon := -122.42

	tests := []struct {
		name       string
		settings   *model.UserSettings
		storeErr   error
		noSession  bool
		wantStatus int
	}{
		{
			name:       "no session returns 401",
			noSession:  true,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "settings not found returns 200 with empty response",
			settings:   nil,
			wantStatus: http.StatusOK,
		},
		{
			name:       "settings with lat/lon returns 200",
			settings:   &model.UserSettings{Latitude: &lat, Longitude: &lon},
			wantStatus: http.StatusOK,
		},
		{
			name:       "service error returns 500",
			storeErr:   fmt.Errorf("db error"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeUserSettingsStore{settings: tt.settings, err: tt.storeErr}
			h := newTestUserSettingsHandler(store)

			req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
			if !tt.noSession {
				req = withSession(req, "user1")
			}
			w := httptest.NewRecorder()
			h.Get(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if tt.wantStatus != http.StatusOK {
				return
			}

			var resp UserSettingsResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if tt.settings == nil {
				if resp.Latitude != nil || resp.Longitude != nil {
					t.Errorf("expected empty settings response, got lat=%v lon=%v", resp.Latitude, resp.Longitude)
				}
				return
			}

			if resp.Latitude == nil || *resp.Latitude != lat {
				t.Errorf("latitude = %v, want %v", resp.Latitude, lat)
			}
			if resp.Longitude == nil || *resp.Longitude != lon {
				t.Errorf("longitude = %v, want %v", resp.Longitude, lon)
			}
		})
	}
}

func TestUpsertSettings(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		noSession  bool
		storeErr   error
		wantStatus int
	}{
		{
			name:       "no session returns 401",
			noSession:  true,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "empty body returns 400",
			body:       "",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "latitude out of range returns 400",
			body:       `{"latitude":200}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "valid lat/lon returns 200",
			body:       `{"latitude":37.77,"longitude":-122.42}`,
			wantStatus: http.StatusOK,
		},
		{
			name:       "service error returns 500",
			body:       `{"latitude":37.77,"longitude":-122.42}`,
			storeErr:   fmt.Errorf("db error"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeUserSettingsStore{err: tt.storeErr}
			h := newTestUserSettingsHandler(store)

			req := httptest.NewRequest(http.MethodPut, "/api/settings", bytes.NewReader([]byte(tt.body)))
			req.Header.Set("Content-Type", "application/json")
			if !tt.noSession {
				req = withSession(req, "user1")
			}
			w := httptest.NewRecorder()
			h.Upsert(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: body=%s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func TestGetNewsCategories(t *testing.T) {
	tests := []struct {
		name                   string
		noSession              bool
		listAllCategoriesErr   error
		listSelectedCategories error
		wantStatus             int
	}{
		{
			name:       "no session returns 401",
			noSession:  true,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "returns 200 with available and selected",
			wantStatus: http.StatusOK,
		},
		{
			name:                 "ListAllCategories error returns 500",
			listAllCategoriesErr: fmt.Errorf("db error"),
			wantStatus:           http.StatusInternalServerError,
		},
		{
			name:                   "ListSelectedCategories error returns 500",
			listSelectedCategories: fmt.Errorf("db error"),
			wantStatus:             http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeUserSettingsStore{
				allCategories:          []model.NewsCategoryType{{ID: "general", Label: "General", SortOrder: 1}},
				selectedCategories:     []model.NewsCategoryType{{ID: "general", Label: "General", SortOrder: 1}},
				listAllCategoriesErr:   tt.listAllCategoriesErr,
				listSelectedCategories: tt.listSelectedCategories,
			}
			h := newTestUserSettingsHandler(store)

			req := httptest.NewRequest(http.MethodGet, "/api/settings/news-categories", nil)
			if !tt.noSession {
				req = withSession(req, "user1")
			}
			w := httptest.NewRecorder()
			h.GetNewsCategories(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if tt.wantStatus != http.StatusOK {
				return
			}

			var resp NewsCategoriesResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if len(resp.Available) == 0 {
				t.Errorf("expected non-empty available categories")
			}
		})
	}
}

func TestSetNewsCategories(t *testing.T) {
	tests := []struct {
		name          string
		body          string
		allCategories []model.NewsCategoryType
		noSession     bool
		wantStatus    int
	}{
		{
			name:       "no session returns 401",
			noSession:  true,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "empty body returns 400",
			body:       "",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty category_ids array returns 400",
			body:       `{"category_ids":[]}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:          "invalid category returns 400",
			body:          `{"category_ids":["nonexistent"]}`,
			allCategories: []model.NewsCategoryType{},
			wantStatus:    http.StatusBadRequest,
		},
		{
			name:          "valid category returns 204",
			body:          `{"category_ids":["general"]}`,
			allCategories: []model.NewsCategoryType{{ID: "general", Label: "General", SortOrder: 1}},
			wantStatus:    http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeUserSettingsStore{allCategories: tt.allCategories}
			h := newTestUserSettingsHandler(store)

			req := httptest.NewRequest(http.MethodPut, "/api/settings/news-categories", bytes.NewReader([]byte(tt.body)))
			req.Header.Set("Content-Type", "application/json")
			if !tt.noSession {
				req = withSession(req, "user1")
			}
			w := httptest.NewRecorder()
			h.SetNewsCategories(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: body=%s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}
