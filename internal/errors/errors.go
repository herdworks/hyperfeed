package errors

import (
	sterrors "errors"
	"fmt"
	"time"
)

// Kind classifies high-level error semantics for control flow.
type Kind int

const (
	KindUnknown     Kind = iota
	KindNotModified      // HTTP 304 or equivalent no-change indicator
	KindRateLimited      // Retry after a delay
	KindBackoff          // Server requested reduced frequency/backoff
	KindTemporary        // Temporary/transient condition
	KindPermanent        // Permanent failure for the given input
)

// Error is a structured error with a Kind and optional metadata.
type Error struct {
	Kind       Kind
	Op         string // optional operation/context label
	Err        error  // wrapped underlying error, if any
	RetryAfter time.Duration
	StatusCode int
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.opString(), e.Err)
	}
	return e.opString()
}

func (e *Error) opString() string {
	if e.Op != "" {
		return e.Op
	}
	return "hyperfeed"
}

func (e *Error) Unwrap() error { return e.Err }

// Sentinels for common conditions.
var (
	ErrNotModified = &Error{Kind: KindNotModified}
	ErrRateLimited = &Error{Kind: KindRateLimited}
	ErrBackoff     = &Error{Kind: KindBackoff}
	ErrTemporary   = &Error{Kind: KindTemporary}
	ErrPermanent   = &Error{Kind: KindPermanent}
)

// IsKind reports whether err (possibly wrapped) has the given Kind.
func IsKind(err error, k Kind) bool {
	var e *Error
	if sterrors.As(err, &e) {
		return e.Kind == k
	}
	return false
}
