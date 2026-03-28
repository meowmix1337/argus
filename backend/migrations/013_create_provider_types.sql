-- +goose Up
CREATE TABLE IF NOT EXISTS provider_types (
    id         TEXT PRIMARY KEY,   -- 'github', future: 'slack', 'email'
    label      TEXT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0
);

INSERT INTO provider_types (id, label, sort_order) VALUES
    ('github', 'GitHub', 1);

-- +goose Down
DROP TABLE IF EXISTS provider_types;
