package service

import (
	"context"
	"errors"
	"testing"

	"github.com/meowmix1337/argus/backend/internal/model"
	apperrors "github.com/meowmix1337/argus/backend/internal/platform/errors"
)

// fakeFollowStore is an in-memory FollowStore for service-layer tests.
type fakeFollowStore struct {
	follows      map[string]map[string]bool // followerID -> followingID -> active
	followErr    error
	unfollowErr  error
	isFollowErr  error
	followersErr error
	followingErr error
}

func newFakeFollowStore() *fakeFollowStore {
	return &fakeFollowStore{
		follows: make(map[string]map[string]bool),
	}
}

func (f *fakeFollowStore) addFollow(followerID, followingID string) {
	if _, ok := f.follows[followerID]; !ok {
		f.follows[followerID] = make(map[string]bool)
	}
	f.follows[followerID][followingID] = true
}

func (f *fakeFollowStore) Follow(_ context.Context, _, followerID, followingID string) error {
	if f.followErr != nil {
		return f.followErr
	}
	if _, ok := f.follows[followerID]; !ok {
		f.follows[followerID] = make(map[string]bool)
	}
	if f.follows[followerID][followingID] {
		return apperrors.ErrAlreadyFollowing
	}
	f.follows[followerID][followingID] = true
	return nil
}

func (f *fakeFollowStore) Unfollow(_ context.Context, followerID, followingID string) (int64, error) {
	if f.unfollowErr != nil {
		return 0, f.unfollowErr
	}
	if m, ok := f.follows[followerID]; ok && m[followingID] {
		delete(m, followingID)
		return 1, nil
	}
	return 0, nil
}

func (f *fakeFollowStore) IsFollowing(_ context.Context, followerID, followingID string) (bool, error) {
	if f.isFollowErr != nil {
		return false, f.isFollowErr
	}
	if m, ok := f.follows[followerID]; ok {
		return m[followingID], nil
	}
	return false, nil
}

func (f *fakeFollowStore) ListFollowers(_ context.Context, _ string, _, _ int) ([]model.UserSummary, int, error) {
	if f.followersErr != nil {
		return nil, 0, f.followersErr
	}
	return []model.UserSummary{}, 0, nil
}

func (f *fakeFollowStore) ListFollowing(_ context.Context, _ string, _, _ int) ([]model.UserSummary, int, error) {
	if f.followingErr != nil {
		return nil, 0, f.followingErr
	}
	return []model.UserSummary{}, 0, nil
}

func (f *fakeFollowStore) GetFollowerIDs(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}

// ---- Follow ----

func TestFollowService_Follow_Success(t *testing.T) {
	store := newFakeFollowStore()
	pub := &fakePublisher{}
	svc := NewFollowService(store, pub)

	err := svc.Follow(context.Background(), "user1", "user2", "User One")
	if err != nil {
		t.Fatalf("Follow: %v", err)
	}
	if !store.follows["user1"]["user2"] {
		t.Error("expected follow relationship to be stored")
	}
	if len(pub.events) != 1 {
		t.Fatalf("expected 1 published event, got %d", len(pub.events))
	}
	if pub.events[0].topic != "user.followed" {
		t.Errorf("expected user.followed event, got %q", pub.events[0].topic)
	}
}

func TestFollowService_Follow_SelfFollow(t *testing.T) {
	svc := NewFollowService(newFakeFollowStore(), &fakePublisher{})
	err := svc.Follow(context.Background(), "user1", "user1", "User One")
	if !errors.Is(err, apperrors.ErrSelfFollow) {
		t.Errorf("expected ErrSelfFollow, got %v", err)
	}
}

func TestFollowService_Follow_AlreadyFollowing(t *testing.T) {
	store := newFakeFollowStore()
	store.addFollow("user1", "user2")
	svc := NewFollowService(store, &fakePublisher{})

	err := svc.Follow(context.Background(), "user1", "user2", "User One")
	if !errors.Is(err, apperrors.ErrAlreadyFollowing) {
		t.Errorf("expected ErrAlreadyFollowing, got %v", err)
	}
}

func TestFollowService_Follow_IsFollowingError_Propagates(t *testing.T) {
	store := newFakeFollowStore()
	store.isFollowErr = errors.New("db failure")
	svc := NewFollowService(store, &fakePublisher{})
	err := svc.Follow(context.Background(), "user1", "user2", "User One")
	if err == nil {
		t.Error("expected IsFollowing error to propagate, got nil")
	}
}

