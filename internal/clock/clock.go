// Package clock implements logical clocks without external dependencies.
package clock

// Relation is the partial-order relationship between two timestamps.
type Relation string

const (
	Before     Relation = "before"
	After      Relation = "after"
	Equal      Relation = "equal"
	Concurrent Relation = "concurrent"
)

type Vector []uint64

func (v Vector) Clone() Vector   { return append(Vector(nil), v...) }
func (v Vector) Tick(worker int) { v[worker]++ }

// Merge takes the componentwise maximum. Dimensions must match.
func (v Vector) Merge(other Vector) {
	if len(v) != len(other) {
		panic("vector clock dimension mismatch")
	}
	for i, n := range other {
		if n > v[i] {
			v[i] = n
		}
	}
}

func Compare(a, b Vector) Relation {
	if len(a) != len(b) {
		panic("vector clock dimension mismatch")
	}
	less, greater := false, false
	for i := range a {
		less = less || a[i] < b[i]
		greater = greater || a[i] > b[i]
	}
	switch {
	case less && greater:
		return Concurrent
	case less:
		return Before
	case greater:
		return After
	default:
		return Equal
	}
}

// Scalar orders numeric values only; Before does not prove causation.
func Scalar[T ~int64 | ~uint64](a, b T) Relation {
	if a < b {
		return Before
	}
	if a > b {
		return After
	}
	return Equal
}

// LamportEvent merges an optional incoming clock and increments once.
func LamportEvent(local, incoming uint64) uint64 {
	if incoming > local {
		local = incoming
	}
	return local + 1
}
