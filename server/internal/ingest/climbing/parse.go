// Package climbing ingests sessions from a climbing logbook app, pushed by a
// client as JSON. The wire format is server/specs/climbing-ingest.md.
package climbing

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/claude/freereps/internal/storage"
)

// maxBodyBytes bounds the request body. A year of sessions from one gym app
// measured 100 KB (Arkose+, 23 sessions, 2026-10-20).
const maxBodyBytes = 8 << 20

// Payload is the request body of POST /api/v1/ingest/climbing.
type Payload struct {
	Source   string    `json:"source"`
	Sessions []Session `json:"sessions"`
}

// Session is one visit as the client sends it.
type Session struct {
	ID      string     `json:"id"`
	Start   time.Time  `json:"start"`
	End     *time.Time `json:"end"`
	Gym     string     `json:"gym"`
	Ascents []Ascent   `json:"ascents"`
}

// Ascent is one route in a session as the client sends it.
type Ascent struct {
	RouteID    string `json:"route_id"`
	Discipline string `json:"discipline"`
	Result     string `json:"result"`
	Grade      string `json:"grade"`
	GradeRank  *int   `json:"grade_rank"`
	Color      string `json:"color"`
	Name       string `json:"name"`
	Attempts   *int   `json:"attempts"`
}

var results = map[string]bool{"flash": true, "top": true, "repeat": true, "attempt": true}

// Parse decodes and validates a payload. Any invalid session rejects the whole
// request, so a client bug cannot store half of a push.
func Parse(r io.Reader) ([]storage.ClimbingSession, error) {
	dec := json.NewDecoder(io.LimitReader(r, maxBodyBytes))
	dec.DisallowUnknownFields()
	var p Payload
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("decoding climbing payload: %w", err)
	}
	source := strings.TrimSpace(p.Source)
	if source == "" {
		return nil, errors.New("source is required")
	}

	out := make([]storage.ClimbingSession, 0, len(p.Sessions))
	seen := map[string]bool{}
	for i, s := range p.Sessions {
		if s.ID == "" {
			return nil, fmt.Errorf("sessions[%d]: id is required", i)
		}
		if seen[s.ID] {
			return nil, fmt.Errorf("sessions[%d]: duplicate id %q", i, s.ID)
		}
		seen[s.ID] = true
		if s.Start.IsZero() {
			return nil, fmt.Errorf("session %s: start is required", s.ID)
		}
		if s.End != nil && s.End.Before(s.Start) {
			return nil, fmt.Errorf("session %s: end is before start", s.ID)
		}
		cs := storage.ClimbingSession{
			Source: source, ExternalID: s.ID, StartTime: s.Start, EndTime: s.End, Gym: s.Gym,
			Ascents: make([]storage.ClimbingAscent, 0, len(s.Ascents)),
		}
		routes := map[string]bool{}
		for j, a := range s.Ascents {
			if a.RouteID == "" {
				return nil, fmt.Errorf("session %s, ascents[%d]: route_id is required", s.ID, j)
			}
			if routes[a.RouteID] {
				return nil, fmt.Errorf("session %s: route %s appears twice", s.ID, a.RouteID)
			}
			routes[a.RouteID] = true
			if !results[a.Result] {
				return nil, fmt.Errorf("session %s, route %s: result %q is not flash, top, repeat or attempt",
					s.ID, a.RouteID, a.Result)
			}
			if a.Attempts != nil && *a.Attempts < 1 {
				return nil, fmt.Errorf("session %s, route %s: attempts must be at least 1", s.ID, a.RouteID)
			}
			discipline := a.Discipline
			if discipline == "" {
				discipline = "boulder"
			}
			cs.Ascents = append(cs.Ascents, storage.ClimbingAscent{
				RouteID: a.RouteID, Discipline: discipline, Result: a.Result, Grade: a.Grade,
				GradeRank: a.GradeRank, Color: a.Color, Name: a.Name, Attempts: a.Attempts,
			})
		}
		out = append(out, cs)
	}
	return out, nil
}
