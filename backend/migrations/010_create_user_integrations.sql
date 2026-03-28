-- +goose Up
CREATE TABLE IF NOT EXISTS user_integrations (
    id                CHAR(36) NOT NULL PRIMARY KEY CHECK(length(id) = 36),
    user_id           CHAR(36) NOT NULL REFERENCES users(id),
    provider          TEXT NOT NULL CHECK(provider IN ('github')),  -- 'github', future: 'slack', 'email'
    access_token      TEXT NOT NULL,              -- encrypted via EncryptionService
    refresh_token     TEXT,                       -- encrypted, nullable (GitHub doesn't use refresh tokens currently)
    provider_user_id  TEXT NOT NULL,              -- GitHub user ID
    provider_username TEXT NOT NULL,              -- GitHub username
    created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    deleted_at        TEXT
);

-- Partial unique index: one active integration per provider per user.
-- Allows reconnect after soft-delete (old row has deleted_at set; new row is active).
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

-- +goose Down
DROP TRIGGER IF EXISTS user_integrations_updated_at;
DROP INDEX  IF EXISTS uq_user_integrations_user_provider_active;
DROP TABLE  IF EXISTS user_integrations;
