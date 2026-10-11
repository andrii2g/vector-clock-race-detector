// Package render creates standalone SVG diagrams without a browser dependency.
package render

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"github.com/andrii2g/vector-clock-race-detector/internal/model"
	"github.com/andrii2g/vector-clock-race-detector/internal/verify"
	"io"
	"strings"
)

func escape(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
func short(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}

// SVG uses schedule steps for vertical position, never physical timestamps.
// Runtime is omitted so diagrams are byte-for-byte reproducible.
func SVG(out io.Writer, tr model.Trace, r model.Report, g verify.Graph) error {
	var b bytes.Buffer
	f := func(format string, args ...any) { fmt.Fprintf(&b, format, args...) }
	width := tr.Workers*340 + 80
	height := len(tr.Events)*105 + 300
	x := func(worker int) int { return 60 + worker*340 }
	y := func(step int) int { return 240 + step*105 }
	f(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" role="img" aria-labelledby="title desc">`, width, height, width, height)
	f(`<title id="title">%s — causality and races</title><desc id="desc">Workers run downward by schedule step, not physical time. Blue arrows are messages; purple dashed arrows are mutex synchronization. Red dashed connectors identify conflicting unordered accesses.</desc>`, escape(tr.Name))
	f(`<defs><marker id="message" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M 0 0 L 10 5 L 0 10 z" fill="#2563eb"/></marker><marker id="mutex" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M 0 0 L 10 5 L 0 10 z" fill="#7c3aed"/></marker><marker id="program" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="5" markerHeight="5" orient="auto"><path d="M 0 0 L 10 5 L 0 10 z" fill="#94a3b8"/></marker></defs>`)
	f(`<style>text{font-family:system-ui,Segoe UI,sans-serif;fill:#0f172a}.mono{font-family:Consolas,monospace}.muted{fill:#475569}.small{font-size:12px}.label{font-size:14px;font-weight:600}</style>`)
	f(`<rect width="100%%" height="100%%" fill="#f8fafc"/><rect x="0" y="0" width="%d" height="170" fill="#e2e8f0"/>`, width)
	f(`<text x="28" y="34" font-size="12" letter-spacing="2" class="muted">VECTOR CLOCK / RACE DETECTOR</text>`)
	f(`<text x="28" y="68" font-size="25" font-weight="700">%s</text>`, escape(short(tr.Name, (width-70)/15)))
	f(`<text x="28" y="96" font-size="14">%d workers · %d events · %d concurrent pairs · %d races · graph verified: %t</text>`, tr.Workers, len(tr.Events), r.ConcurrentPairs, r.DetectedRaces, r.Verified)
	f(`<text x="28" y="122" class="small muted">Downward = schedule order, not elapsed time. Numeric clock order alone does not prove causality.</text>`)
	f(`<path d="M28 146 h24" stroke="#2563eb" stroke-width="2" marker-end="url(#message)"/><text x="60" y="150" class="small">Message</text><path d="M145 146 h24" stroke="#7c3aed" stroke-width="2" stroke-dasharray="5 3"/><text x="178" y="150" class="small">Mutex</text><path d="M250 146 h24" stroke="#dc2626" stroke-width="2" stroke-dasharray="6 4"/><text x="282" y="150" class="small">Race (unordered)</text><text x="435" y="150" class="small muted">V = vector · L = Lamport · wall = fixture ns</text>`)
	for w := 0; w < tr.Workers; w++ {
		f(`<text x="%d" y="200" class="label">WORKER %d</text><line x1="%d" y1="214" x2="%d" y2="%d" stroke="#cbd5e1" stroke-width="2"/>`, x(w), w, x(w), x(w), height-35)
	}
	for _, edge := range g.Edges {
		a, b := tr.Events[edge.From], tr.Events[edge.To]
		color := "#94a3b8"
		dash := ""
		if edge.Kind == "message" {
			color = "#2563eb"
		}
		if edge.Kind == "mutex" {
			color = "#7c3aed"
			dash = ` stroke-dasharray="5 3"`
		}
		f(`<path d="M%d %d C%d %d %d %d %d %d" fill="none" stroke="%s" stroke-width="2"%s marker-end="url(#%s)"><title>%s: %s → %s</title></path>`, x(a.Worker), y(a.Step)+7, x(a.Worker), y(a.Step)+55, x(b.Worker), y(b.Step)-55, x(b.Worker), y(b.Step)-9, color, dash, edge.Kind, escape(edge.Kind), escape(a.ID), escape(b.ID))
	}
	byID := map[string]model.Event{}
	racing := map[string]bool{}
	for _, e := range tr.Events {
		byID[e.ID] = e
	}
	for _, race := range r.Races {
		a, b := byID[race.First], byID[race.Second]
		racing[a.ID] = true
		racing[b.ID] = true
		f(`<path d="M%d %d C%d %d %d %d %d %d" fill="none" stroke="#dc2626" stroke-width="2" stroke-dasharray="6 4"><title>Race on %s: %s ↔ %s (causally unordered)</title></path>`, x(a.Worker), y(a.Step), x(a.Worker)-25, y(a.Step)+60, x(b.Worker)-25, y(b.Step)-60, x(b.Worker), y(b.Step), escape(race.Resource), escape(a.ID), escape(b.ID))
	}
	for _, e := range tr.Events {
		border := "#cbd5e1"
		fill := "#ffffff"
		dot := "#0f766e"
		if racing[e.ID] {
			border = "#fca5a5"
			fill = "#fff1f2"
			dot = "#dc2626"
		}
		target := e.Resource
		if target == "" {
			target = e.Message
		}
		label := fmt.Sprintf("%s · %s %s", e.ID, e.Kind, target)
		vector := strings.ReplaceAll(fmt.Sprint(e.Vector), " ", ", ")
		f(`<g><title>%s; vector %s; Lamport %d; wall %d ns</title>`, escape(label), escape(vector), e.Lamport, e.WallNS)
		f(`<circle cx="%d" cy="%d" r="6" fill="%s"/><rect x="%d" y="%d" width="265" height="78" rx="8" fill="%s" stroke="%s"/>`, x(e.Worker), y(e.Step), dot, x(e.Worker)+16, y(e.Step)-26, fill, border)
		f(`<text x="%d" y="%d" class="label">%s</text><text x="%d" y="%d" class="mono" font-size="14">V %s</text><text x="%d" y="%d" class="small muted">L %d · wall %d ns</text></g>`, x(e.Worker)+27, y(e.Step)-6, escape(short(label, 29)), x(e.Worker)+27, y(e.Step)+16, escape(vector), x(e.Worker)+27, y(e.Step)+37, e.Lamport, e.WallNS)
	}
	f(`</svg>`)
	_, err := out.Write(b.Bytes())
	return err
}
