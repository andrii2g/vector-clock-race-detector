package main

import (
	"bytes"
	"encoding/json"
	"github.com/andrii2g/vector-clock-race-detector/internal/model"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunArtifacts(t *testing.T) {
	dir := t.TempDir()
	var output bytes.Buffer
	if err := run([]string{"run", "--scenario", "early-send", "--out", dir}, &output); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var r model.Report
	if err = json.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	if !r.Verified || r.DetectedRaces != 1 || len(r.Pairs) != 6 {
		t.Fatal(r)
	}
	firstTrace, _ := os.ReadFile(filepath.Join(dir, "trace.json"))
	firstSVG, _ := os.ReadFile(filepath.Join(dir, "diagram.svg"))
	if err = run([]string{"run", "--scenario", "early-send", "--out", dir}, &output); err != nil {
		t.Fatal(err)
	}
	secondTrace, _ := os.ReadFile(filepath.Join(dir, "trace.json"))
	secondSVG, _ := os.ReadFile(filepath.Join(dir, "diagram.svg"))
	if !bytes.Equal(firstTrace, secondTrace) || !bytes.Equal(firstSVG, secondSVG) {
		t.Fatal("trace or diagram is nondeterministic")
	}
}

func TestFixtureAndErrors(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "fixture.json")
	valid := `{"name":"custom","workers":2,"operations":[{"id":"a","worker":0,"kind":"write","resource":"x","wall_ns":10},{"id":"b","worker":1,"kind":"read","resource":"x","wall_ns":-5}]}`
	if err := os.WriteFile(fixture, []byte(valid), 0600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := run([]string{"run", "--input", fixture, "--out", filepath.Join(dir, "out")}, &output); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"unknown"}, {"list", "extra"}, {"run", "--scenario", "missing"}, {"run", "--input", fixture, "--scenario", "early-send"},
		{"run", "extra"}, {"verify", "--cases", "0"}, {"verify", "--max-events", "1001"}, {"run", "--input", fixture + "missing"},
	} {
		if err := run(args, &output); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	for _, bad := range []string{`{`, valid + ` {}`, strings.Replace(valid, `"name"`, `"typo"`, 1), strings.Replace(valid, `"workers":2`, `"workers":1`, 1)} {
		if err := os.WriteFile(fixture, []byte(bad), 0600); err != nil {
			t.Fatal(err)
		}
		if err := run([]string{"run", "--input", fixture, "--out", filepath.Join(dir, "invalid")}, &output); err == nil {
			t.Fatal("accepted bad fixture", bad)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "invalid")); !os.IsNotExist(err) {
		t.Fatal("invalid fixture created output")
	}
}

func TestCommands(t *testing.T) {
	for _, args := range [][]string{nil, {"help"}, {"list"}, {"run", "--help"}, {"verify", "--help"}, {"verify", "--seed", "-5", "--cases", "50", "--max-events", "20"}} {
		var out bytes.Buffer
		if err := run(args, &out); err != nil {
			t.Fatal(err)
		}
		if out.Len() == 0 {
			t.Fatal("empty command output")
		}
	}
}

func TestInputCannotBeOverwritten(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trace.json")
	fixture := []byte(`{"name":"source","workers":2,"operations":[]}`)
	if err := os.WriteFile(path, fixture, 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := run([]string{"run", "--input", path, "--out", dir}, &out); err == nil {
		t.Fatal("would overwrite source fixture")
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, fixture) {
		t.Fatal("fixture was changed")
	}
}

func TestShippedFixtures(t *testing.T) {
	for name, want := range map[string]int{"message-passing": 0, "four-workers": 3} {
		dir := t.TempDir()
		var out bytes.Buffer
		if err := run([]string{"run", "--input", filepath.Join("..", "..", "examples", name+".json"), "--out", dir}, &out); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(dir, "report.json"))
		if err != nil {
			t.Fatal(err)
		}
		var r model.Report
		if err = json.Unmarshal(data, &r); err != nil {
			t.Fatal(err)
		}
		if !r.Verified || r.DetectedRaces != want {
			t.Fatal(name, r.DetectedRaces)
		}
	}
}
