-- +goose Up
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

-- Partial unique index: one active watch per repo per user.
-- Allows re-watching after unwatch (old row has deleted_at set; new row is active).
CREATE UNIQUE INDEX IF NOT EXISTS uq_user_watched_repos_user_owner_repo_active
    ON user_watched_repos (user_id, owner, repo) WHERE deleted_at IS NULL;

-- Index for incoming webhook dispatch: look up watched repos by owner+repo.
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

-- +goose Down
DROP TRIGGER IF EXISTS user_watched_repos_updated_at;
DROP INDEX  IF EXISTS idx_user_watched_repos_owner_repo;
DROP INDEX  IF EXISTS uq_user_watched_repos_user_owner_repo_active;
DROP TABLE  IF EXISTS user_watched_repos;
