package hyperfeed

import (
	"errors"
)

// ErrNotImplemented is returned when a requested feature is not implemented.
var ErrNotImplemented = errors.New("not implemented")

// ErrDeserialization is returned when data cannot be deserialized (e.g. Unmarshal errors).
var ErrDeserialization = errors.New("deserialization error")

// ErrSerialization is returned when data cannot be serialized (e.g. Marshal errors).
var ErrSerialization = errors.New("serialization error")

// ErrFeed is a generic feed error.
var ErrFeed = errors.New("feed error")
