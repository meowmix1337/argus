-- +goose Up

CREATE TABLE IF NOT EXISTS posts (
    id             CHAR(36) NOT NULL PRIMARY KEY CHECK(length(id) = 36),  -- UUID v7, app-generated
    user_id        CHAR(36) NOT NULL REFERENCES users(id),
    content        TEXT NOT NULL CHECK(length(content) > 0 AND length(content) <= 128),
    parent_post_id CHAR(36) REFERENCES posts(id),                          -- NULL = top-level post
    like_count     INTEGER NOT NULL DEFAULT 0 CHECK(like_count >= 0),
    media_urls     TEXT,                                                    -- JSON array of URLs, nullable
    created_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    deleted_at     TEXT
);

-- For listing a user's own posts (profile page, follower-gated at query time).
CREATE INDEX IF NOT EXISTS idx_posts_user_active
    ON posts (user_id, created_at DESC) WHERE deleted_at IS NULL;

-- For the chronological feed query (follower JOIN applied at query time).
CREATE INDEX IF NOT EXISTS idx_posts_created_at
    ON posts (created_at DESC) WHERE deleted_at IS NULL;

-- For fetching replies to a parent post.
CREATE INDEX IF NOT EXISTS idx_posts_parent_active
    ON posts (parent_post_id, created_at DESC)
    WHERE parent_post_id IS NOT NULL AND deleted_at IS NULL;

-- +goose StatementBegin
CREATE TRIGGER posts_updated_at
    AFTER UPDATE ON posts
    FOR EACH ROW
    WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE posts
       SET updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
     WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER IF EXISTS posts_updated_at;
DROP INDEX  IF EXISTS idx_posts_parent_active;
DROP INDEX  IF EXISTS idx_posts_created_at;
DROP INDEX  IF EXISTS idx_posts_user_active;
DROP TABLE  IF EXISTS posts;
