package service

import (
	"context"
	"errors"
	"testing"

	"github.com/meowmix1337/argus/backend/internal/model"
	apperrors "github.com/meowmix1337/argus/backend/internal/platform/errors"
	"github.com/meowmix1337/argus/backend/internal/platform/publisher"
)

// fakePublisher records published events for assertion.
type fakePublisher struct {
	events []publishedEvent
	err    error
}

type publishedEvent struct {
	topic   string
	payload any
}

func (f *fakePublisher) PublishEvent(topic string, payload any) error {
	f.events = append(f.events, publishedEvent{topic: topic, payload: payload})
	return f.err
}

func (f *fakePublisher) Stop() {}

// fakePostStore is an in-memory PostStore for service-layer tests.
type fakePostStore struct {
	posts     map[string]model.Post
	likes     map[string]map[string]bool // postID -> userID -> active
	createErr error
	getErr    error
	deleteErr error
	likeErr   error
	unlikeErr error
	listErr   error
	searchErr error
}

func newFakePostStore(posts ...model.Post) *fakePostStore {
	s := &fakePostStore{
		posts: make(map[string]model.Post),
		likes: make(map[string]map[string]bool),
	}
	for _, p := range posts {
		s.posts[p.ID] = p
	}
	return s
}

func (f *fakePostStore) Create(_ context.Context, p model.PostCreate) (model.Post, error) {
	if f.createErr != nil {
		return model.Post{}, f.createErr
	}
	post := model.Post{
		ID:           p.ID,
		UserID:       p.UserID,
		UserName:     "testuser",
		Content:      p.Content,
		ParentPostID: p.ParentPostID,
		CreatedAt:    "2025-01-01T00:00:00.000Z",
	}
	f.posts[p.ID] = post
	return post, nil
}

func (f *fakePostStore) GetByID(_ context.Context, postID string) (model.Post, error) {
	if f.getErr != nil {
		return model.Post{}, f.getErr
	}
	p, ok := f.posts[postID]
	if !ok {
		return model.Post{}, apperrors.ErrPostNotFound
	}
	return p, nil
}

func (f *fakePostStore) GetByIDWithLike(_ context.Context, postID, viewerID string) (model.Post, error) {
	if f.getErr != nil {
		return model.Post{}, f.getErr
	}
	p, ok := f.posts[postID]
	if !ok {
		return model.Post{}, apperrors.ErrPostNotFound
	}
	if userLikes, exists := f.likes[postID]; exists {
		p.LikedByMe = userLikes[viewerID]
	}
	return p, nil
}

func (f *fakePostStore) Delete(_ context.Context, postID, userID string) (int64, error) {
	if f.deleteErr != nil {
		return 0, f.deleteErr
	}
	p, ok := f.posts[postID]
	if !ok || p.UserID != userID {
		return 0, nil
	}
	delete(f.posts, postID)
	return 1, nil
}

func (f *fakePostStore) Like(_ context.Context, _, postID, userID string) error {
	if f.likeErr != nil {
		return f.likeErr
	}
	if _, ok := f.likes[postID]; !ok {
		f.likes[postID] = make(map[string]bool)
	}
	f.likes[postID][userID] = true
	return nil
}

func (f *fakePostStore) Unlike(_ context.Context, postID, userID string) (int64, error) {
	if f.unlikeErr != nil {
		return 0, f.unlikeErr
	}
	if userLikes, ok := f.likes[postID]; ok && userLikes[userID] {
		delete(userLikes, userID)
		return 1, nil
	}
	return 0, nil
}

func (f *fakePostStore) ListByUser(_ context.Context, authorID, _ string, limit, offset int) ([]model.Post, int, error) {
	if f.listErr != nil {
		return nil, 0, f.listErr
	}
	var out []model.Post
	for _, p := range f.posts {
		if p.UserID == authorID {
			out = append(out, p)
		}
	}
	total := len(out)
	if offset < total {
		out = out[offset:min(offset+limit, total)]
	} else {
		out = nil
	}
	return out, total, nil
}

func (f *fakePostStore) Search(_ context.Context, _, _ string, limit, offset int) ([]model.Post, int, error) {
	if f.searchErr != nil {
		return nil, 0, f.searchErr
	}
	out := make([]model.Post, 0, len(f.posts))
	for _, p := range f.posts {
		out = append(out, p)
	}
	total := len(out)
	if offset < total {
		out = out[offset:min(offset+limit, total)]
	} else {
		out = nil
	}
	return out, total, nil
}

