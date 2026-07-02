package version

import (
	"fmt"
	"time"
)

// ============================ Package Variables ============================
var (
	LibVersion        = "v0.26.5"
	LibReleaseDate, _ = time.Parse(time.RFC3339, "2026-07-02T13:00:00-07:00")
	LibUserAgent      = fmt.Sprintf("HyperFeed/%s (+https://hypertexting.com/agents)", LibVersion)
)
