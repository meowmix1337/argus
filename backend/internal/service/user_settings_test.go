package service

import (
	"context"
	"errors"
	"testing"

	apperrors "github.com/meowmix1337/argus/backend/internal/errors"
	"github.com/meowmix1337/argus/backend/internal/model"
)

// fakeUserSettingsStore is an in-memory UserSettingsStore for service tests.
type fakeUserSettingsStore struct {
	settings       *model.UserSettings
	getErr         error
	upsertErr      error
	categories     []model.NewsCategoryType
	selectedCats   []model.NewsCategoryType
	setCatsErr     error
	listAllCatsErr error
	listSelCatsErr error
}

func (f *fakeUserSettingsStore) Get(_ context.Context, _ string) (model.UserSettings, error) {
	if f.getErr != nil {
		return model.UserSettings{}, f.getErr
	}
	if f.settings == nil {
		return model.UserSettings{}, apperrors.ErrSettingsNotFound
	}
	return *f.settings, nil
}

func (f *fakeUserSettingsStore) Upsert(_ context.Context, _ string, u model.UserSettingsUpsert) (model.UserSettings, error) {
	if f.upsertErr != nil {
		return model.UserSettings{}, f.upsertErr
	}
	s := model.UserSettings{
		CalendarICSURL: u.CalendarICSURL,
		Timezone:       u.Timezone,
	}
	f.settings = &s
	return s, nil
}

func (f *fakeUserSettingsStore) ListAllCategories(_ context.Context) ([]model.NewsCategoryType, error) {
	if f.listAllCatsErr != nil {
		return nil, f.listAllCatsErr
	}
	return f.categories, nil
}

func (f *fakeUserSettingsStore) ListSelectedCategories(_ context.Context, _ string) ([]model.NewsCategoryType, error) {
	if f.listSelCatsErr != nil {
		return nil, f.listSelCatsErr
	}
	return f.selectedCats, nil
}

func (f *fakeUserSettingsStore) SetSelectedCategories(_ context.Context, _ string, _ []string) error {
	return f.setCatsErr
}

// ---- validateCalendarURL ----

func TestValidateCalendarURL_HTTPS(t *testing.T) {
	if err := validateCalendarURL("https://example.com/calendar.ics"); err != nil {
		t.Errorf("expected valid HTTPS URL, got: %v", err)
	}
}

func TestValidateCalendarURL_HTTP(t *testing.T) {
	if err := validateCalendarURL("http://example.com/calendar.ics"); err != nil {
		t.Errorf("expected valid HTTP URL, got: %v", err)
	}
}

// TestValidateCalendarURL_FileScheme_Rejected guards against SSRF via file:// URLs.
func TestValidateCalendarURL_FileScheme_Rejected(t *testing.T) {
	if err := validateCalendarURL("file:///etc/passwd"); err == nil {
		t.Error("expected error for file:// URL (SSRF risk)")
	}
}

func TestValidateCalendarURL_FTPScheme_Rejected(t *testing.T) {
	if err := validateCalendarURL("ftp://files.example.com/cal.ics"); err == nil {
		t.Error("expected error for ftp:// scheme")
	}
}

// TestValidateCalendarURL_NoScheme_Rejected prevents bare hostnames from bypassing
// the scheme check (url.Parse treats them as path-only, scheme becomes "").
func TestValidateCalendarURL_NoScheme_Rejected(t *testing.T) {
	if err := validateCalendarURL("example.com/calendar.ics"); err == nil {
		t.Error("expected error for URL with no scheme")
	}
}

// ---- UserSettingsService ----

func TestUserSettingsService_Get_NotFound_ReturnsNil(t *testing.T) {
	// settings==nil in the store → ErrSettingsNotFound → service returns (nil, nil).
	svc := NewUserSettingsService(&fakeUserSettingsStore{}, nil)
	result, err := svc.Get(context.Background(), "user1")
	if err != nil {
		t.Fatalf("expected nil error when settings not found, got: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result when settings not found, got: %+v", result)
	}
}

// TestUserSettingsService_Upsert_EncPrefix_Rejected prevents users from injecting
// a pre-encrypted value directly, which would bypass the encryption round-trip.
func TestUserSettingsService_Upsert_EncPrefix_Rejected(t *testing.T) {
	svc := NewUserSettingsService(&fakeUserSettingsStore{}, nil)
	encURL := "enc:someciphertext"
	u := model.UserSettingsUpsert{CalendarICSURL: &encURL}
	_, err := svc.Upsert(context.Background(), "user1", u)
	if !errors.Is(err, ErrSettingsValidation) {
		t.Errorf("expected ErrSettingsValidation for 'enc:' prefix, got %v", err)
	}
}

