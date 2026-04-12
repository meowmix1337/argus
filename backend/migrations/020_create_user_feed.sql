-- +goose Up

CREATE TABLE IF NOT EXISTS user_feed (
    id         CHAR(36) NOT NULL PRIMARY KEY CHECK(length(id) = 36),
    user_id    CHAR(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    post_id    CHAR(36) NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

-- Cursor pagination: primary access pattern
CREATE INDEX IF NOT EXISTS idx_user_feed_pagination
    ON user_feed(user_id, created_at DESC, id DESC);

-- Deduplication: INSERT OR IGNORE relies on this
CREATE UNIQUE INDEX IF NOT EXISTS uq_user_feed_user_post
    ON user_feed(user_id, post_id);

-- +goose StatementBegin
CREATE TRIGGER user_feed_updated_at
    AFTER UPDATE ON user_feed
    FOR EACH ROW
    WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE user_feed SET updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- +goose Down

DROP TRIGGER IF EXISTS user_feed_updated_at;
DROP INDEX IF EXISTS uq_user_feed_user_post;
DROP INDEX IF EXISTS idx_user_feed_pagination;
DROP TABLE IF EXISTS user_feed;
