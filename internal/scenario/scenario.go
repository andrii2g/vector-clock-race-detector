// Package scenario provides reproducible teaching examples and generated histories.
package scenario

import (
	"fmt"
	"github.com/andrii2g/vector-clock-race-detector/internal/model"
	"math/rand"
)

func Names() []string {
	return []string{"independent-writes", "synchronized-writes", "message-passing", "early-send", "misleading-wall-clock", "different-mutexes", "concurrent-reads", "transitive-chain"}
}

func Builtin(name string) (model.Scenario, error) {
	s := model.Scenario{Name: name, Workers: 2, Operations: []model.Operation{}}
	add := func(w int, k model.Kind, target string) {
		i := len(s.Operations)
		op := model.Operation{ID: fmt.Sprintf("e%d", i), Worker: w, Kind: k, WallNS: int64(i+1) * 1_000_000}
		if k == model.Send || k == model.Receive {
			op.Message = target
		} else {
			op.Resource = target
		}
		s.Operations = append(s.Operations, op)
	}
	switch name {
	case "independent-writes":
		add(0, model.Read, "y")
		add(0, model.Write, "x")
		add(1, model.Write, "x")
	case "synchronized-writes":
		add(0, model.Acquire, "mu")
		add(0, model.Write, "x")
		add(0, model.Release, "mu")
		add(1, model.Acquire, "mu")
		add(1, model.Write, "x")
		add(1, model.Release, "mu")
	case "message-passing", "misleading-wall-clock":
		add(0, model.Write, "x")
		add(0, model.Send, "m")
		add(1, model.Receive, "m")
		add(1, model.Read, "x")
		if name == "misleading-wall-clock" {
			for i := range s.Operations {
				s.Operations[i].WallNS = int64(10-i) * 1_000_000
			}
		}
	case "early-send":
		add(0, model.Send, "m")
		add(0, model.Write, "x")
		add(1, model.Receive, "m")
		add(1, model.Read, "x")
	case "different-mutexes":
		add(0, model.Acquire, "mu0")
		add(0, model.Write, "x")
		add(0, model.Release, "mu0")
		add(1, model.Acquire, "mu1")
		add(1, model.Write, "x")
		add(1, model.Release, "mu1")
	case "concurrent-reads":
		add(0, model.Read, "x")
		add(1, model.Read, "x")
	case "transitive-chain":
		s.Workers = 3
		add(0, model.Write, "x")
		add(0, model.Send, "m0")
		add(1, model.Receive, "m0")
		add(1, model.Send, "m1")
		add(2, model.Receive, "m1")
		add(2, model.Read, "x")
	default:
		return s, fmt.Errorf("unknown scenario %q", name)
	}
	return s, nil
}

// Generate selects only enabled operations. A history is a finite prefix:
// unreceived messages and held mutexes at the end are permitted.
func Generate(seed int64, workers, events int) (model.Scenario, error) {
	s := model.Scenario{Name: fmt.Sprintf("generated-%d", seed), Workers: workers, Operations: []model.Operation{}}
	if workers < 2 || workers > 4 || events < 0 {
		return s, fmt.Errorf("generation requires 2-4 workers and nonnegative events")
	}
	rng := rand.New(rand.NewSource(seed))
	type pendingMessage struct {
		id     string
		sender int
	}
	pending := []pendingMessage{}
	owners := [2]int{-1, -1}
	nextMessage := 0
	for i := 0; i < events; i++ {
		choices := []model.Operation{}
		for w := 0; w < workers; w++ {
			for _, resource := range []string{"x", "y"} {
				for _, kind := range []model.Kind{model.Read, model.Write} {
					choices = append(choices, model.Operation{Worker: w, Kind: kind, Resource: resource})
				}
			}
			choices = append(choices, model.Operation{Worker: w, Kind: model.Send, Message: fmt.Sprintf("m%d", nextMessage)})
			for _, m := range pending {
				if m.sender != w {
					choices = append(choices, model.Operation{Worker: w, Kind: model.Receive, Message: m.id})
				}
			}
			for l, owner := range owners {
				if owner == -1 {
					choices = append(choices, model.Operation{Worker: w, Kind: model.Acquire, Resource: fmt.Sprintf("mu%d", l)})
				}
				if owner == w {
					choices = append(choices, model.Operation{Worker: w, Kind: model.Release, Resource: fmt.Sprintf("mu%d", l)})
				}
			}
		}
		op := choices[rng.Intn(len(choices))]
		op.ID = fmt.Sprintf("e%d", i)
		// Includes ties, negative offsets and reversals, unrelated to scheduling.
		op.WallNS = int64(rng.Intn(21)-10) * 1_000_000
		switch op.Kind {
		case model.Send:
			pending = append(pending, pendingMessage{op.Message, op.Worker})
			nextMessage++
		case model.Receive:
			for j, m := range pending {
				if m.id == op.Message {
					pending = append(pending[:j], pending[j+1:]...)
					break
				}
			}
		case model.Acquire, model.Release:
			l := 0
			if op.Resource == "mu1" {
				l = 1
			}
			if op.Kind == model.Acquire {
				owners[l] = op.Worker
			} else {
				owners[l] = -1
			}
		}
		s.Operations = append(s.Operations, op)
	}
	return s, nil
}
