-- [attachments] table
UPDATE [attachments]
SET [display] = 1
WHERE [mimetype] = 'text/x-url' AND [display] = 0;

-- Record this maintenance script.
INSERT INTO [hyperfeed_scripts] ([id], [description])
VALUES ('sql/maintenance/2026-03-31-update-link-attachments.sql', 'fix [attachments].[display] for link attachments')
ON CONFLICT DO UPDATE SET [completed_at] = CURRENT_TIMESTAMP;
