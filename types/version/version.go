package version

import (
	"fmt"
	"time"
)

// ============================ Package Variables ============================
var (
	LibVersion        = "v0.19.0"
	LibReleaseDate, _ = time.Parse(time.RFC3339, "2026-03-30T11:00:00-08:00")
	LibUserAgent      = fmt.Sprintf("HyperFeed/%s (+https://hypertexting.com/agents)", LibVersion)
)
