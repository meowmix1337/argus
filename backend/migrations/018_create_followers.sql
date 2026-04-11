-- +goose Up

-- Follow relationships; soft-deleted on unfollow (re-follow reuses the row).
CREATE TABLE IF NOT EXISTS followers (
    id            CHAR(36) NOT NULL PRIMARY KEY CHECK(length(id) = 36),  -- UUID v7, app-generated
    follower_id   CHAR(36) NOT NULL REFERENCES users(id),  -- the user who follows
    following_id  CHAR(36) NOT NULL REFERENCES users(id),  -- the user being followed
    created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    deleted_at    TEXT,
    CHECK(follower_id != following_id)
);

-- One active follow relationship per pair.
CREATE UNIQUE INDEX IF NOT EXISTS uq_followers_follower_following_active
    ON followers (follower_id, following_id) WHERE deleted_at IS NULL;

-- For "who follows user X" (followers list).
CREATE INDEX IF NOT EXISTS idx_followers_following_id
    ON followers (following_id) WHERE deleted_at IS NULL;

-- For "who does user X follow" (following list) and feed fan-out.
CREATE INDEX IF NOT EXISTS idx_followers_follower_id
    ON followers (follower_id) WHERE deleted_at IS NULL;

-- +goose StatementBegin
CREATE TRIGGER followers_updated_at
    AFTER UPDATE ON followers
    FOR EACH ROW
    WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE followers
       SET updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
     WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER IF EXISTS followers_updated_at;
DROP INDEX  IF EXISTS idx_followers_follower_id;
DROP INDEX  IF EXISTS idx_followers_following_id;
DROP INDEX  IF EXISTS uq_followers_follower_following_active;
DROP TABLE  IF EXISTS followers;
