//go:build frb

package feeder

import (
	"testing"
)

// ==================== Feed Reader Behavior Tests      =======================
// ==================== https://rachelbythebay.com/frb/ =======================

// FRB001: Last-Modified storage
// Consistently recognizes and stores Last-Modified data from [status 200] server responses.
func TestFRB001(t *testing.T) {
	t.Skip("FRB001: Last-Modified storage")
}

// FRB002: If-Modified-Since generation
// Generates well-formed conditional requests with If-Modified-Since headers using unaltered Last-Modified data.
func TestFRB002(t *testing.T) {
	t.Skip("FRB002: If-Modified-Since generation")
}

// FRB003: ETag storage
// Consistently recognizes and stores ETag data from [status 200] server responses.
func TestFRB003(t *testing.T) {
	t.Skip("FRB003: ETag storage")
}

// FRB004: If-None-Match generation
// Generates well-formed conditional requests with If-None-Match using unaltered ETag data.
func TestFRB004(t *testing.T) {
	t.Skip("FRB004: If-None-Match generation")
}

// FRB010: No short-term memory loss for cache parameters
// Never "forgets" cache parameters (Last-Modified, ETag) after 12 hours.
func TestFRB010(t *testing.T) {
	t.Skip("FRB010: No short-term memory loss for cache parameters")
}

// FRB011: No long-term memory loss for cache parameters
// Never "forgets" cache parameters (Last-Modified, ETag) on any schedule.
func TestFRB011(t *testing.T) {
	t.Skip("FRB011: No long-term memory loss for cache parameters")
}

// FRB012: No "creative" cache IMS parameters
// Never invents new values for If-Modified-Since headers.
func TestFRB012(t *testing.T) {
	t.Skip("FRB012: No \"creative\" cache IMS parameters")
}

// FRB013: No "creative" cache INM parameters
// Never invents new values for If-None-Match headers.
func TestFRB013(t *testing.T) {
	t.Skip("FRB013: No \"creative\" cache INM parameters")
}

// FRB014: Last-Modified and ETag are an atomic set
// Never mixes ETag and Last-Modified values from different responses.
func TestFRB014(t *testing.T) {
	t.Skip("FRB014: Last-Modified and ETag are an atomic set")
}

// FRB015: Cache parameters survive software upgrades
// Never "forgets" cache parameters (Last-Modified, ETag) just because the program was upgraded to another version.
func TestFRB015(t *testing.T) {
	t.Skip("FRB015: Cache parameters survive software upgrades")
}

// FRB020: Slow down on request
// Honors "Retry-after" header from [status 429] "too many requests" responses to reduce polling frequency.
func TestFRB020(t *testing.T) {
	t.Skip("FRB020: Slow down on request")
}

// FRB021: Slow down even with no hints
// Honors [status 429] "too many requests" responses to reduce polling frequency, even if a "Retry-after" header isn't provided.
func TestFRB021(t *testing.T) {
	t.Skip("FRB021: Slow down even with no hints")
}

// FRB022: Documents also have cache hints
// Honors Cache-Control header max-age setting to reduce polling frequency.
func TestFRB022(t *testing.T) {
	t.Skip("FRB022: Documents also have cache hints")
}

// FRB023: Match the rhythm of the feed
// Automatically slows polling rates for feeds that do not update very often.
func TestFRB023(t *testing.T) {
	t.Skip("FRB023: Match the rhythm of the feed")
}

// FRB024: Some feed specs give hints on when to poll
// Honors elements like "skipHours" from within RSS feeds and does not poll during those hours.
func TestFRB024(t *testing.T) {
	t.Skip("FRB024: Some feed specs give hints on when to poll")
}

// FRB030: No duplicate fetches of content (feed + posts)
// Only generates requests for original URLs for entries within a feed when explicitly requested by the user and never by default.
func TestFRB030(t *testing.T) {
	t.Skip("FRB030: No duplicate fetches of content (feed + posts)")
}

// FRB031: Other URLs also need proper caching behavior
// When requesting other URLs (individual URLs when explicitly requested, IMGs, icons, ...), uses proper caching, conditional requests and request timing as it would when requesting the feed itself.
func TestFRB031(t *testing.T) {
	t.Skip("FRB031: Other URLs also need proper caching behavior")
}

// FRB032: Don't hammer an item
// Never requests an item from the server repeatedly in a short timeframe, even if it's a conditional request.
func TestFRB032(t *testing.T) {
	t.Skip("FRB032: Don't hammer an item")
}

