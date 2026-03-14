-- [discovery] table
-- Abort maintenance if [discovery].[snooze] does not exist.
INSERT INTO hyperfeed_scripts (id)
SELECT 'noop'
WHERE NOT EXISTS (
    SELECT * FROM pragma_table_info('discovery') WHERE [name] = 'snooze'
);

-- [discovery_view] view
DROP VIEW discovery_view;
CREATE VIEW IF NOT EXISTS discovery_view AS
SELECT
    json_object(
        'id', [discovery].[id],
        'url', [discovery].[url],
        'html_url', [discovery].[html_url],
        'favicon_url', [discovery].[favicon_url],
        'cover_url', [discovery].[cover_url],
        'mentions_url', [discovery].[mentions_url],
        'following_url', [discovery].[following_url],
        'email', [discovery].[email],
        'name', [discovery].[name],
        'title', [discovery].[title],
        'description', [discovery].[description],
        'username', [discovery].[username],
        'language', [discovery].[language],
        'occurrences', [discovery].[occurrences],
        'snooze', [discovery].[snooze],
        'created_at', [discovery].[created_at],
        'updated_at', [discovery].[updated_at]
    ) AS 'discovery',
    [discovery].[id],
    [discovery].[url],
    [discovery].[html_url],
    [discovery].[username],
    [discovery].[created_at],
    [discovery].[updated_at]
FROM
    [discovery]
LEFT OUTER JOIN [feeds] ON [discovery].[id] = [feeds].[id]
WHERE
    [discovery].[snooze] <= 0 AND
    [feeds].[id] IS NULL
ORDER BY
    [discovery].[updated_at] DESC,
    [discovery].[occurrences] DESC
;

-- Record this maintenance script.
INSERT INTO [hyperfeed_scripts] ([id], [description])
VALUES ('sql/maintenance/2026-03-13-update-discovery-view.sql', 'update discovery_view to use the new [discovery].[snooze] column')
ON CONFLICT DO UPDATE SET [completed_at] = CURRENT_TIMESTAMP;