func TestFollowService_Follow_StoreError_Propagates(t *testing.T) {
	store := newFakeFollowStore()
	store.followErr = errors.New("db failure")
	svc := NewFollowService(store, &fakePublisher{})
	err := svc.Follow(context.Background(), "user1", "user2", "User One")
	if err == nil {
		t.Error("expected store Follow error to propagate, got nil")
	}
}

func TestFollowService_Follow_PublisherError_DoesNotFail(t *testing.T) {
	store := newFakeFollowStore()
	pub := &fakePublisher{err: errors.New("nsq down")}
	svc := NewFollowService(store, pub)

	err := svc.Follow(context.Background(), "user1", "user2", "User One")
	if err != nil {
		t.Fatalf("expected publisher error to be swallowed, got %v", err)
	}
}

func TestFollowService_Follow_StoreRaceAlreadyFollowing(t *testing.T) {
	store := newFakeFollowStore()
	// IsFollowing returns false, but Follow returns ErrAlreadyFollowing (race condition).
	store.followErr = apperrors.ErrAlreadyFollowing
	svc := NewFollowService(store, &fakePublisher{})

	err := svc.Follow(context.Background(), "user1", "user2", "User One")
	if !errors.Is(err, apperrors.ErrAlreadyFollowing) {
		t.Errorf("expected ErrAlreadyFollowing from store race, got %v", err)
	}
}

// ---- Unfollow ----

func TestFollowService_Unfollow_Success(t *testing.T) {
	store := newFakeFollowStore()
	store.addFollow("user1", "user2")
	svc := NewFollowService(store, &fakePublisher{})

	err := svc.Unfollow(context.Background(), "user1", "user2")
	if err != nil {
		t.Fatalf("Unfollow: %v", err)
	}
}

func TestFollowService_Unfollow_NotFollowing(t *testing.T) {
	svc := NewFollowService(newFakeFollowStore(), &fakePublisher{})
	err := svc.Unfollow(context.Background(), "user1", "user2")
	if !errors.Is(err, apperrors.ErrNotFollowing) {
		t.Errorf("expected ErrNotFollowing, got %v", err)
	}
}

func TestFollowService_Unfollow_StoreError_Propagates(t *testing.T) {
	store := newFakeFollowStore()
	store.unfollowErr = errors.New("db failure")
	svc := NewFollowService(store, &fakePublisher{})
	err := svc.Unfollow(context.Background(), "user1", "user2")
	if err == nil {
		t.Error("expected store error to propagate, got nil")
	}
}

// ---- IsFollowing ----

func TestFollowService_IsFollowing_True(t *testing.T) {
	store := newFakeFollowStore()
	store.addFollow("user1", "user2")
	svc := NewFollowService(store, &fakePublisher{})

	following, err := svc.IsFollowing(context.Background(), "user1", "user2")
	if err != nil {
		t.Fatalf("IsFollowing: %v", err)
	}
	if !following {
		t.Error("expected following=true")
	}
}

func TestFollowService_IsFollowing_False(t *testing.T) {
	svc := NewFollowService(newFakeFollowStore(), &fakePublisher{})
	following, err := svc.IsFollowing(context.Background(), "user1", "user2")
	if err != nil {
		t.Fatalf("IsFollowing: %v", err)
	}
	if following {
		t.Error("expected following=false")
	}
}

// ---- ListFollowers / ListFollowing ----

func TestFollowService_ListFollowers_StoreError_Propagates(t *testing.T) {
	store := newFakeFollowStore()
	store.followersErr = errors.New("db failure")
	svc := NewFollowService(store, &fakePublisher{})
	_, _, err := svc.ListFollowers(context.Background(), "user1", 10, 0)
	if err == nil {
		t.Error("expected store error to propagate, got nil")
	}
}

func TestFollowService_ListFollowing_StoreError_Propagates(t *testing.T) {
	store := newFakeFollowStore()
	store.followingErr = errors.New("db failure")
	svc := NewFollowService(store, &fakePublisher{})
	_, _, err := svc.ListFollowing(context.Background(), "user1", 10, 0)
	if err == nil {
		t.Error("expected store error to propagate, got nil")
	}
}
