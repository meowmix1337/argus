-- +goose Up
INSERT OR IGNORE INTO provider_types (id, label, sort_order) VALUES ('social', 'Social', 10);

-- +goose Down
DELETE FROM provider_types WHERE id = 'social';