// ---- Create ----

func TestPostsService_Create_Success(t *testing.T) {
	store := newFakePostStore()
	pub := &fakePublisher{}
	svc := NewPostsService(store, pub)

	post, err := svc.Create(context.Background(), "user1", "Hello world", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if post.Content != "Hello world" {
		t.Errorf("Content = %q, want %q", post.Content, "Hello world")
	}
	if len(pub.events) != 1 {
		t.Fatalf("expected 1 published event, got %d", len(pub.events))
	}
	if pub.events[0].topic != "post.created" {
		t.Errorf("topic = %q, want %q", pub.events[0].topic, "post.created")
	}
}

func TestPostsService_Create_PublishPayloadFields(t *testing.T) {
	store := newFakePostStore()
	pub := &fakePublisher{}
	svc := NewPostsService(store, pub)

	_, err := svc.Create(context.Background(), "user1", "Hello world", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(pub.events) != 1 {
		t.Fatalf("expected 1 published event, got %d", len(pub.events))
	}
	payload, ok := pub.events[0].payload.(publisher.PostCreatedPayload)
	if !ok {
		t.Fatalf("payload type = %T, want publisher.PostCreatedPayload", pub.events[0].payload)
	}
	if payload.AuthorName != "testuser" {
		t.Errorf("AuthorName = %q, want %q", payload.AuthorName, "testuser")
	}
	if payload.ContentPreview != "Hello world" {
		t.Errorf("ContentPreview = %q, want %q", payload.ContentPreview, "Hello world")
	}
}

func TestPostsService_Create_ContentPreviewTruncatedAt100Runes(t *testing.T) {
	store := newFakePostStore()
	pub := &fakePublisher{}
	svc := NewPostsService(store, pub)

	// Build a 110-ASCII-char string: fits within 128-byte limit but exceeds the 100-rune preview cap.
	runes := make([]rune, 110)
	for i := range runes {
		runes[i] = 'a'
	}
	content := string(runes)

	_, err := svc.Create(context.Background(), "user1", content, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	payload, ok := pub.events[0].payload.(publisher.PostCreatedPayload)
	if !ok {
		t.Fatalf("payload type = %T, want publisher.PostCreatedPayload", pub.events[0].payload)
	}
	if len([]rune(payload.ContentPreview)) != 100 {
		t.Errorf("ContentPreview rune length = %d, want 100", len([]rune(payload.ContentPreview)))
	}
}

func TestPostsService_Create_EmptyContent(t *testing.T) {
	svc := NewPostsService(newFakePostStore(), &fakePublisher{})
	_, err := svc.Create(context.Background(), "user1", "", nil)
	if !errors.Is(err, apperrors.ErrPostValidation) {
		t.Errorf("expected ErrPostValidation, got %v", err)
	}
}

func TestPostsService_Create_WhitespaceOnlyContent(t *testing.T) {
	svc := NewPostsService(newFakePostStore(), &fakePublisher{})
	_, err := svc.Create(context.Background(), "user1", "   ", nil)
	if !errors.Is(err, apperrors.ErrPostValidation) {
		t.Errorf("expected ErrPostValidation for whitespace-only, got %v", err)
	}
}

func TestPostsService_Create_ExceedsMaxLength(t *testing.T) {
	svc := NewPostsService(newFakePostStore(), &fakePublisher{})
	long := make([]byte, maxPostLength+1)
	for i := range long {
		long[i] = 'a'
	}
	_, err := svc.Create(context.Background(), "user1", string(long), nil)
	if !errors.Is(err, apperrors.ErrPostValidation) {
		t.Errorf("expected ErrPostValidation for long content, got %v", err)
	}
}

func TestPostsService_Create_SanitizesHTML(t *testing.T) {
	store := newFakePostStore()
	svc := NewPostsService(store, &fakePublisher{})

	post, err := svc.Create(context.Background(), "user1", "<b>bold</b> text", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if post.Content != "bold text" {
		t.Errorf("Content = %q, want %q (HTML should be stripped)", post.Content, "bold text")
	}
}

func TestPostsService_Create_HTMLOnlyBecomesEmpty(t *testing.T) {
	svc := NewPostsService(newFakePostStore(), &fakePublisher{})
	_, err := svc.Create(context.Background(), "user1", "<script>alert('xss')</script>", nil)
	if !errors.Is(err, apperrors.ErrPostValidation) {
		t.Errorf("expected ErrPostValidation for HTML-only content, got %v", err)
	}
}

func TestPostsService_Create_StoreError_Propagates(t *testing.T) {
	store := newFakePostStore()
	store.createErr = errors.New("db failure")
	svc := NewPostsService(store, &fakePublisher{})
	_, err := svc.Create(context.Background(), "user1", "Hello", nil)
	if err == nil {
		t.Error("expected store error to propagate, got nil")
	}
}

func TestPostsService_Create_PublisherError_DoesNotFail(t *testing.T) {
	store := newFakePostStore()
	pub := &fakePublisher{err: errors.New("nsq down")}
	svc := NewPostsService(store, pub)

	_, err := svc.Create(context.Background(), "user1", "Hello", nil)
	if err != nil {
		t.Fatalf("expected publisher error to be swallowed, got %v", err)
	}
}

// ---- GetByID ----

func TestPostsService_GetByID_Success(t *testing.T) {
	store := newFakePostStore(model.Post{ID: "p1", UserID: "user1", Content: "test"})
	svc := NewPostsService(store, &fakePublisher{})

	post, err := svc.GetByID(context.Background(), "p1", "viewer1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if post.ID != "p1" {
		t.Errorf("ID = %q, want %q", post.ID, "p1")
	}
}

func TestPostsService_GetByID_NotFound(t *testing.T) {
	svc := NewPostsService(newFakePostStore(), &fakePublisher{})
	_, err := svc.GetByID(context.Background(), "missing", "viewer1")
	if !errors.Is(err, apperrors.ErrPostNotFound) {
		t.Errorf("expected ErrPostNotFound, got %v", err)
	}
}

func TestPostsService_GetByID_StoreError_Propagates(t *testing.T) {
	store := newFakePostStore(model.Post{ID: "p1"})
	store.getErr = errors.New("db failure")
	svc := NewPostsService(store, &fakePublisher{})
	_, err := svc.GetByID(context.Background(), "p1", "viewer1")
	if err == nil {
		t.Error("expected store error to propagate, got nil")
	}
}

// ---- Delete ----

func TestPostsService_Delete_Success(t *testing.T) {
	store := newFakePostStore(model.Post{ID: "p1", UserID: "user1"})
	svc := NewPostsService(store, &fakePublisher{})

	err := svc.Delete(context.Background(), "p1", "user1")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestPostsService_Delete_NotFound(t *testing.T) {
	svc := NewPostsService(newFakePostStore(), &fakePublisher{})
	err := svc.Delete(context.Background(), "missing", "user1")
	if !errors.Is(err, apperrors.ErrPostNotFound) {
		t.Errorf("expected ErrPostNotFound, got %v", err)
	}
}

func TestPostsService_Delete_WrongOwner(t *testing.T) {
	store := newFakePostStore(model.Post{ID: "p1", UserID: "user1"})
	svc := NewPostsService(store, &fakePublisher{})

	err := svc.Delete(context.Background(), "p1", "user2")
	if !errors.Is(err, apperrors.ErrPostNotFound) {
		t.Errorf("expected ErrPostNotFound for wrong owner, got %v", err)
	}
}

func TestPostsService_Delete_StoreError_Propagates(t *testing.T) {
	store := newFakePostStore(model.Post{ID: "p1", UserID: "user1"})
	store.deleteErr = errors.New("db failure")
	svc := NewPostsService(store, &fakePublisher{})
	err := svc.Delete(context.Background(), "p1", "user1")
	if err == nil {
		t.Error("expected store error to propagate, got nil")
	}
}

// ---- ToggleLike ----

func TestPostsService_ToggleLike_LikesWhenNoExistingLike(t *testing.T) {
	store := newFakePostStore(model.Post{ID: "p1", UserID: "author1"})
	svc := NewPostsService(store, &fakePublisher{})

	post, err := svc.ToggleLike(context.Background(), "p1", "user1")
	if err != nil {
		t.Fatalf("ToggleLike: %v", err)
	}
	if !post.LikedByMe {
		t.Error("expected LikedByMe=true after liking")
	}
}

func TestPostsService_ToggleLike_UnlikesWhenAlreadyLiked(t *testing.T) {
	store := newFakePostStore(model.Post{ID: "p1", UserID: "author1"})
	// Pre-seed like
	store.likes["p1"] = map[string]bool{"user1": true}
	svc := NewPostsService(store, &fakePublisher{})

	post, err := svc.ToggleLike(context.Background(), "p1", "user1")
	if err != nil {
		t.Fatalf("ToggleLike: %v", err)
	}
	if post.LikedByMe {
		t.Error("expected LikedByMe=false after unliking")
	}
}

func TestPostsService_ToggleLike_UnlikeError_Propagates(t *testing.T) {
	store := newFakePostStore(model.Post{ID: "p1"})
	store.unlikeErr = errors.New("db failure")
	svc := NewPostsService(store, &fakePublisher{})
	_, err := svc.ToggleLike(context.Background(), "p1", "user1")
	if err == nil {
		t.Error("expected unlike error to propagate, got nil")
	}
}

func TestPostsService_ToggleLike_LikeError_Propagates(t *testing.T) {
	store := newFakePostStore(model.Post{ID: "p1"})
	store.likeErr = errors.New("db failure")
	svc := NewPostsService(store, &fakePublisher{})
	_, err := svc.ToggleLike(context.Background(), "p1", "user1")
	if err == nil {
		t.Error("expected like error to propagate, got nil")
	}
}

func TestPostsService_ToggleLike_GetAfterToggleError_Propagates(t *testing.T) {
	store := newFakePostStore() // empty store — GetByIDWithLike will fail after unlike returns 0
	svc := NewPostsService(store, &fakePublisher{})
	// Unlike returns 0 rows, Like succeeds, but GetByIDWithLike fails
	// because post doesn't exist in store (simulating a race)
	_, err := svc.ToggleLike(context.Background(), "missing", "user1")
	if err == nil {
		t.Error("expected error from get-after-toggle, got nil")
	}
}

// ---- ListByUser ----

func TestPostsService_ListByUser_Success(t *testing.T) {
	store := newFakePostStore(
		model.Post{ID: "p1", UserID: "author1", Content: "post1"},
		model.Post{ID: "p2", UserID: "author1", Content: "post2"},
		model.Post{ID: "p3", UserID: "author2", Content: "other"},
	)
	svc := NewPostsService(store, &fakePublisher{})

	posts, total, err := svc.ListByUser(context.Background(), "author1", "viewer1", 10, 0)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(posts) != 2 {
		t.Errorf("len(posts) = %d, want 2", len(posts))
	}
}

func TestPostsService_ListByUser_StoreError_Propagates(t *testing.T) {
	store := newFakePostStore()
	store.listErr = errors.New("db failure")
	svc := NewPostsService(store, &fakePublisher{})
	_, _, err := svc.ListByUser(context.Background(), "author1", "viewer1", 10, 0)
	if err == nil {
		t.Error("expected store error to propagate, got nil")
	}
}

// ---- Search ----

func TestPostsService_Search_Success(t *testing.T) {
	store := newFakePostStore(model.Post{ID: "p1", UserID: "u1", Content: "hello"})
	svc := NewPostsService(store, &fakePublisher{})

	posts, total, err := svc.Search(context.Background(), "hello", "viewer1", 10, 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(posts) != 1 {
		t.Errorf("len(posts) = %d, want 1", len(posts))
	}
}

func TestPostsService_Search_EmptyQuery(t *testing.T) {
	svc := NewPostsService(newFakePostStore(), &fakePublisher{})
	_, _, err := svc.Search(context.Background(), "", "viewer1", 10, 0)
	if !errors.Is(err, apperrors.ErrPostValidation) {
		t.Errorf("expected ErrPostValidation for empty query, got %v", err)
	}
}

func TestPostsService_Search_WhitespaceQuery(t *testing.T) {
	svc := NewPostsService(newFakePostStore(), &fakePublisher{})
	_, _, err := svc.Search(context.Background(), "   ", "viewer1", 10, 0)
	if !errors.Is(err, apperrors.ErrPostValidation) {
		t.Errorf("expected ErrPostValidation for whitespace query, got %v", err)
	}
}

func TestPostsService_Search_StoreError_Propagates(t *testing.T) {
	store := newFakePostStore()
	store.searchErr = errors.New("db failure")
	svc := NewPostsService(store, &fakePublisher{})
	_, _, err := svc.Search(context.Background(), "hello", "viewer1", 10, 0)
	if err == nil {
		t.Error("expected store error to propagate, got nil")
	}
}
