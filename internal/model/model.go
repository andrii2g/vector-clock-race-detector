// Package model contains the shared trace format, not causal algorithms.
package model

import "github.com/andrii2g/vector-clock-race-detector/internal/clock"

type Kind string

const (
	Read    Kind = "read"
	Write   Kind = "write"
	Send    Kind = "send"
	Receive Kind = "receive"
	Acquire Kind = "acquire"
	Release Kind = "release"
)

// Operations are listed in execution order; their per-worker subsequence is
// that worker's program. WallNS is metadata, never a scheduling constraint.
type Operation struct {
	ID       string `json:"id"`
	Worker   int    `json:"worker"`
	Kind     Kind   `json:"kind"`
	Resource string `json:"resource,omitempty"`
	Message  string `json:"message,omitempty"`
	WallNS   int64  `json:"wall_ns"`
}

type Scenario struct {
	Name       string      `json:"name"`
	Workers    int         `json:"workers"`
	Operations []Operation `json:"operations"`
}

type Event struct {
	Operation
	Step    int          `json:"step"`
	Lamport uint64       `json:"lamport"`
	Vector  clock.Vector `json:"vector"`
}

type ClockMetrics struct {
	PhysicalAssignments       int `json:"physical_assignments"`
	LamportIncrements         int `json:"lamport_increments"`
	LamportMerges             int `json:"lamport_merges"`
	VectorIncrements          int `json:"vector_increments"`
	VectorMerges              int `json:"vector_merges"`
	VectorComponentOperations int `json:"vector_component_operations"`
}

type SyncMetrics struct {
	Sends    int `json:"sends"`
	Receives int `json:"receives"`
	Acquires int `json:"acquires"`
	Releases int `json:"releases"`
}

type Trace struct {
	Name            string       `json:"name"`
	Workers         int          `json:"workers"`
	Events          []Event      `json:"events"`
	Clocks          ClockMetrics `json:"clock_operations"`
	Synchronization SyncMetrics  `json:"synchronization_events"`
}

func (op Operation) IsAccess() bool { return op.Kind == Read || op.Kind == Write }

type Race struct {
	First    string `json:"first"`
	Second   string `json:"second"`
	Resource string `json:"resource"`
}

type PairComparison struct {
	First    string         `json:"first"`
	Second   string         `json:"second"`
	Physical clock.Relation `json:"physical_numeric_order"`
	Lamport  clock.Relation `json:"lamport_numeric_order"`
	Vector   clock.Relation `json:"vector_causal_order"`
}

type Report struct {
	Races               []Race           `json:"races"`
	DetectedRaces       int              `json:"detected_races"`
	ConcurrentPairs     int              `json:"concurrent_event_pairs"`
	CausalComparisons   int              `json:"detector_causal_comparisons"`
	AnalysisComparisons int              `json:"all_pair_causal_comparisons"`
	DetectorRuntimeNS   int64            `json:"detector_runtime_ns"`
	Verified            bool             `json:"verified"`
	Pairs               []PairComparison `json:"pairs"`
}
