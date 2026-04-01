package version

import (
	"fmt"
	"time"
)

// ============================ Package Variables ============================
var (
	LibVersion        = "v0.19.4"
	LibReleaseDate, _ = time.Parse(time.RFC3339, "2026-04-01T15:00:00-07:00")
	LibUserAgent      = fmt.Sprintf("HyperFeed/%s (+https://hypertexting.com/agents)", LibVersion)
)
