package verify_test

import (
	"github.com/andrii2g/vector-clock-race-detector/internal/detector"
	"github.com/andrii2g/vector-clock-race-detector/internal/model"
	"github.com/andrii2g/vector-clock-race-detector/internal/scenario"
	"github.com/andrii2g/vector-clock-race-detector/internal/simulator"
	"github.com/andrii2g/vector-clock-race-detector/internal/verify"
	"testing"
)

func TestGraphKnownReachability(t *testing.T) {
	// This expected graph is hand-derived without constructing any clocks.
	s := model.Scenario{Workers: 4, Operations: []model.Operation{
		{ID: "a", Worker: 0, Kind: model.Write, Resource: "x"},
		{ID: "s", Worker: 0, Kind: model.Send, Message: "m"},
		{ID: "late", Worker: 0, Kind: model.Write, Resource: "x"},
		{ID: "r", Worker: 1, Kind: model.Receive, Message: "m"},
		{ID: "l1", Worker: 1, Kind: model.Acquire, Resource: "mu"},
		{ID: "u1", Worker: 1, Kind: model.Release, Resource: "mu"},
		{ID: "l2", Worker: 2, Kind: model.Acquire, Resource: "mu"},
		{ID: "b", Worker: 2, Kind: model.Read, Resource: "x"},
		{ID: "independent", Worker: 3, Kind: model.Write, Resource: "x"},
	}}
	g, err := verify.Build(s)
	if err != nil {
		t.Fatal(err)
	}
	want := map[[2]int]bool{}
	for _, chain := range [][]int{{0, 1, 2}, {0, 1, 3, 4, 5, 6, 7}} {
		for i, a := range chain {
			for _, b := range chain[i+1:] {
				want[[2]int{a, b}] = true
			}
		}
	}
	for i := range g.Reach {
		for j, got := range g.Reach[i] {
			if got != want[[2]int{i, j}] {
				t.Fatalf("reach(%d,%d)=%t", i, j, got)
			}
		}
	}
	if len(g.Edges) != 7 {
		t.Fatalf("got %d direct edges", len(g.Edges))
	}
}

func TestVerifierRejectsTampering(t *testing.T) {
	for _, mutate := range []func(*model.Trace, *model.Report){
		func(tr *model.Trace, r *model.Report) { tr.Workers = 4 },
		func(tr *model.Trace, r *model.Report) { tr.Events[0].Step = 99 },
		func(tr *model.Trace, r *model.Report) { tr.Events[0].Vector = nil },
		func(tr *model.Trace, r *model.Report) { tr.Events[1].Lamport = 0 },
		func(tr *model.Trace, r *model.Report) { r.ConcurrentPairs++ },
		func(tr *model.Trace, r *model.Report) { r.Races[0].Resource = "invented" },
	} {
		s, _ := scenario.Builtin("independent-writes")
		tr, _ := simulator.Run(s)
		r := detector.Detect(tr)
		mutate(&tr, &r)
		if _, err := verify.Check(s, tr, r); err == nil {
			t.Fatal("accepted corrupted result")
		}
	}
}
