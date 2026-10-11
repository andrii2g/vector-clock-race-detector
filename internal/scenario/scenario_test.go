package scenario_test

import (
	"github.com/andrii2g/vector-clock-race-detector/internal/clock"
	"github.com/andrii2g/vector-clock-race-detector/internal/detector"
	"github.com/andrii2g/vector-clock-race-detector/internal/model"
	"github.com/andrii2g/vector-clock-race-detector/internal/scenario"
	"github.com/andrii2g/vector-clock-race-detector/internal/simulator"
	"github.com/andrii2g/vector-clock-race-detector/internal/verify"
	"reflect"
	"testing"
)

func check(t *testing.T, s model.Scenario) model.Report {
	t.Helper()
	tr, err := simulator.Run(s)
	if err != nil {
		t.Fatal(err)
	}
	r := detector.Detect(tr)
	if _, err := verify.Check(s, tr, r); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestBuiltinExpectations(t *testing.T) {
	want := map[string]int{"independent-writes": 1, "synchronized-writes": 0, "message-passing": 0, "early-send": 1, "misleading-wall-clock": 0, "different-mutexes": 1, "concurrent-reads": 0, "transitive-chain": 0}
	for _, name := range scenario.Names() {
		t.Run(name, func(t *testing.T) {
			s, err := scenario.Builtin(name)
			if err != nil {
				t.Fatal(err)
			}
			r := check(t, s)
			if r.DetectedRaces != want[name] {
				t.Fatalf("got %d races", r.DetectedRaces)
			}
			if name == "misleading-wall-clock" {
				for _, p := range r.Pairs {
					if p.Physical != clock.After || p.Vector != clock.Before {
						t.Fatal(p)
					}
				}
			}
		})
	}
}

func TestGeneratedHistories(t *testing.T) {
	seen := map[model.Kind]bool{}
	for seed := int64(0); seed < 500; seed++ {
		s, err := scenario.Generate(seed, 2+int(seed%3), int(seed%41))
		if err != nil {
			t.Fatal(err)
		}
		check(t, s)
		for _, op := range s.Operations {
			seen[op.Kind] = true
		}
		again, _ := scenario.Generate(seed, s.Workers, len(s.Operations))
		if !reflect.DeepEqual(s, again) {
			t.Fatal("nondeterministic generator")
		}
	}
	if len(seen) != 6 {
		t.Fatalf("generator did not exercise every operation: %v", seen)
	}
}

func FuzzGraphAgreement(f *testing.F) {
	f.Add(int64(42), uint8(2), uint8(20))
	f.Add(int64(-7), uint8(4), uint8(50))
	f.Fuzz(func(t *testing.T, seed int64, w, n uint8) {
		s, _ := scenario.Generate(seed, 2+int(w%3), int(n%65))
		check(t, s)
	})
}

func BenchmarkDetectionAndPairAnalysis(b *testing.B) {
	s, _ := scenario.Generate(42, 4, 100)
	tr, _ := simulator.Run(s)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detector.Detect(tr)
	}
}
