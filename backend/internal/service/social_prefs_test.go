package service

import (
	"context"
	"errors"
	"testing"

	"github.com/meowmix1337/argus/backend/internal/model"
)

type fakeSocialPrefsStore struct {
	prefs     model.SocialNotificationPrefs
	getErr    error
	upsertErr error
	upserted  *model.SocialNotificationPrefs
}

func (f *fakeSocialPrefsStore) GetPrefs(_ context.Context, userID string) (model.SocialNotificationPrefs, error) {
	if f.getErr != nil {
		return model.SocialNotificationPrefs{}, f.getErr
	}
	if f.prefs.UserID == "" {
		return model.SocialNotificationPrefs{UserID: userID}, nil // default
	}
	return f.prefs, nil
}

func (f *fakeSocialPrefsStore) UpsertPrefs(_ context.Context, prefs model.SocialNotificationPrefs) error {
	if f.upsertErr != nil {
		return f.upsertErr
	}
	f.upserted = &prefs
	return nil
}

func TestSocialPrefsService_GetPrefs_DefaultsWhenNoRow(t *testing.T) {
	svc := NewSocialPrefsService(&fakeSocialPrefsStore{})
	prefs, err := svc.GetPrefs(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("GetPrefs: %v", err)
	}
	if prefs.MutePosts || prefs.MuteFollows {
		t.Errorf("expected default false prefs, got mutePosts=%v muteFollows=%v", prefs.MutePosts, prefs.MuteFollows)
	}
}

func TestSocialPrefsService_GetPrefs_ReturnsStored(t *testing.T) {
	stored := model.SocialNotificationPrefs{UserID: "user-1", MutePosts: true, MuteFollows: false}
	svc := NewSocialPrefsService(&fakeSocialPrefsStore{prefs: stored})
	prefs, err := svc.GetPrefs(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("GetPrefs: %v", err)
	}
	if !prefs.MutePosts {
		t.Error("expected MutePosts=true")
	}
}

func TestSocialPrefsService_GetPrefs_StoreError_Propagates(t *testing.T) {
	svc := NewSocialPrefsService(&fakeSocialPrefsStore{getErr: errors.New("db error")})
	_, err := svc.GetPrefs(context.Background(), "user-1")
	if err == nil {
		t.Error("expected error to propagate")
	}
}

func TestSocialPrefsService_UpsertPrefs_Success(t *testing.T) {
	store := &fakeSocialPrefsStore{}
	svc := NewSocialPrefsService(store)
	err := svc.UpsertPrefs(context.Background(), "user-1", true, false)
	if err != nil {
		t.Fatalf("UpsertPrefs: %v", err)
	}
	if store.upserted == nil {
		t.Fatal("expected prefs to be upserted")
	}
	if !store.upserted.MutePosts {
		t.Error("expected MutePosts=true in upserted prefs")
	}
	if store.upserted.UserID != "user-1" {
		t.Errorf("UserID = %q, want user-1", store.upserted.UserID)
	}
}

func TestSocialPrefsService_UpsertPrefs_StoreError_Propagates(t *testing.T) {
	svc := NewSocialPrefsService(&fakeSocialPrefsStore{upsertErr: errors.New("db error")})
	err := svc.UpsertPrefs(context.Background(), "user-1", false, true)
	if err == nil {
		t.Error("expected error to propagate")
	}
}
