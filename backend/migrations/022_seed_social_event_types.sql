-- +goose Up
INSERT OR IGNORE INTO notification_event_types (id, label, sort_order) VALUES
    ('social.post.created', 'New Post',     10),
    ('social.new_follower', 'New Follower', 11);

-- +goose Down
DELETE FROM notification_event_types WHERE id IN ('social.post.created', 'social.new_follower');
