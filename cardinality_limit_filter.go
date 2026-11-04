// Package cardinalitylimitfilter provides a filter that bounds the number of distinct
// label sets (cardinality) a metric can carry. When the configured limit is exceeded,
// new label sets are either dropped or collapsed into a single overflow bucket.
//
// The package is deliberately small and dependency-free so it can be embedded in
// agents that must not grow memory without bound.
package cardinalitylimitfilter

import (
	"errors"
	"sync"
)

// Mode selects what happens to a label set that would exceed the cardinality limit.
type Mode int

const (
	// Drop discards the label set entirely. The caller receives (nil, false).
	Drop Mode = iota
	// Aggregate replaces the label set with a single shared overflow label set.
	Aggregate
)

// ErrInvalidLimit is returned by New when limit is less than 1.
var ErrInvalidLimit = errors.New("cardinality limit must be at least 1")

// Filter tracks distinct label sets and enforces a cardinality limit.
//
// The zero value is not usable; call New. Filter is safe for concurrent use.
type Filter struct {
	mu      sync.Mutex
	limit   int
	mode    Mode
	seen    map[string]struct{}
	overflow []byte
}

// New returns a Filter that allows up to limit distinct label sets.
//
// When the limit is reached, behaviour depends on mode:
//   - Drop: Allow returns (nil, false) for unseen label sets.
//   - Aggregate: Allow returns a copy of the overflow label set for unseen label sets.
//
// The overflow label set is chosen by the caller so that it can be meaningful to
// downstream consumers (for example, a label that says "other").
func New(limit int, mode Mode, overflow []byte) (*Filter, error) {
	if limit < 1 {
		return nil, ErrInvalidLimit
	}
	// Copy the overflow slice so the caller cannot mutate it later and change
	// what the filter returns.
	o := make([]byte, len(overflow))
	copy(o, overflow)
	return &Filter{
		limit:    limit,
		mode:     mode,
		seen:     make(map[string]struct{}, limit),
		overflow: o,
	}, nil
}

// Allow reports whether the label set is admitted and returns the label set that
// should be used. The returned slice is always a copy, so callers may modify it.
//
// For a label set that has already been seen, Allow returns the same bytes that were
// passed in (as a copy). For a new label set within the limit, it is recorded and
// returned. For a new label set beyond the limit, behaviour follows the Mode.
func (f *Filter) Allow(labels []byte) ([]byte, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	key := string(labels)
	if _, ok := f.seen[key]; ok {
		return clone(labels), true
	}

	if len(f.seen) >= f.limit {
		switch f.mode {
		case Drop:
			return nil, false
		case Aggregate:
			return clone(f.overflow), true
		default:
			// Unknown mode is a programming error; treat it as Drop so we never
			// silently admit unbounded cardinality.
			return nil, false
		}
	}

	f.seen[key] = struct{}{}
	return clone(labels), true
}

// Len returns the number of distinct label sets currently tracked.
func (f *Filter) Len() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.seen)
}

// Limit returns the configured cardinality limit.
func (f *Filter) Limit() int {
	return f.limit
}

// clone returns a copy of b, or nil if b is nil. A non-nil empty slice is preserved
// as an empty slice so callers can distinguish "no labels" from "dropped".
func clone(b []byte) []byte {
	if b == nil {
		return nil
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out
}
