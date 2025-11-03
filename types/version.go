package hyperfeed

import "time"

// ============================ Package Variables ============================
var (
	LibVersion        = "v0.1.0-dev"
	LibSchemaVersion  = 3
	LibReleaseDate, _ = time.Parse(time.RFC3339, "2025-11-03T10:00:00-08:00")
)
