// Package simulator executes fixture schedules without physical clock reads or goroutines.
package simulator

import (
	"fmt"
	"github.com/andrii2g/vector-clock-race-detector/internal/clock"
	"github.com/andrii2g/vector-clock-race-detector/internal/model"
)

type stamp struct {
	vector clock.Vector
	scalar uint64
}
type message struct {
	stamp
	sender   int
	received bool
}
type mutex struct {
	stamp
	owner int
}

func Run(s model.Scenario) (model.Trace, error) {
	trace := model.Trace{Name: s.Name, Workers: s.Workers, Events: make([]model.Event, 0, len(s.Operations))}
	if s.Workers < 2 || s.Workers > 4 {
		return trace, fmt.Errorf("workers must be between 2 and 4")
	}
	workers := make([]stamp, s.Workers)
	for i := range workers {
		workers[i].vector = make(clock.Vector, s.Workers)
	}
	messages := map[string]*message{}
	locks := map[string]*mutex{}
	ids := map[string]bool{}
	for step, op := range s.Operations {
		fail := func(reason string) (model.Trace, error) {
			return trace, fmt.Errorf("step %d event %q: %s", step, op.ID, reason)
		}
		if op.ID == "" || ids[op.ID] {
			return fail("event ID must be nonempty and unique")
		}
		ids[op.ID] = true
		if op.Worker < 0 || op.Worker >= s.Workers {
			return fail("worker out of range")
		}
		w := &workers[op.Worker]
		var incoming *stamp
		switch op.Kind {
		case model.Read, model.Write, model.Acquire, model.Release:
			if op.Resource == "" || op.Message != "" {
				return fail("memory/mutex operation requires resource and no message")
			}
		case model.Send, model.Receive:
			if op.Message == "" || op.Resource != "" {
				return fail("message operation requires message and no resource")
			}
		default:
			return fail("unknown operation kind")
		}
		switch op.Kind {
		case model.Send:
			if messages[op.Message] != nil {
				return fail("message already sent")
			}
			trace.Synchronization.Sends++
		case model.Receive:
			msg := messages[op.Message]
			if msg == nil {
				return fail("receive has no preceding send")
			}
			if msg.received {
				return fail("message already received")
			}
			if msg.sender == op.Worker {
				return fail("message must cross workers")
			}
			incoming = &msg.stamp
			msg.received = true
			trace.Synchronization.Receives++
		case model.Acquire:
			lock := locks[op.Resource]
			if lock == nil {
				lock = &mutex{stamp: stamp{vector: make(clock.Vector, s.Workers)}, owner: -1}
				locks[op.Resource] = lock
			}
			if lock.owner != -1 {
				return fail("mutex is held; acquire would block")
			}
			incoming = &lock.stamp
			lock.owner = op.Worker
			trace.Synchronization.Acquires++
		case model.Release:
			lock := locks[op.Resource]
			if lock == nil || lock.owner != op.Worker {
				return fail("release by worker that does not own mutex")
			}
			trace.Synchronization.Releases++
		}
		var scalar uint64
		if incoming != nil {
			w.vector.Merge(incoming.vector)
			scalar = incoming.scalar
			trace.Clocks.VectorMerges++
			trace.Clocks.LamportMerges++
			trace.Clocks.VectorComponentOperations += s.Workers
		}
		w.vector.Tick(op.Worker)
		w.scalar = clock.LamportEvent(w.scalar, scalar)
		trace.Clocks.VectorIncrements++
		trace.Clocks.LamportIncrements++
		trace.Clocks.VectorComponentOperations++
		trace.Clocks.PhysicalAssignments++
		snapshot := stamp{vector: w.vector.Clone(), scalar: w.scalar}
		switch op.Kind {
		case model.Send:
			messages[op.Message] = &message{stamp: snapshot, sender: op.Worker}
		case model.Release:
			locks[op.Resource].stamp = snapshot
			locks[op.Resource].owner = -1
		}
		trace.Events = append(trace.Events, model.Event{Operation: op, Step: step, Lamport: w.scalar, Vector: w.vector.Clone()})
	}
	return trace, nil
}
