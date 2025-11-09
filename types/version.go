package hyperfeed

import (
	"fmt"
	"time"
)

// ============================ Package Variables ============================
var (
	LibVersion        = "v0.1.0-dev"
	LibSchemaVersion  = 4
	LibReleaseDate, _ = time.Parse(time.RFC3339, "2025-11-08T12:00:00-08:00")
	LibUserAgent      = fmt.Sprintf("HyperFeed/%s (+https://hypertexting.com)", LibVersion)
)
