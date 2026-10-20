package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ClimbingSession is one visit logged in a climbing app, with every route the
// app records for it. See server/specs/climbing-ingest.md.
type ClimbingSession struct {
	Source     string           `json:"source"`
	ExternalID string           `json:"id"`
	StartTime  time.Time        `json:"start"`
	EndTime    *time.Time       `json:"end"`
	Gym        string           `json:"gym"`
	Ascents    []ClimbingAscent `json:"ascents"`
}

// ClimbingAscent is one route in a session. Result is flash, top, repeat or
// attempt; GradeRank and Attempts are nil when the app does not provide them.
type ClimbingAscent struct {
	RouteID    string `json:"route_id"`
	Discipline string `json:"discipline"`
	Result     string `json:"result"`
	Grade      string `json:"grade"`
	GradeRank  *int   `json:"grade_rank"`
	Color      string `json:"color"`
	Name       string `json:"name"`
	Attempts   *int   `json:"attempts"`
}

// ReplaceClimbingSessions writes each session and replaces all of its ascents,
// in one transaction. Replacing rather than ON CONFLICT DO NOTHING is what lets
// a route logged or removed in the app after an earlier push reach the
// database. Sessions absent from the call are left alone.
func (db *DB) ReplaceClimbingSessions(ctx context.Context, userID int, sessions []ClimbingSession) (int64, error) {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var ascents int64
	for _, s := range sessions {
		if _, err := tx.Exec(ctx,
			`INSERT INTO climbing_sessions (user_id, source, external_id, start_time, end_time, gym, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, NOW())
			 ON CONFLICT (user_id, source, external_id) DO UPDATE SET
			   start_time = EXCLUDED.start_time, end_time = EXCLUDED.end_time,
			   gym = EXCLUDED.gym, updated_at = NOW()`,
			userID, s.Source, s.ExternalID, s.StartTime, s.EndTime, s.Gym); err != nil {
			return 0, fmt.Errorf("upserting climbing session %s: %w", s.ExternalID, err)
		}
		if _, err := tx.Exec(ctx,
			`DELETE FROM climbing_ascents WHERE user_id = $1 AND source = $2 AND session_external_id = $3`,
			userID, s.Source, s.ExternalID); err != nil {
			return 0, fmt.Errorf("clearing ascents of %s: %w", s.ExternalID, err)
		}
		rows := make([][]any, 0, len(s.Ascents))
		for _, a := range s.Ascents {
			rows = append(rows, []any{userID, s.Source, s.ExternalID, a.RouteID, a.Discipline,
				a.Result, a.Grade, a.GradeRank, a.Color, a.Name, a.Attempts})
		}
		n, err := tx.CopyFrom(ctx, pgx.Identifier{"climbing_ascents"},
			[]string{"user_id", "source", "session_external_id", "route_external_id", "discipline",
				"result", "grade", "grade_rank", "color", "name", "attempts"},
			pgx.CopyFromRows(rows))
		if err != nil {
			return 0, fmt.Errorf("inserting ascents of %s: %w", s.ExternalID, err)
		}
		ascents += n
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("committing climbing sessions: %w", err)
	}
	return ascents, nil
}

// QueryClimbingSessions returns the sessions starting in [start, end), newest
// first, each with its ascents ordered hardest first.
func (db *DB) QueryClimbingSessions(ctx context.Context, start, end time.Time, userID int) ([]ClimbingSession, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT source, external_id, start_time, end_time, gym
		 FROM climbing_sessions
		 WHERE user_id = $1 AND start_time >= $2 AND start_time < $3
		 ORDER BY start_time DESC`,
		userID, start, end)
	if err != nil {
		return nil, fmt.Errorf("querying climbing sessions: %w", err)
	}
	result := []ClimbingSession{}
	index := map[[2]string]int{}
	for rows.Next() {
		s := ClimbingSession{Ascents: []ClimbingAscent{}}
		if err := rows.Scan(&s.Source, &s.ExternalID, &s.StartTime, &s.EndTime, &s.Gym); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scanning climbing session: %w", err)
		}
		index[[2]string{s.Source, s.ExternalID}] = len(result)
		result = append(result, s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading climbing sessions: %w", err)
	}
	if len(result) == 0 {
		return result, nil
	}

	rows, err = db.Pool.Query(ctx,
		`SELECT a.source, a.session_external_id, a.route_external_id, a.discipline, a.result,
		        a.grade, a.grade_rank, a.color, a.name, a.attempts
		 FROM climbing_ascents a
		 JOIN climbing_sessions s
		   ON s.user_id = a.user_id AND s.source = a.source AND s.external_id = a.session_external_id
		 WHERE s.user_id = $1 AND s.start_time >= $2 AND s.start_time < $3
		 ORDER BY a.grade_rank DESC NULLS LAST, a.name`,
		userID, start, end)
	if err != nil {
		return nil, fmt.Errorf("querying climbing ascents: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var source, session string
		var a ClimbingAscent
		if err := rows.Scan(&source, &session, &a.RouteID, &a.Discipline, &a.Result,
			&a.Grade, &a.GradeRank, &a.Color, &a.Name, &a.Attempts); err != nil {
			return nil, fmt.Errorf("scanning climbing ascent: %w", err)
		}
		if i, ok := index[[2]string{source, session}]; ok {
			result[i].Ascents = append(result[i].Ascents, a)
		}
	}
	return result, rows.Err()
}
