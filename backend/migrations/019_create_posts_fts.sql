-- +goose Up

-- Full-text search index for post content.
-- Maintained by triggers on posts; soft-deleted posts are excluded from the index.
-- Query: SELECT post_id FROM posts_fts WHERE posts_fts MATCH ? ORDER BY rank;
CREATE VIRTUAL TABLE IF NOT EXISTS posts_fts USING fts5(
    post_id UNINDEXED,  -- maps back to posts.id for JOIN
    content             -- indexed: post text (same 128-char limit as posts.content)
);

-- Seed FTS from any posts already in the database at migration time.
INSERT INTO posts_fts (post_id, content)
SELECT id, content FROM posts WHERE deleted_at IS NULL;

-- Insert new post into FTS index (skipped for posts inserted already-deleted).
-- +goose StatementBegin
CREATE TRIGGER posts_fts_insert
    AFTER INSERT ON posts
    FOR EACH ROW
    WHEN NEW.deleted_at IS NULL
BEGIN
    INSERT INTO posts_fts (post_id, content) VALUES (NEW.id, NEW.content);
END;
-- +goose StatementEnd

-- Keep FTS in sync when a post is edited or soft-deleted.
-- Always removes then re-inserts so content changes and soft-deletes are both handled.
-- +goose StatementBegin
CREATE TRIGGER posts_fts_update
    AFTER UPDATE ON posts
    FOR EACH ROW
BEGIN
    DELETE FROM posts_fts WHERE post_id = OLD.id;
    INSERT INTO posts_fts (post_id, content)
        SELECT id, content FROM posts WHERE id = NEW.id AND deleted_at IS NULL;
END;
-- +goose StatementEnd

-- Remove hard-deleted posts from FTS (defensive; app uses soft-delete).
-- +goose StatementBegin
CREATE TRIGGER posts_fts_delete
    AFTER DELETE ON posts
    FOR EACH ROW
BEGIN
    DELETE FROM posts_fts WHERE post_id = OLD.id;
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER IF EXISTS posts_fts_delete;
DROP TRIGGER IF EXISTS posts_fts_update;
DROP TRIGGER IF EXISTS posts_fts_insert;
DROP TABLE  IF EXISTS posts_fts;
