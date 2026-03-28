-- +goose Up
-- SQLite does not support ALTER TABLE ADD CONSTRAINT, so tables must be
-- dropped and recreated to swap CHECK constraints for FK references.
-- Tables are brand-new (no production data) so this is safe.

-- Drop in reverse dependency order (notifications → user_watched_repos → user_integrations).
DROP TRIGGER IF EXISTS notifications_updated_at;
DROP INDEX  IF EXISTS idx_notifications_user_all;
DROP INDEX  IF EXISTS idx_notifications_user_unread;
DROP INDEX  IF EXISTS uq_notifications_github_delivery_id;
DROP TABLE  IF EXISTS notifications;

DROP TRIGGER IF EXISTS user_watched_repos_updated_at;
DROP INDEX  IF EXISTS idx_user_watched_repos_owner_repo;
DROP INDEX  IF EXISTS uq_user_watched_repos_user_owner_repo_active;
DROP TABLE  IF EXISTS user_watched_repos;

DROP TRIGGER IF EXISTS user_integrations_updated_at;
DROP INDEX  IF EXISTS uq_user_integrations_user_provider_active;
DROP TABLE  IF EXISTS user_integrations;

-- Recreate user_integrations with FK to provider_types.
CREATE TABLE IF NOT EXISTS user_integrations (
    id                CHAR(36) NOT NULL PRIMARY KEY CHECK(length(id) = 36),
    user_id           CHAR(36) NOT NULL REFERENCES users(id),
    provider_id       TEXT NOT NULL REFERENCES provider_types(id),
    access_token      TEXT NOT NULL,              -- encrypted via EncryptionService
    refresh_token     TEXT,                       -- encrypted, nullable (GitHub doesn't use refresh tokens currently)
    provider_user_id  TEXT NOT NULL,              -- GitHub user ID
    provider_username TEXT NOT NULL,              -- GitHub username
    created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    deleted_at        TEXT
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_user_integrations_user_provider_active
    ON user_integrations (user_id, provider_id) WHERE deleted_at IS NULL;

-- +goose StatementBegin
CREATE TRIGGER user_integrations_updated_at
    AFTER UPDATE ON user_integrations
    FOR EACH ROW
    WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE user_integrations
       SET updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
     WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- Recreate user_watched_repos (schema unchanged; must be recreated because it
-- references user_integrations which was just dropped and recreated).
CREATE TABLE IF NOT EXISTS user_watched_repos (
    id              CHAR(36) NOT NULL PRIMARY KEY CHECK(length(id) = 36),
    user_id         CHAR(36) NOT NULL REFERENCES users(id),
    integration_id  CHAR(36) NOT NULL REFERENCES user_integrations(id),
    owner           TEXT NOT NULL,               -- repo owner (org or user)
    repo            TEXT NOT NULL,               -- repo name
    webhook_id      TEXT NOT NULL,               -- GitHub webhook ID for cleanup
    webhook_secret  TEXT NOT NULL,               -- encrypted HMAC secret per repo
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    deleted_at      TEXT
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_user_watched_repos_user_owner_repo_active
    ON user_watched_repos (user_id, owner, repo) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_user_watched_repos_owner_repo
    ON user_watched_repos (owner, repo) WHERE deleted_at IS NULL;

-- +goose StatementBegin
CREATE TRIGGER user_watched_repos_updated_at
    AFTER UPDATE ON user_watched_repos
    FOR EACH ROW
    WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE user_watched_repos
       SET updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
     WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- Recreate notifications with FKs to provider_types and notification_event_types.
CREATE TABLE IF NOT EXISTS notifications (
    id                   CHAR(36) NOT NULL PRIMARY KEY CHECK(length(id) = 36),
    user_id              CHAR(36) NOT NULL REFERENCES users(id),
    provider_id          TEXT NOT NULL REFERENCES provider_types(id),
    event_type_id        TEXT NOT NULL REFERENCES notification_event_types(id),
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

CREATE UNIQUE INDEX IF NOT EXISTS uq_notifications_github_delivery_id
    ON notifications (github_delivery_id) WHERE github_delivery_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_notifications_user_unread
    ON notifications (user_id, created_at DESC) WHERE read_at IS NULL AND dismissed_at IS NULL AND deleted_at IS NULL;

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
-- Restore tables to post-010/011/012 state (CHECK constraints, no FK to lookup tables).

DROP TRIGGER IF EXISTS notifications_updated_at;
DROP INDEX  IF EXISTS idx_notifications_user_all;
DROP INDEX  IF EXISTS idx_notifications_user_unread;
DROP INDEX  IF EXISTS uq_notifications_github_delivery_id;
DROP TABLE  IF EXISTS notifications;

DROP TRIGGER IF EXISTS user_watched_repos_updated_at;
DROP INDEX  IF EXISTS idx_user_watched_repos_owner_repo;
DROP INDEX  IF EXISTS uq_user_watched_repos_user_owner_repo_active;
DROP TABLE  IF EXISTS user_watched_repos;

DROP TRIGGER IF EXISTS user_integrations_updated_at;
DROP INDEX  IF EXISTS uq_user_integrations_user_provider_active;
DROP TABLE  IF EXISTS user_integrations;

CREATE TABLE IF NOT EXISTS user_integrations (
    id                CHAR(36) NOT NULL PRIMARY KEY CHECK(length(id) = 36),
    user_id           CHAR(36) NOT NULL REFERENCES users(id),
    provider          TEXT NOT NULL CHECK(provider IN ('github')),
    access_token      TEXT NOT NULL,
    refresh_token     TEXT,
    provider_user_id  TEXT NOT NULL,
    provider_username TEXT NOT NULL,
    created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    deleted_at        TEXT
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_user_integrations_user_provider_active
    ON user_integrations (user_id, provider) WHERE deleted_at IS NULL;

-- +goose StatementBegin
CREATE TRIGGER user_integrations_updated_at
    AFTER UPDATE ON user_integrations
    FOR EACH ROW
    WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE user_integrations
       SET updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
     WHERE id = NEW.id;
END;
-- +goose StatementEnd

CREATE TABLE IF NOT EXISTS user_watched_repos (
    id              CHAR(36) NOT NULL PRIMARY KEY CHECK(length(id) = 36),
    user_id         CHAR(36) NOT NULL REFERENCES users(id),
    integration_id  CHAR(36) NOT NULL REFERENCES user_integrations(id),
    owner           TEXT NOT NULL,
    repo            TEXT NOT NULL,
    webhook_id      TEXT NOT NULL,
    webhook_secret  TEXT NOT NULL,
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    deleted_at      TEXT
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_user_watched_repos_user_owner_repo_active
    ON user_watched_repos (user_id, owner, repo) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_user_watched_repos_owner_repo
    ON user_watched_repos (owner, repo) WHERE deleted_at IS NULL;

-- +goose StatementBegin
CREATE TRIGGER user_watched_repos_updated_at
    AFTER UPDATE ON user_watched_repos
    FOR EACH ROW
    WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE user_watched_repos
       SET updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
     WHERE id = NEW.id;
END;
-- +goose StatementEnd

CREATE TABLE IF NOT EXISTS notifications (
    id                   CHAR(36) NOT NULL PRIMARY KEY CHECK(length(id) = 36),
    user_id              CHAR(36) NOT NULL REFERENCES users(id),
    provider             TEXT NOT NULL CHECK(provider IN ('github')),
    event_type           TEXT NOT NULL CHECK(event_type IN ('pr_opened', 'pr_merged', 'pr_closed', 'pr_comment', 'pr_review_comment')),
    title                TEXT NOT NULL,
    body                 TEXT,
    url                  TEXT,
    read_at              TEXT,
    dismissed_at         TEXT,
    github_delivery_id   TEXT,
    created_at           TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at           TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    deleted_at           TEXT
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_notifications_github_delivery_id
    ON notifications (github_delivery_id) WHERE github_delivery_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_notifications_user_unread
    ON notifications (user_id, created_at DESC) WHERE read_at IS NULL AND dismissed_at IS NULL AND deleted_at IS NULL;

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
