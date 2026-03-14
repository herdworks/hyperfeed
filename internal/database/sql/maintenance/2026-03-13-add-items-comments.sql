-- [items] table
INSERT INTO hyperfeed_scripts (id)
SELECT 'noop'
WHERE EXISTS (
    SELECT * FROM pragma_table_info('items') WHERE [name] = 'comments_url'
);
ALTER TABLE [items] ADD COLUMN [comments_url] TEXT;

-- Record this maintenance script.
INSERT INTO [hyperfeed_scripts] ([id], [description])
VALUES ('sql/maintenance/2026-03-13-add-items-comments.sql', 'add [items].[comments_url] column')
ON CONFLICT DO UPDATE SET [completed_at] = CURRENT_TIMESTAMP;
