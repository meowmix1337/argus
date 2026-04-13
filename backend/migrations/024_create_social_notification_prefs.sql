-- +goose Up
CREATE TABLE IF NOT EXISTS social_notification_prefs (
    user_id      TEXT PRIMARY KEY REFERENCES users(id),
    mute_posts   INTEGER NOT NULL DEFAULT 0,
    mute_follows INTEGER NOT NULL DEFAULT 0,
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS social_notification_prefs;
