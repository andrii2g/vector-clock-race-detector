// Command vcrace simulates and independently verifies causal histories.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/andrii2g/vector-clock-race-detector/internal/detector"
	"github.com/andrii2g/vector-clock-race-detector/internal/model"
	"github.com/andrii2g/vector-clock-race-detector/internal/render"
	"github.com/andrii2g/vector-clock-race-detector/internal/scenario"
	"github.com/andrii2g/vector-clock-race-detector/internal/simulator"
	"github.com/andrii2g/vector-clock-race-detector/internal/verify"
	"io"
	"os"
	"path/filepath"
)

const maxEvents = 1000 // Reports, oracle reachability, and pair enumeration are quadratic.

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	if len(args) == 0 {
		return usage(out)
	}
	switch args[0] {
	case "help", "-h", "--help":
		return usage(out)
	case "list":
		if len(args) != 1 {
			return fmt.Errorf("list takes no arguments")
		}
		for _, name := range scenario.Names() {
			fmt.Fprintln(out, name)
		}
		return nil
	case "run":
		return runScenario(args[1:], out)
	case "verify":
		return verifyGenerated(args[1:], out)
	default:
		return fmt.Errorf("unknown command %q; use vcrace help", args[0])
	}
}

func usage(out io.Writer) error {
	_, err := fmt.Fprintln(out, `vcrace — deterministic vector-clock race detector

Commands:
  list
  run [-scenario independent-writes | -input fixture.json] [-out artifacts]
  verify [-seed 42] [-cases 1000] [-max-events 30]

Every run records physical, Lamport and vector clocks and checks a graph oracle.
Artifacts: trace.json, report.json, diagram.svg. Races are findings, not errors.
The operations array is the schedule; wall_ns never controls execution.
Run output files are replaced if they already exist.`)
	return err
}

func flags(name string, out io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(out)
	return fs
}

func loadFixture(path string) (model.Scenario, error) {
	var s model.Scenario
	f, err := os.Open(path)
	if err != nil {
		return s, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return s, err
	}
	if info.Size() > 2*1024*1024 {
		return s, fmt.Errorf("fixture exceeds 2 MiB limit")
	}
	dec := json.NewDecoder(io.LimitReader(f, 2*1024*1024+1))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&s); err != nil {
		return s, fmt.Errorf("decode fixture: %w", err)
	}
	var extra any
	if err = dec.Decode(&extra); err != io.EOF {
		return s, fmt.Errorf("fixture must contain exactly one JSON object")
	}
	return s, nil
}

func runScenario(args []string, out io.Writer) error {
	fs := flags("run", out)
	name := fs.String("scenario", "independent-writes", "built-in scenario name")
	input := fs.String("input", "", "JSON fixture path")
	dir := fs.String("out", "artifacts", "output directory")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	selected := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "scenario" {
			selected = true
		}
	})
	if *input != "" && selected {
		return fmt.Errorf("choose either -scenario or -input")
	}
	var s model.Scenario
	var err error
	if *input != "" {
		s, err = loadFixture(*input)
	} else {
		s, err = scenario.Builtin(*name)
	}
	if err != nil {
		return err
	}
	if len(s.Operations) > maxEvents {
		return fmt.Errorf("history exceeds %d-event limit", maxEvents)
	}
	tr, err := simulator.Run(s)
	if err != nil {
		return err
	}
	r := detector.Detect(tr)
	g, err := verify.Check(s, tr, r)
	if err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}
	r.Verified = true
	var svg bytes.Buffer
	if err = render.SVG(&svg, tr, r, g); err != nil {
		return err
	}
	traceJSON, err := json.MarshalIndent(tr, "", "  ")
	if err != nil {
		return err
	}
	reportJSON, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(*dir, 0755); err != nil {
		return err
	}
	for _, file := range []struct {
		name string
		data []byte
	}{{"trace.json", append(traceJSON, '\n')}, {"report.json", append(reportJSON, '\n')}, {"diagram.svg", svg.Bytes()}} {
		if err = os.WriteFile(filepath.Join(*dir, file.name), file.data, 0644); err != nil {
			return err
		}
	}
	fmt.Fprintf(out, "%s: %d events, %d concurrent pairs, %d races; graph verified\n", s.Name, len(tr.Events), r.ConcurrentPairs, r.DetectedRaces)
	for _, race := range r.Races {
		fmt.Fprintf(out, "  race on %s: %s <-> %s\n", race.Resource, race.First, race.Second)
	}
	fmt.Fprintf(out, "Detector: %d comparisons, %d ns (runtime varies)\nArtifacts: %s\n", r.CausalComparisons, r.DetectorRuntimeNS, *dir)
	return nil
}

func verifyGenerated(args []string, out io.Writer) error {
	fs := flags("verify", out)
	seed := fs.Int64("seed", 42, "first history seed")
	cases := fs.Int("cases", 1000, "number of histories")
	n := fs.Int("max-events", 30, "maximum events per history")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 || *cases < 1 || *cases > 100000 || *n < 1 || *n > maxEvents {
		return fmt.Errorf("require 1-100000 cases, 1-%d max-events, and no positional arguments", maxEvents)
	}
	totalEvents, totalRaces := 0, 0
	for i := 0; i < *cases; i++ {
		// Cycle through lengths and worker counts to cover empty histories and
		// every worker count even in short verification runs.
		s, err := scenario.Generate(*seed+int64(i), 2+i%3, i%(*n+1))
		if err != nil {
			return err
		}
		tr, err := simulator.Run(s)
		if err != nil {
			return fmt.Errorf("seed %d: %w", *seed+int64(i), err)
		}
		r := detector.Detect(tr)
		if _, err = verify.Check(s, tr, r); err != nil {
			return fmt.Errorf("seed %d workers %d events %d: %w", *seed+int64(i), s.Workers, len(s.Operations), err)
		}
		totalEvents += len(tr.Events)
		totalRaces += r.DetectedRaces
	}
	fmt.Fprintf(out, "Verified %d histories (seed %d, workers 2-4, up to %d events): %d events, %d race pairs; zero disagreements\n", *cases, *seed, *n, totalEvents, totalRaces)
	return nil
}
