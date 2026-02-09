-- Maintenance script conditions
-- If the condition doesn't exist, attempting a "noop" write to hyperfeed_scripts triggers the maintenance system to abort this script with no error.
INSERT INTO hyperfeed_scripts (id)
SELECT 'noop'
WHERE EXISTS (
    SELECT * FROM pragma_table_info('attachments') WHERE [name] = 'display'
);

-- Maintenance script body


-- Maintenance script idempotency
INSERT INTO [hyperfeed_scripts] ([id], [description])
VALUES ('sql/maintenance/template.sql', 'template maintenance script')
ON CONFLICT DO UPDATE SET [completed_at] = CURRENT_TIMESTAMP;
