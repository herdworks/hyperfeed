-- Check if the attachments.display column already exists and no-op if it does.
INSERT INTO hyperfeed_scripts (id)
SELECT 'noop'
WHERE EXISTS (
    SELECT * FROM pragma_table_info('attachments') WHERE [name] = 'display'
);

-- Add column attachments.display to indicate whether an attachment should be displayed in the UI.
ALTER TABLE attachments ADD COLUMN display INTEGER DEFAULT 0 CHECK (display IN (0, 1));

-- Record maintenance completion
INSERT OR IGNORE INTO hyperfeed_scripts (id, description, completed_at)
VALUES ('sql/maintenance/attachments_display_column.sql', 'added attachments.display column', CURRENT_TIMESTAMP);
