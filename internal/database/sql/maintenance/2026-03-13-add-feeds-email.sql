-- [feeds] table
INSERT INTO hyperfeed_scripts (id)
SELECT 'noop'
WHERE EXISTS (
    SELECT * FROM pragma_table_info('feeds') WHERE [name] = 'email'
);
ALTER TABLE [feeds] ADD COLUMN [email] TEXT;

-- Record this maintenance script.
INSERT INTO [hyperfeed_scripts] ([id], [description])
VALUES ('sql/maintenance/2026-03-13-add-feeds-email.sql', 'add [feeds].[email] column')
ON CONFLICT DO UPDATE SET [completed_at] = CURRENT_TIMESTAMP;
