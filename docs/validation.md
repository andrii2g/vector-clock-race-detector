# Validation record

Validated on 2026-10-11 with Go 1.26.3, Windows/amd64. The module targets Go 1.22.

| Check | Result |
| --- | --- |
| Unit, integration, and regression tests | Passed |
| `go vet ./...` | Passed |
| CLI binary build | Passed |
| Cross-package statement coverage | 94.9% |
| Seeded verification, seed 42, 10,000 histories, at most 64 events | 319,725 events; 407,841 race pairs; zero graph/vector disagreements |
| Ten-second fuzz run with two workers | 72,478 executions; passed |
| Both shipped JSON examples | Graph verified; expected 0 and 3 race pairs |
| SVG validation | XML parsing and escaping tests passed; independent-writes diagram visually inspected in headless Chrome |
| Reproducibility | Repeated trace JSON and SVG match byte-for-byte |
| Go `-race` instrumentation | Not available: cgo disabled and no C compiler installed |

The simulator's own race detection and graph verification do not depend on cgo.
`go test -race` would check the Go implementation's host-memory accesses, not the
simulated history's causality.

Reproduce the main checks from the repository root:

```sh
go test -count=1 ./...
go vet ./...
go build -o artifacts/vcrace ./cmd/vcrace
go test '-coverpkg=./...' '-coverprofile=artifacts/coverage.out' ./...
go tool cover '-func=artifacts/coverage.out'
go run ./cmd/vcrace verify --seed 42 --cases 10000 --max-events 64
go test ./internal/scenario -fuzz=FuzzGraphAgreement -fuzztime=10s -parallel=2
go test ./internal/scenario '-run=^$' '-bench=.' -benchmem
```

Use `artifacts/vcrace.exe` as the binary output path on Windows. Quoted flags keep
PowerShell from splitting dotted argument values. Fuzz counts, timings, coverage
after source changes, and benchmark results may vary. The benchmark includes
all-pair reporting; `detector_runtime_ns` in each report excludes that work.