// FRB033: Don't hammer a server
// Staggers requests for multiple items from the same server such that it does not monopolize resources.
func TestFRB033(t *testing.T) {
	t.Skip("FRB033: Don't hammer a server")
}

// FRB034: Many feeds, few servers
// Recognizes that multiple feeds can come from the same server by hostname and sequences requests appropriately.
func TestFRB034(t *testing.T) {
	t.Skip("FRB034: Many feeds, few servers")
}

// FRB035: Many hostnames, few IPs
// Recognizes that multiple feeds can come from the same physical server by IP address (and yet different hostnames) and sequences requests appropriately.
func TestFRB035(t *testing.T) {
	t.Skip("FRB035: Many hostnames, few IPs")
}

// FRB036: Feed addition equals a single request
// Adds new feeds with a single request for the specified URL and gets every bit of data it needs from that one operation only.
func TestFRB036(t *testing.T) {
	t.Skip("FRB036: Feed addition equals a single request")
}

// FRB037: Relying on cron is not enough
// Never fetches again just because the program was started again.
func TestFRB037(t *testing.T) {
	t.Skip("FRB037: Relying on cron is not enough")
}

// FRB040: Many devices, one user, one cache
// Manages instances across multiple devices for the same user by sharing a cache or other state.
func TestFRB040(t *testing.T) {
	t.Skip("FRB040: Many devices, one user, one cache")
}

// FRB041: Multiple devices do not get additional polls
// Manages instances across multiple devices for the same user such that they all collectively honor polling frequencies.
func TestFRB041(t *testing.T) {
	t.Skip("FRB041: Multiple devices do not get additional polls")
}

// FRB042: No multiplicative effects
// Never adds more server load just because there are multiple devices running the feed reader on behalf of the user.
func TestFRB042(t *testing.T) {
	t.Skip("FRB042: No multiplicative effects")
}

// FRB050: Use GET
// Only requests resources with the GET method. This applies to the feed and other resources as well.
func TestFRB050(t *testing.T) {
	t.Skip("FRB050: Use GET")
}

// FRB051: No HEAD
// Never requests resources with the HEAD method.
func TestFRB051(t *testing.T) {
	t.Skip("FRB051: No HEAD")
}

// FRB060: Check user input of URLs
// Sanity-checks URLs at feed creation time in case they "paste in" a space, carriage return, quotes, or any other characters which are unlikely for a URL. Assists the user if needed.
func TestFRB060(t *testing.T) {
	t.Skip("FRB060: Check user input of URLs")
}

// FRB061: Don't guess at the URL provided
// Uses the URL as provided by the user. Feed reader should report on problems and allow the user to change it to something else.
func TestFRB061(t *testing.T) {
	t.Skip("FRB061: Don't guess at the URL provided")
}

// FRB062: No extra parameters
// Never appends extra parameters to the URL except as explicitly requested by the user for a specific feed.
func TestFRB062(t *testing.T) {
	t.Skip("FRB062: No extra parameters")
}

// FRB063: No cookies by default
// Never sends Cookie headers unless explicitly requested by the user for a specific feed.
func TestFRB063(t *testing.T) {
	t.Skip("FRB063: No cookies by default")
}

// FRB064: No cache-busting behavior
// Never attempts "cache buster" activities such as "no-cache" headers, "bypass Varnish" cookies, appending nonces to URLs, or any other technique.
func TestFRB064(t *testing.T) {
	t.Skip("FRB064: No cache-busting behavior")
}

// FRB065: No refer(r)ers
// Never sends Referer headers when requesting a feed.
func TestFRB065(t *testing.T) {
	t.Skip("FRB065: No refer(r)ers")
}

// FRB066: No "normalizing" query strings
// Allows URLs like "http://example.org/feed?atom" to exist without mangling the "?atom" part into something else.
func TestFRB066(t *testing.T) {
	t.Skip("FRB066: No \"normalizing\" query strings")
}

// FRB070: Limit on-demand fetch activity
// Does at most a single conditional "forced fetch" of a single feed when explicitly (and interactively) requested by the user for that specific feed. Treats it as an exceptional event and does not attempt to normalize such behavior.
func TestFRB070(t *testing.T) {
	t.Skip("FRB070: Limit on-demand fetch activity")
}

// FRB071: Confirmations for unconditional requests
// Requires explicit confirmation from the user to do an unconditional request during a "force fetch" of a feed, like how a "shift reload" works in a web browser. Warns the user about generating excessive requests, and reports if it fails.
func TestFRB071(t *testing.T) {
	t.Skip("FRB071: Confirmations for unconditional requests")
}

