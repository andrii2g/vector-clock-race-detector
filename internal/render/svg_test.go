package render_test

import (
	"bytes"
	"encoding/xml"
	"errors"
	"github.com/andrii2g/vector-clock-race-detector/internal/detector"
	"github.com/andrii2g/vector-clock-race-detector/internal/render"
	"github.com/andrii2g/vector-clock-race-detector/internal/scenario"
	"github.com/andrii2g/vector-clock-race-detector/internal/simulator"
	"github.com/andrii2g/vector-clock-race-detector/internal/verify"
	"io"
	"strings"
	"testing"
)

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("write failure") }

func TestSVGValidEscapedAndComplete(t *testing.T) {
	s, _ := scenario.Builtin("independent-writes")
	s.Name = `<script>alert("x")</script> & demo`
	s.Operations[1].Resource = "<&x>"
	s.Operations[2].Resource = "<&x>"
	tr, _ := simulator.Run(s)
	r := detector.Detect(tr)
	g, err := verify.Check(s, tr, r)
	if err != nil {
		t.Fatal(err)
	}
	r.Verified = true
	var out bytes.Buffer
	if err = render.SVG(&out, tr, r, g); err != nil {
		t.Fatal(err)
	}
	dec := xml.NewDecoder(bytes.NewReader(out.Bytes()))
	circles := 0
	titles := 0
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if elem, ok := tok.(xml.StartElement); ok {
			if elem.Name.Local == "script" {
				t.Fatal("SVG injection")
			}
			if elem.Name.Local == "circle" {
				circles++
			}
			if elem.Name.Local == "title" {
				titles++
			}
		}
	}
	if circles != 3 || titles < 5 || !strings.Contains(out.String(), "Race on &lt;&amp;x&gt;") {
		t.Fatal("missing events or race annotation")
	}
	if err = render.SVG(brokenWriter{}, tr, r, g); err == nil {
		t.Fatal("lost write error")
	}
}
