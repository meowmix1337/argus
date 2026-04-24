package service

import (
	"context"
	"fmt"

	"github.com/meowmix1337/argus/backend/internal/model"
)

// fakeIntegrationStore implements IntegrationStore for tests.
type fakeIntegrationStore struct {
	integration model.UserIntegration
	getErr      error
	deleteN     int64
	deleteErr   error
}

func (f *fakeIntegrationStore) Create(_ context.Context, i model.IntegrationCreate) (model.UserIntegration, error) {
	return model.UserIntegration{ID: i.ID, UserID: i.UserID}, nil
}

func (f *fakeIntegrationStore) GetByUserAndProvider(_ context.Context, _, _ string) (model.UserIntegration, error) {
	return f.integration, f.getErr
}

func (f *fakeIntegrationStore) GetByID(_ context.Context, _, _ string) (model.UserIntegration, error) {
	return f.integration, f.getErr
}

func (f *fakeIntegrationStore) Delete(_ context.Context, _, _ string) (int64, error) {
	return f.deleteN, f.deleteErr
}

// fakeWatchedRepoStore implements WatchedRepoStore for tests.
type fakeWatchedRepoStore struct {
	repos        []model.WatchedRepo
	listErr      error
	listByIntErr error
}

func (f *fakeWatchedRepoStore) Create(_ context.Context, w model.WatchedRepoCreate) (model.WatchedRepo, error) {
	return model.WatchedRepo{ID: w.ID}, nil
}

func (f *fakeWatchedRepoStore) GetByID(_ context.Context, _, _ string) (model.WatchedRepo, error) {
	return model.WatchedRepo{}, fmt.Errorf("not found")
}

func (f *fakeWatchedRepoStore) ListByIntegration(_ context.Context, _, _ string) ([]model.WatchedRepo, error) {
	if f.listByIntErr != nil {
		return nil, f.listByIntErr
	}
	return f.repos, f.listErr
}

func (f *fakeWatchedRepoStore) GetByOwnerRepo(_ context.Context, _, _ string) ([]model.WatchedRepo, error) {
	return f.repos, nil
}

func (f *fakeWatchedRepoStore) Delete(_ context.Context, _, _ string) (int64, error) {
	return 1, nil
}