// FRB080: Descriptive program identifier
// Sends descriptive User-Agent header which identifies the client as a feed reader program by default.
func TestFRB080(t *testing.T) {
	t.Skip("FRB080: Descriptive program identifier")
}

// FRB081: Program/project/product URL
// Sends contact URL for the feed reader in the User-Agent header, if appropriate.
func TestFRB081(t *testing.T) {
	t.Skip("FRB081: Program/project/product URL")
}

// FRB082: Versioning
// Sends a version number in the User-Agent header.
func TestFRB082(t *testing.T) {
	t.Skip("FRB082: Versioning")
}

// FRB083: Useful versioning
// Version number in User-Agent changes in a useful way at appropriate intervals. Version number is not random or static.
func TestFRB083(t *testing.T) {
	t.Skip("FRB083: Useful versioning")
}

// FRB084: No masquerading as another reader by default
// Never claims to be another program in the User-Agent by default.
func TestFRB084(t *testing.T) {
	t.Skip("FRB084: No masquerading as another reader by default")
}

// FRB085: No masquerading as a browser by default
// Never claims to be a "real web browser" by default.
func TestFRB085(t *testing.T) {
	t.Skip("FRB085: No masquerading as a browser by default")
}

// FRB086: You are not your libraries
// Never sends default User-Agent data from underlying libraries.
func TestFRB086(t *testing.T) {
	t.Skip("FRB086: You are not your libraries")
}

// FRB087: No morphing
// Never randomizes or rotates elements of the User-Agent header.
func TestFRB087(t *testing.T) {
	t.Skip("FRB087: No morphing")
}

// FRB088: Maintain user privacy in User-Agent header
// Never provide a user ID in the user-agent header unless explicitly requested by the user for a specific feed after explaining the privacy implications of such a setting.
func TestFRB088(t *testing.T) {
	t.Skip("FRB088: Maintain user privacy in User-Agent header")
}

// FRB089: You are not jwz
// Never claims to be "Mozilla" in the User-Agent header.
func TestFRB089(t *testing.T) {
	t.Skip("FRB089: You are not jwz")
}

// FRB100: Don't add bad URLs
// Honors non-[status 200] response (3xx, 4xx, 5xx) at feed creation time and will not add a broken feed.
func TestFRB100(t *testing.T) {
	t.Skip("FRB100: Don't add bad URLs")
}

// FRB101: Provide bodies on feed creation failure
// Makes the bodies of a non-[status 200] response available to the user at feed addition time in case it explains what's going on.
func TestFRB101(t *testing.T) {
	t.Skip("FRB101: Provide bodies on feed creation failure")
}

// FRB102: Don't pick that up if it's not a feed
// Recognizes non-feed (not RSS, not Atom, ...) responses at feed addition time and will not add a non-feed URL.
func TestFRB102(t *testing.T) {
	t.Skip("FRB102: Don't pick that up if it's not a feed")
}

// FRB103: Notice link hints to feed URLs
// Honors "link rel=alternate" method of feed detection when pointed at a non-feed URL by the user.
func TestFRB103(t *testing.T) {
	t.Skip("FRB103: Notice link hints to feed URLs")
}

// FRB104: No guessing at feed URLs
// Never attempts to "guess" a feed URL.
func TestFRB104(t *testing.T) {
	t.Skip("FRB104: No guessing at feed URLs")
}

// FRB105: No guessing at other URLs
// Never attempts to "guess" any other resource on the server.
func TestFRB105(t *testing.T) {
	t.Skip("FRB105: No guessing at other URLs")
}

// FRB106: Maintain user privacy of URLs
// Never sends feed URLs to "cloud" servers or any other servers beyond the user's control without user's explicit consent.
func TestFRB106(t *testing.T) {
	t.Skip("FRB106: Maintain user privacy of URLs")
}

// FRB110: Slow down and then stop if it stops being a feed
// Recognizes non-feed responses on existing feeds by reducing polling frequency until it eventually disables the feed.
func TestFRB110(t *testing.T) {
	t.Skip("FRB110: Slow down and then stop if it stops being a feed")
}

// FRB111: Tell the user about feed retirement
// When a URL stops providing feed content, let the user know so they have the choice of doing something about it.
func TestFRB111(t *testing.T) {
	t.Skip("FRB111: Tell the user about feed retirement")
}

