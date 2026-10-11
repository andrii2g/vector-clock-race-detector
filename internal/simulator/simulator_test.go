package simulator_test

import (
	"github.com/andrii2g/vector-clock-race-detector/internal/clock"
	"github.com/andrii2g/vector-clock-race-detector/internal/detector"
	"github.com/andrii2g/vector-clock-race-detector/internal/model"
	"github.com/andrii2g/vector-clock-race-detector/internal/simulator"
	"github.com/andrii2g/vector-clock-race-detector/internal/verify"
	"testing"
)

func access(id string, w int, k model.Kind) model.Operation {
	return model.Operation{ID: id, Worker: w, Kind: k, Resource: "x"}
}
func msg(id string, w int, k model.Kind) model.Operation {
	return model.Operation{ID: id, Worker: w, Kind: k, Message: "m"}
}
func lock(id string, w int, k model.Kind) model.Operation {
	return model.Operation{ID: id, Worker: w, Kind: k, Resource: "mu"}
}

func TestHistories(t *testing.T) {
	for _, tc := range []struct {
		name  string
		ops   []model.Operation
		races int
	}{
		{"independent", []model.Operation{access("a", 0, model.Write), access("b", 1, model.Write)}, 1},
		{"reads", []model.Operation{access("a", 0, model.Read), access("b", 1, model.Read)}, 0},
		{"same worker", []model.Operation{access("a", 0, model.Write), access("b", 0, model.Read)}, 0},
		{"message", []model.Operation{access("a", 0, model.Write), msg("s", 0, model.Send), msg("r", 1, model.Receive), access("b", 1, model.Read)}, 0},
		{"early send", []model.Operation{msg("s", 0, model.Send), access("a", 0, model.Write), msg("r", 1, model.Receive), access("b", 1, model.Read)}, 1},
		{"mutex", []model.Operation{lock("l0", 0, model.Acquire), access("a", 0, model.Write), lock("u0", 0, model.Release), lock("l1", 1, model.Acquire), access("b", 1, model.Write), lock("u1", 1, model.Release)}, 0},
		{"all old writes", []model.Operation{access("a", 0, model.Write), access("b", 0, model.Write), access("c", 1, model.Read)}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := model.Scenario{Workers: 2, Operations: tc.ops}
			tr, err := simulator.Run(s)
			if err != nil {
				t.Fatal(err)
			}
			r := detector.Detect(tr)
			if r.DetectedRaces != tc.races {
				t.Fatalf("races=%d want=%d", r.DetectedRaces, tc.races)
			}
			if _, err := verify.Check(s, tr, r); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestExactMessageClocks(t *testing.T) {
	s := model.Scenario{Workers: 2, Operations: []model.Operation{access("a", 0, model.Write), msg("s", 0, model.Send), msg("r", 1, model.Receive), access("b", 1, model.Read)}}
	tr, err := simulator.Run(s)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []clock.Vector{{1, 0}, {2, 0}, {2, 1}, {2, 2}} {
		if clock.Compare(tr.Events[i].Vector, want) != clock.Equal {
			t.Fatal(tr.Events[i])
		}
	}
	if tr.Clocks.VectorMerges != 1 || tr.Clocks.VectorComponentOperations != 6 || tr.Events[3].Lamport != 4 {
		t.Fatal(tr)
	}
	// Mutating one recorded event must not affect other snapshots.
	tr.Events[1].Vector[0] = 999
	if tr.Events[2].Vector[0] != 2 {
		t.Fatal("aliased timestamps")
	}
}

func TestInvalidSchedules(t *testing.T) {
	for _, ops := range [][]model.Operation{
		{msg("r", 1, model.Receive)},
		{msg("s", 0, model.Send), msg("s2", 1, model.Send)},
		{msg("s", 0, model.Send), msg("r", 1, model.Receive), msg("r2", 1, model.Receive)},
		{msg("s", 0, model.Send), msg("r", 0, model.Receive)},
		{lock("l", 0, model.Acquire), lock("l2", 1, model.Acquire)},
		{lock("l", 0, model.Acquire), lock("u", 1, model.Release)},
		{lock("u", 0, model.Release)},
		{access("a", 2, model.Read)},
		{access("a", 0, model.Read), access("a", 1, model.Write)},
		{{ID: "x", Kind: "unknown"}},
		{{ID: "x", Kind: model.Write}},
	} {
		s := model.Scenario{Workers: 2, Operations: ops}
		if _, err := simulator.Run(s); err == nil {
			t.Fatalf("accepted invalid schedule: %+v", ops)
		}
		if _, err := verify.Build(s); err == nil {
			t.Fatalf("oracle accepted invalid schedule: %+v", ops)
		}
	}
}

func TestVerifierRejectsCorruption(t *testing.T) {
	s := model.Scenario{Workers: 2, Operations: []model.Operation{access("a", 0, model.Write), access("b", 1, model.Write)}}
	tr, _ := simulator.Run(s)
	r := detector.Detect(tr)
	tr.Events[1].Vector = clock.Vector{1, 1}
	if _, err := verify.Check(s, tr, r); err == nil {
		t.Fatal("accepted fabricated causality")
	}
	tr, _ = simulator.Run(s)
	r.Races = nil
	r.DetectedRaces = 0
	if _, err := verify.Check(s, tr, r); err == nil {
		t.Fatal("accepted missing race")
	}
}