func TestUserSettingsService_Upsert_InvalidCalendarScheme(t *testing.T) {
	svc := NewUserSettingsService(&fakeUserSettingsStore{}, nil)
	badURL := "ftp://calendar.example.com/feed.ics"
	u := model.UserSettingsUpsert{CalendarICSURL: &badURL}
	_, err := svc.Upsert(context.Background(), "user1", u)
	if !errors.Is(err, ErrSettingsValidation) {
		t.Errorf("expected ErrSettingsValidation for ftp:// scheme, got %v", err)
	}
}

// TestUserSettingsService_Upsert_EncryptsAndDecryptsCalendarURL verifies the full
// encrypt-store-decrypt round-trip: the caller sees the plaintext URL, never the ciphertext.
func TestUserSettingsService_Upsert_EncryptsAndDecryptsCalendarURL(t *testing.T) {
	enc := mustNewEncryptionService(t)
	svc := NewUserSettingsService(&fakeUserSettingsStore{}, enc)

	calURL := "https://calendar.example.com/feed.ics"
	u := model.UserSettingsUpsert{CalendarICSURL: &calURL}
	settings, err := svc.Upsert(context.Background(), "user1", u)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if settings.CalendarICSURL == nil {
		t.Fatal("expected CalendarICSURL to be set after upsert")
	}
	if *settings.CalendarICSURL != calURL {
		t.Errorf("CalendarICSURL = %q, want %q (must be decrypted in the response)", *settings.CalendarICSURL, calURL)
	}
}

// ---- SetSelectedCategories ----

func TestUserSettingsService_SetSelectedCategories_InvalidCategory(t *testing.T) {
	store := &fakeUserSettingsStore{
		categories: []model.NewsCategoryType{
			{ID: "technology"}, {ID: "sports"},
		},
	}
	svc := NewUserSettingsService(store, nil)
	err := svc.SetSelectedCategories(context.Background(), "user1", []string{"technology", "politics"})
	if !errors.Is(err, ErrCategoryNotFound) {
		t.Errorf("expected ErrCategoryNotFound for unknown category 'politics', got %v", err)
	}
}

func TestUserSettingsService_SetSelectedCategories_ValidCategories(t *testing.T) {
	store := &fakeUserSettingsStore{
		categories: []model.NewsCategoryType{
			{ID: "technology"}, {ID: "sports"},
		},
	}
	svc := NewUserSettingsService(store, nil)
	if err := svc.SetSelectedCategories(context.Background(), "user1", []string{"technology", "sports"}); err != nil {
		t.Fatalf("SetSelectedCategories: %v", err)
	}
}

// ---- ListAllCategories ----

func TestUserSettingsService_ListAllCategories_ReturnsAll(t *testing.T) {
	store := &fakeUserSettingsStore{
		categories: []model.NewsCategoryType{
			{ID: "technology"}, {ID: "sports"}, {ID: "health"},
		},
	}
	svc := NewUserSettingsService(store, nil)
	cats, err := svc.ListAllCategories(context.Background())
	if err != nil {
		t.Fatalf("ListAllCategories: %v", err)
	}
	if len(cats) != 3 {
		t.Errorf("expected 3 categories, got %d", len(cats))
	}
}

// ---- Get ----

func TestUserSettingsService_Get_Success(t *testing.T) {
	calURL := "https://example.com/cal.ics"
	store := &fakeUserSettingsStore{settings: &model.UserSettings{CalendarICSURL: &calURL}}
	svc := NewUserSettingsService(store, nil)
	result, err := svc.Get(context.Background(), "user1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil settings")
	}
}

func TestUserSettingsService_Get_GenericError_Propagates(t *testing.T) {
	store := &fakeUserSettingsStore{getErr: errors.New("db failure")}
	svc := NewUserSettingsService(store, nil)
	if _, err := svc.Get(context.Background(), "user1"); err == nil {
		t.Error("expected error from generic store failure, got nil")
	}
}

func TestUserSettingsService_ListAllCategories_Empty(t *testing.T) {
	svc := NewUserSettingsService(&fakeUserSettingsStore{}, nil)
	cats, err := svc.ListAllCategories(context.Background())
	if err != nil {
		t.Fatalf("ListAllCategories: %v", err)
	}
	if len(cats) != 0 {
		t.Errorf("expected 0 categories, got %d", len(cats))
	}
}