// FRB112: Slow down and stop on 403
// Honors [status 403] "forbidden" responses on existing feeds by reducing polling frequency until it eventually disables the feed.
func TestFRB112(t *testing.T) {
	t.Skip("FRB112: Slow down and stop on 403")
}

// FRB113: Slow down and stop on 404
// Honors [status 404] "not found" responses on existing feeds by reducing polling frequency until it eventually disables the feed.
func TestFRB113(t *testing.T) {
	t.Skip("FRB113: Slow down and stop on 404")
}

// FRB114: Stop on 410
// Honors [status 410] "gone" responses on existing feeds by disabling the feed.
func TestFRB114(t *testing.T) {
	t.Skip("FRB114: Stop on 410")
}

// FRB115: Slow down and stop on other errors
// Honors other HTTP error responses (4xx, 5xx) on existing feeds by reducing polling frequency until it eventually disables the feed.
func TestFRB115(t *testing.T) {
	t.Skip("FRB115: Slow down and stop on other errors")
}

// FRB116: Slow down and stop on DNS failures
// Recognizes hostnames no longer resolving in DNS and reduces polling frequencies until the feed is disabled.
func TestFRB116(t *testing.T) {
	t.Skip("FRB116: Slow down and stop on DNS failures")
}

// FRB117: Slow down and stop on connection failures
// Recognizes hostnames no longer answering connections and reduces polling frequencies until the feed is disabled.
func TestFRB117(t *testing.T) {
	t.Skip("FRB117: Slow down and stop on connection failures")
}

// FRB118: Slow down and stop on server response failures
// Recognizes hostnames no longer returning useful responses after answering a connection and reduces polling frequencies until the feed is disabled.
func TestFRB118(t *testing.T) {
	t.Skip("FRB118: Slow down and stop on server response failures")
}

// FRB119: Slow down on errors
// Handles fetch errors by slowing down instead of speeding up.
func TestFRB119(t *testing.T) {
	t.Skip("FRB119: Slow down on errors")
}

// FRB120: Communicate errors and response bodies to the user
// Provides bodies of error responses in reports of feed fetching problems in case the server is trying to communicate with them: *why* is it a 403, 404, 410?
func TestFRB120(t *testing.T) {
	t.Skip("FRB120: Communicate errors and response bodies to the user")
}

// FRB130: Permanent moves, 301 flavor
// Honors [status 301] "moved permanently" responses by following the Location header and ceasing polls of the old URL.
func TestFRB130(t *testing.T) {
	t.Skip("FRB130: Permanent moves, 301 flavor")
}

// FRB131: Permanent moves, 308 flavor
// Honors [status 308] "permanent redirect" responses by following the Location header and ceasing polls of the old URL.
func TestFRB131(t *testing.T) {
	t.Skip("FRB131: Permanent moves, 308 flavor")
}

// FRB132: Inform the user on permanent moves
// Informs user about changes to the URL due to 301/308 redirects.
func TestFRB132(t *testing.T) {
	t.Skip("FRB132: Inform the user on permanent moves")
}

// FRB133: Temporary moves, 302 flavor
// Honors [status 302] "found" responses by following the Location header.
func TestFRB133(t *testing.T) {
	t.Skip("FRB133: Temporary moves, 302 flavor")
}

// FRB134: Temporary moves, 307 flavor
// Honors [status 307] "temporary redirect" responses by following the Location header.
func TestFRB134(t *testing.T) {
	t.Skip("FRB134: Temporary moves, 307 flavor")
}

// FRB135: Inform the user on temporary moves
// Informs user about changes to the URL due to 302/307 redirects.
func TestFRB135(t *testing.T) {
	t.Skip("FRB135: Inform the user on temporary moves")
}

// FRB136: Inform the user of TLS problems
// Verifies identity components and brings problems with them to the attention of the user.
func TestFRB136(t *testing.T) {
	t.Skip("FRB136: Inform the user of TLS problems")
}

// FRB137: Feeds can merge
// Recognizes when multiple feeds now have the same URL and reduces it to a single poll-worthy entry.
func TestFRB137(t *testing.T) {
	t.Skip("FRB137: Feeds can merge")
}

// FRB140: Response compression
// Supports at least one widely-used algorithm for compression of HTTP responses and uses it as it would an uncompressed response.
func TestFRB140(t *testing.T) {
	t.Skip("FRB140: Response compression")
}

// FRB141: Willingness to decompress
// Conveys appropriate values in Accept-Encoding header(s) to the server to indicate willingness to use responses which are compressed in some way.
func TestFRB141(t *testing.T) {
	t.Skip("FRB141: Willingness to decompress")
}
