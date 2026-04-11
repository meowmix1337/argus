-- +goose Up

-- Like records; soft-deleted on unlike so the toggle is auditable.
-- posts.like_count is kept in sync by triggers below.
CREATE TABLE IF NOT EXISTS post_likes (
    id         CHAR(36) NOT NULL PRIMARY KEY CHECK(length(id) = 36),  -- UUID v7, app-generated
    post_id    CHAR(36) NOT NULL REFERENCES posts(id),
    user_id    CHAR(36) NOT NULL REFERENCES users(id),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    deleted_at TEXT
);

-- One active like per user per post.
CREATE UNIQUE INDEX IF NOT EXISTS uq_post_likes_post_user_active
    ON post_likes (post_id, user_id) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_post_likes_user_active
    ON post_likes (user_id) WHERE deleted_at IS NULL;

-- +goose StatementBegin
CREATE TRIGGER post_likes_updated_at
    AFTER UPDATE ON post_likes
    FOR EACH ROW
    WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE post_likes
       SET updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
     WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- Increment posts.like_count when a new active like is inserted.
-- +goose StatementBegin
CREATE TRIGGER post_likes_increment
    AFTER INSERT ON post_likes
    FOR EACH ROW
    WHEN NEW.deleted_at IS NULL
BEGIN
    UPDATE posts SET like_count = like_count + 1 WHERE id = NEW.post_id;
END;
-- +goose StatementEnd

-- Keep posts.like_count in sync when a like is toggled (soft-delete on/off).
-- WHEN guard skips the trigger when deleted_at didn't actually change value.
-- MAX(0, ...) guards against going negative if data is ever inconsistent.
-- +goose StatementBegin
CREATE TRIGGER post_likes_toggle
    AFTER UPDATE OF deleted_at ON post_likes
    FOR EACH ROW
    WHEN (OLD.deleted_at IS NULL) != (NEW.deleted_at IS NULL)
BEGIN
    UPDATE posts
       SET like_count = MAX(0, like_count + CASE
               WHEN OLD.deleted_at IS NOT NULL AND NEW.deleted_at IS NULL     THEN  1   -- re-like
               WHEN OLD.deleted_at IS NULL     AND NEW.deleted_at IS NOT NULL THEN -1   -- unlike
               ELSE 0
           END)
     WHERE id = NEW.post_id;
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER IF EXISTS post_likes_toggle;
DROP TRIGGER IF EXISTS post_likes_increment;
DROP TRIGGER IF EXISTS post_likes_updated_at;
DROP INDEX  IF EXISTS idx_post_likes_user_active;
DROP INDEX  IF EXISTS uq_post_likes_post_user_active;
DROP TABLE  IF EXISTS post_likes;
