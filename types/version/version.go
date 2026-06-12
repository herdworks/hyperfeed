package version

import (
	"fmt"
	"time"
)

// ============================ Package Variables ============================
var (
	LibVersion        = "v0.25.0"
	LibReleaseDate, _ = time.Parse(time.RFC3339, "2026-06-12T00:00:00-07:00")
	LibUserAgent      = fmt.Sprintf("HyperFeed/%s (+https://hypertexting.com/agents)", LibVersion)
)
