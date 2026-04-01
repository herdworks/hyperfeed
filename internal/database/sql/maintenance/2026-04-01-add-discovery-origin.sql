-- [discovery] table
INSERT INTO hyperfeed_scripts (id)
SELECT 'noop'
WHERE EXISTS (
    SELECT * FROM pragma_table_info('discovery') WHERE [name] = 'origin'
);
ALTER TABLE [discovery] ADD COLUMN [origin] TEXT;

-- Record this maintenance script.
INSERT INTO [hyperfeed_scripts] ([id], [description])
VALUES ('sql/maintenance/2026-04-01-add-discovery-origin.sql', 'add origin column to the discovery table')
ON CONFLICT DO UPDATE SET [completed_at] = CURRENT_TIMESTAMP;
