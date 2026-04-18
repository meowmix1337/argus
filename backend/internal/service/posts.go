package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/microcosm-cc/bluemonday"

	"github.com/meowmix1337/argus/backend/internal/model"
	apperrors "github.com/meowmix1337/argus/backend/internal/platform/errors"
	platformevents "github.com/meowmix1337/argus/backend/internal/platform/events"
)

const (
	maxPostLength          = 128
	contentPreviewMaxRunes = 100
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
}

// PostsService manages social feed posts.
type PostsService struct {
	store     PostStore
	publisher platformevents.Publisher
	sanitizer *bluemonday.Policy
}

// NewPostsService creates a new PostsService.
func NewPostsService(store PostStore, publisher platformevents.Publisher) *PostsService {
	return &PostsService{
		store:     store,
		publisher: publisher,
		sanitizer: bluemonday.StrictPolicy(),
	}
}

// Create creates a new post after sanitizing content.
func (s *PostsService) Create(ctx context.Context, userID, content string, parentPostID *string) (model.Post, error) {
	content = strings.TrimSpace(s.sanitizer.Sanitize(content))
	if content == "" {
		return model.Post{}, fmt.Errorf("%w: content cannot be empty", apperrors.ErrPostValidation)
	}
	if len(content) > maxPostLength {
		return model.Post{}, fmt.Errorf("%w: content exceeds %d characters", apperrors.ErrPostValidation, maxPostLength)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return model.Post{}, fmt.Errorf("generate post id: %w", err)
	}

	post, err := s.store.Create(ctx, model.PostCreate{
		ID:           id.String(),
		UserID:       userID,
		Content:      content,
		ParentPostID: parentPostID,
	})
	if err != nil {
		return model.Post{}, fmt.Errorf("create post: %w", err)
	}

	// Truncate content preview to 100 runes (UTF-8 safe).
	preview := []rune(content)
	if len(preview) > contentPreviewMaxRunes {
		preview = preview[:contentPreviewMaxRunes]
	}

	if pubErr := s.publisher.PublishEvent(platformevents.TopicPostCreated, platformevents.PostCreatedPayload{
		PostID:         post.ID,
		UserID:         post.UserID,
		AuthorName:     post.UserName,
		ContentPreview: string(preview),
	}); pubErr != nil {
		slog.Error("failed to publish post.created event", "error", pubErr, "post_id", post.ID)
	}

	return post, nil
}

// GetByID returns a single post by ID (no follower check — accessible to any authenticated user).
func (s *PostsService) GetByID(ctx context.Context, postID, viewerID string) (model.Post, error) {
	post, err := s.store.GetByIDWithLike(ctx, postID, viewerID)
	if err != nil {
		if errors.Is(err, apperrors.ErrPostNotFound) {
			return model.Post{}, apperrors.ErrPostNotFound
		}
		return model.Post{}, fmt.Errorf("get post: %w", err)
	}
	return post, nil
}

// Delete soft-deletes a post (owner only).
func (s *PostsService) Delete(ctx context.Context, postID, userID string) error {
	rows, err := s.store.Delete(ctx, postID, userID)
	if err != nil {
		return fmt.Errorf("delete post: %w", err)
	}
	if rows == 0 {
		return apperrors.ErrPostNotFound
	}
	return nil
}

// ToggleLike likes or unlikes a post. Returns the updated post.
func (s *PostsService) ToggleLike(ctx context.Context, postID, userID string) (model.Post, error) {
	// Try to unlike first (idempotent toggle).
	rows, err := s.store.Unlike(ctx, postID, userID)
	if err != nil {
		return model.Post{}, fmt.Errorf("unlike post: %w", err)
	}

	if rows == 0 {
		// No active like existed — create one.
		id, err := uuid.NewV7()
		if err != nil {
			return model.Post{}, fmt.Errorf("generate like id: %w", err)
		}
		if err := s.store.Like(ctx, id.String(), postID, userID); err != nil {
			return model.Post{}, fmt.Errorf("like post: %w", err)
		}
	}

	post, err := s.store.GetByIDWithLike(ctx, postID, userID)
	if err != nil {
		return model.Post{}, fmt.Errorf("get post after toggle: %w", err)
	}

	return post, nil
}

// ListByUser returns posts by a specific author, visible only if viewer follows the author
// or is the author themselves.
func (s *PostsService) ListByUser(ctx context.Context, authorID, viewerID string, limit, offset int) ([]model.Post, int, error) {
	posts, total, err := s.store.ListByUser(ctx, authorID, viewerID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list posts by user: %w", err)
	}
	return posts, total, nil
}

// Search returns posts matching the FTS5 query, filtered to authors the viewer follows or self.
func (s *PostsService) Search(ctx context.Context, query, viewerID string, limit, offset int) ([]model.Post, int, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, 0, fmt.Errorf("%w: search query cannot be empty", apperrors.ErrPostValidation)
	}
	posts, total, err := s.store.Search(ctx, query, viewerID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("search posts: %w", err)
	}
	return posts, total, nil
}
