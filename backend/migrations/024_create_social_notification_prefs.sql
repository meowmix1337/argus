-- +goose Up
CREATE TABLE IF NOT EXISTS social_notification_prefs (
    user_id      TEXT PRIMARY KEY REFERENCES users(id),
    mute_posts   INTEGER NOT NULL DEFAULT 0,
    mute_follows INTEGER NOT NULL DEFAULT 0,
    created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    deleted_at   TEXT
);

-- +goose StatementBegin
CREATE TRIGGER social_notification_prefs_updated_at
    AFTER UPDATE ON social_notification_prefs
    FOR EACH ROW
    WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE social_notification_prefs
       SET updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
     WHERE user_id = NEW.user_id;
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER IF EXISTS social_notification_prefs_updated_at;
DROP TABLE IF EXISTS social_notification_prefs;
