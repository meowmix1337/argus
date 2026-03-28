-- +goose Up
CREATE TABLE IF NOT EXISTS user_watched_repos (
    id              TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL REFERENCES users(id),
    integration_id  TEXT NOT NULL REFERENCES user_integrations(id),
    owner           TEXT NOT NULL,               -- repo owner (org or user)
    repo            TEXT NOT NULL,               -- repo name
    webhook_id      TEXT NOT NULL,               -- GitHub webhook ID for cleanup
    webhook_secret  TEXT NOT NULL,               -- encrypted HMAC secret per repo
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    deleted_at      TEXT,
    UNIQUE(user_id, owner, repo) -- one watch per repo per user (active only enforced in app)
);

CREATE INDEX IF NOT EXISTS idx_user_watched_repos_lookup
    ON user_watched_repos (owner, repo) WHERE deleted_at IS NULL;

-- +goose Down
DROP INDEX  IF EXISTS idx_user_watched_repos_lookup;
DROP TABLE  IF EXISTS user_watched_repos;
