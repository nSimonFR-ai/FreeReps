package climbing

import (
	"strings"
	"testing"
)

const valid = `{"source":"Arkose+","sessions":[{"id":"s1","start":"2026-10-18T08:48:57Z","end":null,
 "gym":"Arkose Chevaleret","ascents":[
  {"route_id":"r1","result":"flash","grade":"bleu 2","grade_rank":52,"color":"bleu","name":"18"},
  {"route_id":"r2","discipline":"boulder","result":"attempt","attempts":3}]}]}`

// TestParseValid checks the field mapping and the boulder default, which is
// what lets a bouldering-only client omit discipline.
func TestParseValid(t *testing.T) {
	got, err := Parse(strings.NewReader(valid))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Source != "Arkose+" || got[0].ExternalID != "s1" || got[0].EndTime != nil {
		t.Fatalf("unexpected session: %+v", got)
	}
	a := got[0].Ascents
	if len(a) != 2 || a[0].Discipline != "boulder" || *a[0].GradeRank != 52 || a[0].Attempts != nil {
		t.Fatalf("unexpected ascents: %+v", a)
	}
	if *a[1].Attempts != 3 || a[1].GradeRank != nil {
		t.Fatalf("unexpected second ascent: %+v", a[1])
	}
}

// TestParseRejects covers each rule that would otherwise store a row the
// replace-per-session model cannot express: a duplicate route breaks the
// primary key mid-transaction, and an unknown result breaks the CHECK.
func TestParseRejects(t *testing.T) {
	cases := map[string]string{
		"no source":       `{"sessions":[]}`,
		"no id":           `{"source":"a","sessions":[{"start":"2026-10-18T08:00:00Z"}]}`,
		"duplicate id":    `{"source":"a","sessions":[{"id":"s","start":"2026-10-18T08:00:00Z"},{"id":"s","start":"2026-10-18T08:00:00Z"}]}`,
		"no start":        `{"source":"a","sessions":[{"id":"s"}]}`,
		"end before":      `{"source":"a","sessions":[{"id":"s","start":"2026-10-18T08:00:00Z","end":"2026-10-18T07:00:00Z"}]}`,
		"no route id":     `{"source":"a","sessions":[{"id":"s","start":"2026-10-18T08:00:00Z","ascents":[{"result":"top"}]}]}`,
		"duplicate route": `{"source":"a","sessions":[{"id":"s","start":"2026-10-18T08:00:00Z","ascents":[{"route_id":"r","result":"top"},{"route_id":"r","result":"flash"}]}]}`,
		"bad result":      `{"source":"a","sessions":[{"id":"s","start":"2026-10-18T08:00:00Z","ascents":[{"route_id":"r","result":"sent"}]}]}`,
		"zero attempts":   `{"source":"a","sessions":[{"id":"s","start":"2026-10-18T08:00:00Z","ascents":[{"route_id":"r","result":"top","attempts":0}]}]}`,
		"unknown field":   `{"source":"a","sessions":[],"extra":1}`,
	}
	for name, body := range cases {
		if _, err := Parse(strings.NewReader(body)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}
