-- +goose Up
ALTER TABLE notifications ADD COLUMN reference_id TEXT;

-- Partial unique index: prevents duplicate social notifications on consumer retry.
-- NULL values are excluded (GitHub notifications don't set reference_id).
CREATE UNIQUE INDEX IF NOT EXISTS uq_notifications_reference
    ON notifications (user_id, event_type_id, reference_id)
    WHERE reference_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS uq_notifications_reference;
-- SQLite does not support DROP COLUMN before 3.35.0; leave column in place on rollback.