// ---- ListSelectedCategories ----

func TestUserSettingsService_ListSelectedCategories_ReturnsSelected(t *testing.T) {
	store := &fakeUserSettingsStore{
		selectedCats: []model.NewsCategoryType{{ID: "technology"}, {ID: "health"}},
	}
	svc := NewUserSettingsService(store, nil)
	cats, err := svc.ListSelectedCategories(context.Background(), "user1")
	if err != nil {
		t.Fatalf("ListSelectedCategories: %v", err)
	}
	if len(cats) != 2 {
		t.Errorf("expected 2 selected categories, got %d", len(cats))
	}
}

func TestUserSettingsService_SetSelectedCategories_EmptyListSucceeds(t *testing.T) {
	// Clearing all selected categories is a valid operation.
	store := &fakeUserSettingsStore{
		categories: []model.NewsCategoryType{{ID: "technology"}},
	}
	svc := NewUserSettingsService(store, nil)
	if err := svc.SetSelectedCategories(context.Background(), "user1", []string{}); err != nil {
		t.Fatalf("SetSelectedCategories with empty list: %v", err)
	}
}

func TestUserSettingsService_SetSelectedCategories_StoreError_Propagates(t *testing.T) {
	store := &fakeUserSettingsStore{
		categories: []model.NewsCategoryType{{ID: "technology"}},
		setCatsErr: errors.New("db failure"),
	}
	svc := NewUserSettingsService(store, nil)
	if err := svc.SetSelectedCategories(context.Background(), "user1", []string{"technology"}); err == nil {
		t.Error("expected store error to propagate, got nil")
	}
}

func TestUserSettingsService_SetSelectedCategories_ListAllCatsError_Propagates(t *testing.T) {
	store := &fakeUserSettingsStore{listAllCatsErr: errors.New("db failure")}
	svc := NewUserSettingsService(store, nil)
	if err := svc.SetSelectedCategories(context.Background(), "user1", []string{"technology"}); err == nil {
		t.Error("expected listAllCategories error to propagate, got nil")
	}
}

func TestUserSettingsService_ListAllCategories_StoreError_Propagates(t *testing.T) {
	store := &fakeUserSettingsStore{listAllCatsErr: errors.New("db failure")}
	svc := NewUserSettingsService(store, nil)
	if _, err := svc.ListAllCategories(context.Background()); err == nil {
		t.Error("expected store error to propagate, got nil")
	}
}

func TestUserSettingsService_ListSelectedCategories_StoreError_Propagates(t *testing.T) {
	store := &fakeUserSettingsStore{listSelCatsErr: errors.New("db failure")}
	svc := NewUserSettingsService(store, nil)
	if _, err := svc.ListSelectedCategories(context.Background(), "user1"); err == nil {
		t.Error("expected store error to propagate, got nil")
	}
}

func TestUserSettingsService_Upsert_StoreError_Propagates(t *testing.T) {
	store := &fakeUserSettingsStore{upsertErr: errors.New("db failure")}
	svc := NewUserSettingsService(store, nil)
	if _, err := svc.Upsert(context.Background(), "user1", model.UserSettingsUpsert{}); err == nil {
		t.Error("expected upsert store error to propagate, got nil")
	}
}

// TestUserSettingsService_Upsert_WithTimezone covers the timezone-trimming branch
// that fires when u.Timezone is non-nil.
func TestUserSettingsService_Upsert_WithTimezone(t *testing.T) {
	store := &fakeUserSettingsStore{}
	svc := NewUserSettingsService(store, nil)
	tz := "  America/New_York  "
	s, err := svc.Upsert(context.Background(), "user1", model.UserSettingsUpsert{Timezone: &tz})
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if s.Timezone == nil || *s.Timezone != "America/New_York" {
		t.Errorf("Timezone = %v, want %q (should be trimmed)", s.Timezone, "America/New_York")
	}
}

// TestUserSettingsService_Get_DecryptError_Propagates ensures that a bad ciphertext
// stored in the DB causes Get to return an error rather than silently returning garbage.
func TestUserSettingsService_Get_DecryptError_Propagates(t *testing.T) {
	enc := mustNewEncryptionService(t)
	badURL := "enc:notvalidbase64!!!"
	store := &fakeUserSettingsStore{settings: &model.UserSettings{CalendarICSURL: &badURL}}
	svc := NewUserSettingsService(store, enc)
	if _, err := svc.Get(context.Background(), "user1"); err == nil {
		t.Error("expected decrypt error to propagate, got nil")
	}
}
