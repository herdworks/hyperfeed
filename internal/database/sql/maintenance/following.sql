-- Maintenance script: add following_url to the websites table.
-- This script will be wrapped in a transaction; no need to wrap in BEGIN/COMMIT.

INSERT INTO hyperfeed_scripts (id)
SELECT 'noop'
WHERE EXISTS (
    SELECT * FROM pragma_table_info('websites') WHERE [name] = 'following_url'
);

ALTER TABLE [websites] ADD COLUMN [following_url] TEXT;

-- Record this maintenance script.
INSERT INTO [hyperfeed_scripts] ([id], [description])
VALUES ('sql/maintenance/following.sql', 'add following_url to the websites table')
ON CONFLICT DO UPDATE SET [completed_at] = CURRENT_TIMESTAMP;
