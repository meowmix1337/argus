-- +goose Up
CREATE TABLE IF NOT EXISTS notification_event_types (
    id         TEXT PRIMARY KEY,   -- 'pr_opened', 'pr_merged', etc.
    label      TEXT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0
);

INSERT INTO notification_event_types (id, label, sort_order) VALUES
    ('pr_opened',         'PR Opened',         1),
    ('pr_merged',         'PR Merged',         2),
    ('pr_closed',         'PR Closed',         3),
    ('pr_comment',        'PR Comment',        4),
    ('pr_review_comment', 'PR Review Comment', 5);

-- +goose Down
DROP TABLE IF EXISTS notification_event_types;
