package clock

import "testing"

func TestPartialOrder(t *testing.T) {
	for _, tc := range []struct {
		a, b Vector
		want Relation
	}{
		{Vector{1, 0}, Vector{1, 1}, Before},
		{Vector{2, 3}, Vector{1, 3}, After},
		{Vector{1, 0}, Vector{0, 1}, Concurrent},
		{Vector{2, 3}, Vector{2, 3}, Equal},
		{Vector{0, 0, 0, 0}, Vector{0, 0, 0, 1}, Before},
	} {
		if got := Compare(tc.a, tc.b); got != tc.want {
			t.Fatalf("%v vs %v: %s, want %s", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestMergeAndSnapshots(t *testing.T) {
	v := Vector{1, 0, 3}
	snapshot := v.Clone()
	v.Merge(Vector{0, 4, 2})
	v.Tick(0)
	if Compare(v, Vector{2, 4, 3}) != Equal {
		t.Fatal(v)
	}
	if Compare(snapshot, Vector{1, 0, 3}) != Equal {
		t.Fatal("snapshot mutated")
	}
	if LamportEvent(3, 9) != 10 || LamportEvent(9, 3) != 10 {
		t.Fatal("Lamport merge")
	}
}

func TestDimensionMismatch(t *testing.T) {
	for _, fn := range []func(){func() { Compare(Vector{1}, Vector{1, 2}) }, func() { Vector{1}.Merge(Vector{1, 2}) }} {
		func() {
			defer func() {
				if recover() == nil {
					t.Error("expected dimension panic")
				}
			}()
			fn()
		}()
	}
}
