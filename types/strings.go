package hyperfeed

import "fmt"

// ============================ String Helper Functions =======================

func First(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func FirstOf[T fmt.Stringer](values ...T) T {
	var zero T
	for _, v := range values {
		if v.String() != "" {
			return v
		}
	}
	return zero
}
