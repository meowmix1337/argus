-- +goose Up

-- Fixed bill category lookup table (not user-defined in V1)
CREATE TABLE IF NOT EXISTS bill_category_types (
    id         TEXT PRIMARY KEY,
    label      TEXT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0
);

INSERT INTO bill_category_types (id, label, sort_order) VALUES
    ('rent',          'Rent / Mortgage', 1),
    ('utilities',     'Utilities',       2),
    ('subscriptions', 'Subscriptions',   3),
    ('insurance',     'Insurance',       4),
    ('loans',         'Loans',           5),
    ('medical',       'Medical',         6),
    ('other',         'Other',           7);

-- Per-user bills (due-date tracker)
-- recurrence_type values:
--   'once'      — one-time bill; due_date (YYYY-MM-DD) required
--   'monthly'   — every month; due_day (1-31) required
--   'annual'    — once per year; due_day + due_month required
--   'weekly'    — every week; anchor_date (YYYY-MM-DD) required
--   'biweekly'  — every two weeks; anchor_date required
--   'quarterly' — every three months; anchor_date required
CREATE TABLE IF NOT EXISTS bills (
    id               CHAR(36) NOT NULL PRIMARY KEY CHECK(length(id) = 36),
    user_id          CHAR(36) NOT NULL REFERENCES users(id),
    name             TEXT NOT NULL,
    amount           REAL,           -- nullable; optional dollar amount
    category_id      TEXT NOT NULL REFERENCES bill_category_types(id),
    recurrence_type  TEXT NOT NULL
                     CHECK(recurrence_type IN ('once','weekly','biweekly','monthly','quarterly','annual')),
    -- Fields used depending on recurrence_type (see comments above)
    due_date         TEXT,           -- YYYY-MM-DD; for 'once'
    due_day          INTEGER,        -- 1-31; for 'monthly' and 'annual'
    due_month        INTEGER,        -- 1-12; for 'annual'
    anchor_date      TEXT,           -- YYYY-MM-DD; reference date for 'weekly','biweekly','quarterly'
    notes            TEXT,           -- optional free text or URL
    created_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    deleted_at       TEXT
);

CREATE INDEX IF NOT EXISTS idx_bills_user_active
    ON bills (user_id) WHERE deleted_at IS NULL;

-- +goose StatementBegin
CREATE TRIGGER bills_updated_at
    AFTER UPDATE ON bills
    FOR EACH ROW
    WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE bills
       SET updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
     WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER IF EXISTS bills_updated_at;
DROP INDEX  IF EXISTS idx_bills_user_active;
DROP TABLE  IF EXISTS bills;
DROP TABLE  IF EXISTS bill_category_types;
