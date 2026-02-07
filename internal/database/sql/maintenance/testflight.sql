-- Example maintenance script.
-- This script records itself in hyperfeed_scripts.
-- This script will be wrapped in a transaction; no need to wrap in BEGIN/COMMIT.
INSERT INTO [hyperfeed_scripts] ([id], [description])
SELECT 'sql/maintenance/testflight.sql', 'example maintenance script'
WHERE NOT EXISTS (
    SELECT 1 FROM [hyperfeed_scripts] WHERE [id] = 'sql/maintenance/testflight.sql'
);
