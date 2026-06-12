-- Migration from version 5 to version 6.
-- Add searchable flat [search] columns for unified in-database search:
--   * websites_view gains a [search] column concatenating the website's own
--     text with the text of its FOLLOWED feeds (a website matches a query if
--     any feed the user follows matches — e.g. following the MKBHD YouTube
--     channel feed makes the youtube.com website match "MKBHD").
--   * hot_hyperlinks_view gains flat [title]/[description] columns and a
--     [search] column over title/description/href.
-- Consumers filter with a single `[search] LIKE '%query%'` condition; no
-- table changes, so this is a pure view rebuild (views cannot be ALTERed).

DROP VIEW IF EXISTS websites_view;
DROP VIEW IF EXISTS hot_hyperlinks_view;

CREATE VIEW websites_view AS
SELECT
    json_object(
        'id', [websites].[id],
        'html_url', NULLIF([websites].[html_url], ''),
        'favicon_url', NULLIF([websites].[favicon_url], ''),
        'cover_url', NULLIF([websites].[cover_url], ''),
        'mentions_url', NULLIF([websites].[mentions_url], ''),
        'following_url', NULLIF([websites].[following_url], ''),
        'email', [websites].[email],
        'name', [websites].[name],
        'username', [websites].[username],
        'description', [websites].[description],
        'verified', CASE WHEN [websites].[verified] = 1 THEN json('true') ELSE json('false') END,
        'following', CASE WHEN [websites].[following] = 1 THEN json('true') ELSE json('false') END
    ) AS 'website',
    [websites].[id],
    [websites].[html_url],
    [websites].[username],
    [websites].[email],
    [websites].[following],
    COALESCE([websites].[username], '') || ' ' ||
    COALESCE([websites].[name], '') || ' ' ||
    COALESCE([websites].[description], '') || ' ' ||
    COALESCE([websites].[html_url], '') || ' ' ||
    COALESCE((
        SELECT group_concat(
            COALESCE([feeds].[url], '') || ' ' ||
            COALESCE([feeds].[html_url], '') || ' ' ||
            COALESCE([feeds].[title], '') || ' ' ||
            COALESCE([feeds].[description], ''),
            ' '
        )
        FROM [feeds]
        WHERE [feeds].[website_id] = [websites].[id] AND [feeds].[following] = 1
    ), '') AS [search]
FROM
    [websites]
;

CREATE VIEW hot_hyperlinks_view AS
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
    [link_counts].[score],
    [links].[title],
    [links].[description],
    COALESCE([links].[title], '') || ' ' ||
    COALESCE([links].[description], '') || ' ' ||
    [links].[href] AS [search]
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
            [items].[feed_at] >= datetime('now', '-48 hours')
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

-- Version bump (must be last).
UPDATE hyperfeed_schema
SET version = 6, updated_at = CURRENT_TIMESTAMP
WHERE id = 1 AND version = 5;
