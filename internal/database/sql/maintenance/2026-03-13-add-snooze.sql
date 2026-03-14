-- [discovery] table
INSERT INTO hyperfeed_scripts (id)
SELECT 'noop'
WHERE EXISTS (
    SELECT * FROM pragma_table_info('discovery') WHERE [name] = 'snooze'
);
ALTER TABLE [discovery] ADD COLUMN [snooze] INTEGER DEFAULT 0;

-- Record this maintenance script.
INSERT INTO [hyperfeed_scripts] ([id], [description])
VALUES ('sql/maintenance/snooze.sql', 'add snooze column to the discovery table')
ON CONFLICT DO UPDATE SET [completed_at] = CURRENT_TIMESTAMP;
