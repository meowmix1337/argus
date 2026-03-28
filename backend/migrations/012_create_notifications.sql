-- +goose Up
CREATE TABLE IF NOT EXISTS notifications (
    id                   CHAR(36) NOT NULL PRIMARY KEY CHECK(length(id) = 36),
    user_id              CHAR(36) NOT NULL REFERENCES users(id),
    provider             TEXT NOT NULL CHECK(provider IN ('github')),  -- 'github', future: 'slack', 'email'
    event_type           TEXT NOT NULL CHECK(event_type IN ('pr_opened', 'pr_merged', 'pr_closed', 'pr_comment', 'pr_review_comment')),
    title                TEXT NOT NULL,
    body                 TEXT,
    url                  TEXT,                    -- deep link to GitHub PR/comment
    read_at              TEXT,                    -- NULL = unread
    dismissed_at         TEXT,                    -- NULL = not dismissed
    github_delivery_id   TEXT,                    -- X-GitHub-Delivery header; uniqueness enforced via partial index below
    created_at           TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at           TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    deleted_at           TEXT
);

-- Partial unique index on delivery ID: prevents duplicate notifications from GitHub webhook retries.
-- Nullable: non-NULL values must be unique; multiple NULLs are allowed (non-GitHub providers).
CREATE UNIQUE INDEX IF NOT EXISTS uq_notifications_github_delivery_id
    ON notifications (github_delivery_id) WHERE github_delivery_id IS NOT NULL;

-- Fast unread count and unread panel queries (most common read path).
CREATE INDEX IF NOT EXISTS idx_notifications_user_unread
    ON notifications (user_id, created_at DESC) WHERE read_at IS NULL AND dismissed_at IS NULL AND deleted_at IS NULL;

-- General listing index (all notifications for a user, newest first).
CREATE INDEX IF NOT EXISTS idx_notifications_user_all
    ON notifications (user_id, created_at DESC) WHERE deleted_at IS NULL;

-- +goose StatementBegin
CREATE TRIGGER notifications_updated_at
    AFTER UPDATE ON notifications
    FOR EACH ROW
    WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE notifications
       SET updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
     WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER IF EXISTS notifications_updated_at;
DROP INDEX  IF EXISTS idx_notifications_user_all;
DROP INDEX  IF EXISTS idx_notifications_user_unread;
DROP INDEX  IF EXISTS uq_notifications_github_delivery_id;
DROP TABLE  IF EXISTS notifications;
