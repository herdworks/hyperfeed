-- Backfill attachments.display to match Attachment.Displayable().
UPDATE attachments
SET display = CASE
    WHEN href LIKE '%/tracking/%' THEN 0
    WHEN max(COALESCE(width, 0), COALESCE(height, 0)) > 0 AND
         max(COALESCE(width, 0), COALESCE(height, 0)) < 512 THEN 0
    ELSE 1
END
WHERE display IS NULL;

-- Record maintenance completion
INSERT OR IGNORE INTO hyperfeed_scripts (id, description, completed_at)
VALUES ('sql/maintenance/attachments_display_fill.sql', 'backfill attachments.display', CURRENT_TIMESTAMP);
