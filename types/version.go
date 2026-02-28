package hyperfeed

import (
	"fmt"
	"time"
)

// ============================ Package Variables ============================
var (
	LibVersion        = "v0.18.0"
	LibReleaseDate, _ = time.Parse(time.RFC3339, "2026-02-26T10:00:00-08:00")
	LibUserAgent      = fmt.Sprintf("HyperFeed/%s (+https://hypertexting.com/agents)", LibVersion)
)
