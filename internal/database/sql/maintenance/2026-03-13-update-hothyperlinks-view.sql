-- Maintenance script: drop and recreate hot_hyperlinks_view with a -3 day window.
-- This script will be wrapped in a transaction; no need to wrap in BEGIN/COMMIT.

-- Drop the existing view.
DROP VIEW IF EXISTS hot_hyperlinks_view;

-- Recreate the view with a -3 day window.
CREATE VIEW IF NOT EXISTS hot_hyperlinks_view AS
SELECT
    json_object(
        'website_id', [links].[website_id],
        'id', [links].[id],
        'href', [links].[href],
        'cover_href', NULLIF([links].[cover_href], ''),
        'title', [links].[title],
        'description', [links].[description],
        'website_count', [link_counts].[website_count],
        'item_count', [link_counts].[item_count],
        'score', [link_counts].[score]
    ) as 'link',
    [links].[website_id],
    [links].[id],
    [links].[href],
    [link_counts].[website_count],
    [link_counts].[item_count],
    [link_counts].[score]
FROM
    [links]
    JOIN (
        SELECT
            [item_links].[link_id],
            COUNT(DISTINCT [item_links].[website_id]) AS [website_count],
            COUNT(*) AS [item_count],
            SUM(COALESCE([item_links].[weight], 0)) AS [score],
            MAX([items].[feed_at]) AS [last_feed_at]
        FROM
            [item_links]
            JOIN [items] ON [items].[id] = [item_links].[item_id]
        WHERE
            [items].[feed_at] >= datetime('now', '-3 days')
        GROUP BY
            [item_links].[link_id]
        HAVING
            COUNT(DISTINCT [item_links].[website_id]) > 1 AND
            SUM(COALESCE([item_links].[weight], 0)) > 0.0
    ) AS [link_counts] ON [link_counts].[link_id] = [links].[id]
ORDER BY
    [link_counts].[website_count] DESC,
    [link_counts].[score] DESC,
    [link_counts].[item_count] DESC,
    [link_counts].[last_feed_at] DESC
LIMIT 100
;

-- Record this maintenance script.
INSERT INTO [hyperfeed_scripts] ([id], [description])
VALUES ('sql/maintenance/2026-03-13-update-hothyperlinks-view.sql', 'drop and recreate hot_hyperlinks_view with -3 day window')
ON CONFLICT DO UPDATE SET [completed_at] = CURRENT_TIMESTAMP;
