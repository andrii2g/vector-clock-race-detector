// Package detector finds conflicting accesses using vector timestamps.
package detector

import (
	"github.com/andrii2g/vector-clock-race-detector/internal/clock"
	"github.com/andrii2g/vector-clock-race-detector/internal/model"
	"time"
)

// Detect expects a valid simulator trace. It reports all conflicting pairs,
// not just a last-write approximation. Runtime excludes all-pair analysis.
func Detect(trace model.Trace) model.Report {
	r := model.Report{Races: []model.Race{}, Pairs: []model.PairComparison{}}
	start := time.Now()
	accesses := map[string][]model.Event{}
	for _, e := range trace.Events {
		if !e.IsAccess() {
			continue
		}
		for _, prev := range accesses[e.Resource] {
			if prev.Worker == e.Worker || (prev.Kind == model.Read && e.Kind == model.Read) {
				continue
			}
			r.CausalComparisons++
			if clock.Compare(prev.Vector, e.Vector) == clock.Concurrent {
				r.Races = append(r.Races, model.Race{First: prev.ID, Second: e.ID, Resource: e.Resource})
			}
		}
		accesses[e.Resource] = append(accesses[e.Resource], e)
	}
	r.DetectorRuntimeNS = time.Since(start).Nanoseconds()
	r.DetectedRaces = len(r.Races)
	for i, a := range trace.Events {
		for _, b := range trace.Events[i+1:] {
			relation := clock.Compare(a.Vector, b.Vector)
			r.AnalysisComparisons++
			if relation == clock.Concurrent {
				r.ConcurrentPairs++
			}
			r.Pairs = append(r.Pairs, model.PairComparison{First: a.ID, Second: b.ID, Physical: clock.Scalar(a.WallNS, b.WallNS), Lamport: clock.Scalar(a.Lamport, b.Lamport), Vector: relation})
		}
	}
	return r
}
