package hyperfeed

// ============================ String Helper Functions =======================

func First(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
