-- +goose Up
CREATE TABLE IF NOT EXISTS bill_payments (
    id                TEXT PRIMARY KEY,
    bill_id           TEXT NOT NULL REFERENCES bills(id),
    user_id           TEXT NOT NULL REFERENCES users(id),
    computed_due_date TEXT NOT NULL,  -- YYYY-MM-DD: identifies which occurrence was paid
    paid_date         TEXT NOT NULL,  -- YYYY-MM-DD: user-entered actual payment date
    note              TEXT,           -- optional, max 32 chars enforced in service
    created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE(bill_id, computed_due_date)
);

CREATE INDEX IF NOT EXISTS idx_bill_payments_user_year
    ON bill_payments (user_id, computed_due_date);

-- +goose Down
DROP INDEX  IF EXISTS idx_bill_payments_user_year;
DROP TABLE  IF EXISTS bill_payments;
