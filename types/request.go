// Package types holds the shared leaf types of the project. It is the bottom of
// the layering hierarchy described in context.md §3.6 and imports nothing
// internal.
package types

import "time"

// OpType is the kind of operation a Request represents.
type OpType uint8

const (
	// OpGet is a read of a key.
	OpGet OpType = iota
	// OpPut is a write of a key.
	OpPut
	// OpDelete is a removal of a key.
	OpDelete
)

// String implements fmt.Stringer.
func (o OpType) String() string {
	switch o {
	case OpGet:
		return "get"
	case OpPut:
		return "put"
	case OpDelete:
		return "delete"
	default:
		return "unknown"
	}
}

// Request is a single unit of work presented to the cache. It is produced by a
// trace source (synthetic generator, CSV loader or live injector) and consumed
// by the cache core and the workload monitor.
//
// FROZEN after P1: see docs/DECISIONS.md before changing.
type Request struct {
	Key       string
	Size      int64
	Timestamp time.Time
	RequestID uint64
	Op        OpType
}
