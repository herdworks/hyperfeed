package version

import (
	"fmt"
	"time"
)

// ============================ Package Variables ============================
var (
	LibVersion        = "v0.26.4"
	LibReleaseDate, _ = time.Parse(time.RFC3339, "2026-07-01T10:58:23-07:00")
	LibUserAgent      = fmt.Sprintf("HyperFeed/%s (+https://hypertexting.com/agents)", LibVersion)
)
