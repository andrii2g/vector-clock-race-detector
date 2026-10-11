// Package verify constructs a happens-before graph from raw operations.
// Graph construction and race enumeration deliberately do not use clocks.
package verify

import (
	"fmt"
	"github.com/andrii2g/vector-clock-race-detector/internal/clock"
	"github.com/andrii2g/vector-clock-race-detector/internal/model"
)

type Edge struct {
	From int    `json:"from"`
	To   int    `json:"to"`
	Kind string `json:"kind"`
}
type Graph struct {
	Edges []Edge
	Reach [][]bool
}

func Build(s model.Scenario) (Graph, error) {
	g := Graph{Edges: []Edge{}, Reach: make([][]bool, len(s.Operations))}
	if s.Workers < 2 || s.Workers > 4 {
		return g, fmt.Errorf("graph: invalid worker count")
	}
	previous := map[int]int{}
	sends := map[string]int{}
	consumed := map[string]bool{}
	owners := map[string]int{}
	releases := map[string]int{}
	ids := map[string]bool{}
	adj := make([][]int, len(s.Operations))
	add := func(from, to int, kind string) {
		g.Edges = append(g.Edges, Edge{from, to, kind})
		adj[from] = append(adj[from], to)
	}
	for i, op := range s.Operations {
		g.Reach[i] = make([]bool, len(s.Operations))
		if op.Worker < 0 || op.Worker >= s.Workers || op.ID == "" || ids[op.ID] {
			return g, fmt.Errorf("graph: invalid event %q", op.ID)
		}
		ids[op.ID] = true
		if p, ok := previous[op.Worker]; ok {
			add(p, i, "program")
		}
		previous[op.Worker] = i
		switch op.Kind {
		case model.Read, model.Write:
			if op.Resource == "" || op.Message != "" {
				return g, fmt.Errorf("graph: invalid access %q", op.ID)
			}
		case model.Send:
			_, exists := sends[op.Message]
			if op.Message == "" || op.Resource != "" || exists {
				return g, fmt.Errorf("graph: invalid send %q", op.ID)
			}
			sends[op.Message] = i
		case model.Receive:
			p, ok := sends[op.Message]
			if !ok || consumed[op.Message] || op.Resource != "" || s.Operations[p].Worker == op.Worker {
				return g, fmt.Errorf("graph: invalid receive %q", op.ID)
			}
			add(p, i, "message")
			consumed[op.Message] = true
		case model.Acquire:
			if _, held := owners[op.Resource]; held || op.Resource == "" || op.Message != "" {
				return g, fmt.Errorf("graph: invalid acquire %q", op.ID)
			}
			if p, ok := releases[op.Resource]; ok {
				add(p, i, "mutex")
			}
			owners[op.Resource] = op.Worker
		case model.Release:
			owner, held := owners[op.Resource]
			if !held || owner != op.Worker || op.Message != "" {
				return g, fmt.Errorf("graph: invalid release %q", op.ID)
			}
			delete(owners, op.Resource)
			releases[op.Resource] = i
		default:
			return g, fmt.Errorf("graph: unknown kind %q", op.Kind)
		}
	}
	// Iterative DFS from every vertex: independent of vector merge/comparison.
	for source := range adj {
		stack := append([]int(nil), adj[source]...)
		for len(stack) > 0 {
			n := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if g.Reach[source][n] {
				continue
			}
			g.Reach[source][n] = true
			stack = append(stack, adj[n]...)
		}
	}
	return g, nil
}

func Check(s model.Scenario, trace model.Trace, report model.Report) (Graph, error) {
	g, err := Build(s)
	if err != nil {
		return g, err
	}
	if trace.Workers != s.Workers || len(trace.Events) != len(s.Operations) {
		return g, fmt.Errorf("trace shape differs from fixture")
	}
	for i, e := range trace.Events {
		if e.Operation != s.Operations[i] || e.Step != i || len(e.Vector) != s.Workers {
			return g, fmt.Errorf("trace event %d differs from fixture", i)
		}
	}
	wantRaces := map[model.Race]bool{}
	concurrent := 0
	for i, a := range trace.Events {
		for j := i + 1; j < len(trace.Events); j++ {
			b := trace.Events[j]
			want := clock.Concurrent
			if g.Reach[i][j] {
				want = clock.Before
			} else if g.Reach[j][i] {
				want = clock.After
			} else {
				concurrent++
			}
			if got := clock.Compare(a.Vector, b.Vector); got != want {
				return g, fmt.Errorf("%s / %s: vector=%s graph=%s", a.ID, b.ID, got, want)
			}
			if g.Reach[i][j] && a.Lamport >= b.Lamport {
				return g, fmt.Errorf("Lamport invariant violated: %s / %s", a.ID, b.ID)
			}
			// Independently state the race predicate over raw operations and reachability.
			x, y := s.Operations[i], s.Operations[j]
			xAccess := x.Kind == model.Read || x.Kind == model.Write
			yAccess := y.Kind == model.Read || y.Kind == model.Write
			if xAccess && yAccess && x.Worker != y.Worker && x.Resource == y.Resource && (x.Kind == model.Write || y.Kind == model.Write) && !g.Reach[i][j] && !g.Reach[j][i] {
				wantRaces[model.Race{First: x.ID, Second: y.ID, Resource: x.Resource}] = true
			}
		}
	}
	if concurrent != report.ConcurrentPairs {
		return g, fmt.Errorf("concurrent pair count differs: graph=%d detector=%d", concurrent, report.ConcurrentPairs)
	}
	if len(wantRaces) != len(report.Races) || report.DetectedRaces != len(report.Races) {
		return g, fmt.Errorf("race count differs: graph=%d detector=%d", len(wantRaces), len(report.Races))
	}
	for _, race := range report.Races {
		if !wantRaces[race] {
			return g, fmt.Errorf("unexpected or duplicate race: %+v", race)
		}
		delete(wantRaces, race)
	}
	return g, nil
}
