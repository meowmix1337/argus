-- +goose Up
CREATE TABLE IF NOT EXISTS user_integrations (
    id                TEXT PRIMARY KEY,
    user_id           TEXT NOT NULL REFERENCES users(id),
    provider          TEXT NOT NULL,              -- 'github', future: 'slack', 'email'
    access_token      TEXT NOT NULL,              -- encrypted via EncryptionService
    refresh_token     TEXT,                       -- encrypted, nullable (GitHub doesn't use refresh tokens currently)
    provider_user_id  TEXT NOT NULL,              -- GitHub user ID
    provider_username TEXT NOT NULL,              -- GitHub username
    created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    deleted_at        TEXT,
    UNIQUE(user_id, provider) -- one integration per provider per user (active only enforced in app)
);

CREATE INDEX IF NOT EXISTS idx_user_integrations_user_provider
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
DROP INDEX  IF EXISTS idx_user_integrations_user_provider;
DROP TABLE  IF EXISTS user_integrations;
