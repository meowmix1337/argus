package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/meowmix1337/argus/backend/internal/model"
	platformcrypto "github.com/meowmix1337/argus/backend/internal/platform/crypto"
	apperrors "github.com/meowmix1337/argus/backend/internal/platform/errors"
)

// ErrSettingsNotFound is returned when user settings do not exist.
var ErrSettingsNotFound = apperrors.ErrSettingsNotFound

// ErrSettingsValidation is returned when settings input fails validation.
var ErrSettingsValidation = apperrors.ErrSettingsValidation

// ErrCategoryNotFound is returned when an invalid news category ID is provided.
var ErrCategoryNotFound = apperrors.ErrCategoryNotFound

// UserSettingsStore defines the data-access contract for user settings.
type UserSettingsStore interface {
	Get(ctx context.Context, userID string) (model.UserSettings, error)
	Upsert(ctx context.Context, userID string, u model.UserSettingsUpsert) (model.UserSettings, error)
	ListAllCategories(ctx context.Context) ([]model.NewsCategoryType, error)
	ListSelectedCategories(ctx context.Context, userID string) ([]model.NewsCategoryType, error)
	SetSelectedCategories(ctx context.Context, userID string, categoryIDs []string) error
}

// UserSettingsService manages user settings via a store.
type UserSettingsService struct {
	store UserSettingsStore
	enc   *platformcrypto.EncryptionService
}

// NewUserSettingsService creates a new UserSettingsService backed by the given store.
func NewUserSettingsService(store UserSettingsStore, enc *platformcrypto.EncryptionService) *UserSettingsService {
	return &UserSettingsService{store: store, enc: enc}
}

// Get returns the settings for the given user, or nil if no settings have been configured yet.
func (s *UserSettingsService) Get(ctx context.Context, userID string) (*model.UserSettings, error) {
	settings, err := s.store.Get(ctx, userID)
	if err != nil {
		if errors.Is(err, apperrors.ErrSettingsNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get user settings: %w", err)
	}
	if err := s.decryptSensitiveFields(&settings); err != nil {
		return nil, err
	}
	return &settings, nil
}

// Upsert creates or updates settings for the given user, returning the final state.
func (s *UserSettingsService) Upsert(ctx context.Context, userID string, u model.UserSettingsUpsert) (model.UserSettings, error) {
	if u.Timezone != nil {
		trimmed := strings.TrimSpace(*u.Timezone)
		u.Timezone = &trimmed
	}
	if u.CalendarICSURL != nil {
		trimmed := strings.TrimSpace(*u.CalendarICSURL)
		u.CalendarICSURL = &trimmed
		if strings.HasPrefix(trimmed, "enc:") {
			return model.UserSettings{}, fmt.Errorf("%w: calendar URL must not start with \"enc:\"", ErrSettingsValidation)
		}
		if trimmed != "" {
			if err := validateCalendarURL(trimmed); err != nil {
				return model.UserSettings{}, err
			}
		}
		if s.enc != nil && trimmed != "" {
			encrypted, encErr := s.enc.Encrypt(trimmed)
			if encErr != nil {
				return model.UserSettings{}, fmt.Errorf("encrypt calendar URL: %w", encErr)
			}
			u.CalendarICSURL = &encrypted
		}
	}

	settings, err := s.store.Upsert(ctx, userID, u)
	if err != nil {
		return model.UserSettings{}, fmt.Errorf("upsert user settings: %w", err)
	}
	if err := s.decryptSensitiveFields(&settings); err != nil {
		return model.UserSettings{}, err
	}
	return settings, nil
}

// ListAllCategories returns all available news category types.
func (s *UserSettingsService) ListAllCategories(ctx context.Context) ([]model.NewsCategoryType, error) {
	categories, err := s.store.ListAllCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("list all news categories: %w", err)
	}
	return categories, nil
}

// ListSelectedCategories returns the news categories the user has selected.
func (s *UserSettingsService) ListSelectedCategories(ctx context.Context, userID string) ([]model.NewsCategoryType, error) {
	categories, err := s.store.ListSelectedCategories(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list selected news categories: %w", err)
	}
	return categories, nil
}

// SetSelectedCategories replaces the user's selected news categories.
// Returns ErrCategoryNotFound if any provided category ID does not exist.
func (s *UserSettingsService) SetSelectedCategories(ctx context.Context, userID string, categoryIDs []string) error {
	// Validate that all provided category IDs exist.
	all, err := s.store.ListAllCategories(ctx)
	if err != nil {
		return fmt.Errorf("validate categories: %w", err)
	}
	valid := make(map[string]struct{}, len(all))
	for _, c := range all {
		valid[c.ID] = struct{}{}
	}
	for _, id := range categoryIDs {
		if _, ok := valid[id]; !ok {
			return fmt.Errorf("%w: %q", ErrCategoryNotFound, id)
		}
	}

	if err := s.store.SetSelectedCategories(ctx, userID, categoryIDs); err != nil {
		return fmt.Errorf("set selected categories: %w", err)
	}
	return nil
}

// validateCalendarURL rejects URLs with schemes other than http/https to prevent SSRF.
func validateCalendarURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%w: invalid calendar URL", ErrSettingsValidation)
	}
	switch u.Scheme {
	case "https", "http":
		return nil
	default:
		return fmt.Errorf("%w: calendar URL must use https:// or http:// scheme", ErrSettingsValidation)
	}
}

func (s *UserSettingsService) decryptSensitiveFields(m *model.UserSettings) error {
	if s.enc != nil && m.CalendarICSURL != nil && *m.CalendarICSURL != "" {
		decrypted, err := s.enc.Decrypt(*m.CalendarICSURL)
		if err != nil {
			return fmt.Errorf("decrypt calendar URL: %w", err)
		}
		m.CalendarICSURL = &decrypted
	}
	return nil
}
