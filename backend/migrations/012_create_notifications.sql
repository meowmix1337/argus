-- +goose Up
CREATE TABLE IF NOT EXISTS notifications (
    id                   TEXT PRIMARY KEY,
    user_id              TEXT NOT NULL REFERENCES users(id),
    provider             TEXT NOT NULL,           -- 'github', future: 'slack', 'email'
    event_type           TEXT NOT NULL,           -- 'pr_opened', 'pr_merged', 'pr_closed', 'pr_comment', 'pr_review_comment'
    title                TEXT NOT NULL,
    body                 TEXT,
    url                  TEXT,                    -- deep link to GitHub PR/comment
    read_at              TEXT,                    -- NULL = unread
    dismissed_at         TEXT,                    -- NULL = not dismissed
    github_delivery_id   TEXT UNIQUE,             -- X-GitHub-Delivery header; UNIQUE prevents duplicate notifications from GitHub retries
    created_at           TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_notifications_user_unread
    ON notifications (user_id, created_at DESC) WHERE read_at IS NULL AND dismissed_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_notifications_user_all
    ON notifications (user_id, created_at DESC);

-- +goose Down
DROP INDEX  IF EXISTS idx_notifications_user_all;
DROP INDEX  IF EXISTS idx_notifications_user_unread;
DROP TABLE  IF EXISTS notifications;
