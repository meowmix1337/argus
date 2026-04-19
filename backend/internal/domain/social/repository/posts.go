package repository

import (
	"context"

	"github.com/meowmix1337/argus/backend/internal/model"
)

// PostStore defines the data-access contract for posts.
type PostStore interface {
	Create(ctx context.Context, p model.PostCreate) (model.Post, error)
	GetByID(ctx context.Context, postID string) (model.Post, error)
	GetByIDWithLike(ctx context.Context, postID, viewerID string) (model.Post, error)
	Delete(ctx context.Context, postID, userID string) (int64, error)
	Like(ctx context.Context, id, postID, userID string) error
	Unlike(ctx context.Context, postID, userID string) (int64, error)
	ListByUser(ctx context.Context, authorID, viewerID string, limit, offset int) ([]model.Post, int, error)
	Search(ctx context.Context, query, viewerID string, limit, offset int) ([]model.Post, int, error)
	ListPostIDsByAuthor(ctx context.Context, authorID string, limit int) ([]model.PostRef, error)
}
