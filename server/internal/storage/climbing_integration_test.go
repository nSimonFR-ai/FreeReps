//go:build integration

// A climbing session is edited in the app after the first push — tops are
// logged during the visit — so an ingest replaces the session's ascents rather
// than adding to them. These tests pin that replacement, and that it leaves
// other sessions alone, against the real keys and foreign key.
//
// Run with:
//
//	FREEREPS_TEST_DSN=postgres://user:pass@host:port/db?sslmode=disable \
//	  go test -tags integration ./internal/storage/
package storage

import (
	"context"
	"testing"
	"time"
)

const climbingTestUser = 4249

func climbingTestDB(t *testing.T) *DB {
	t.Helper()
	db := aggTestDB(t)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx,
		`INSERT INTO users (id, login) VALUES ($1, 'climbing-test') ON CONFLICT (id) DO NOTHING`,
		climbingTestUser); err != nil {
		t.Fatalf("creating test user: %v", err)
	}
	if _, err := db.Pool.Exec(ctx, `DELETE FROM climbing_sessions WHERE user_id = $1`, climbingTestUser); err != nil {
		t.Fatalf("clearing climbing_sessions: %v", err)
	}
	return db
}

func rank(n int) *int { return &n }

func climbingSession(id string, start time.Time, routes ...string) ClimbingSession {
	s := ClimbingSession{Source: "Test", ExternalID: id, StartTime: start, Gym: "Gym"}
	for i, r := range routes {
		s.Ascents = append(s.Ascents, ClimbingAscent{
			RouteID: r, Discipline: "boulder", Result: "top", Grade: "g", GradeRank: rank(i),
		})
	}
	return s
}

func TestClimbingPushReplacesTheSessionsAscents(t *testing.T) {
	db := climbingTestDB(t)
	ctx := context.Background()
	day := time.Date(2026, 10, 18, 8, 48, 0, 0, time.UTC)

	if n, err := db.ReplaceClimbingSessions(ctx, climbingTestUser, []ClimbingSession{
		climbingSession("a", day, "r1", "r2"),
		climbingSession("b", day.Add(-72*time.Hour), "r9"),
	}); err != nil || n != 3 {
		t.Fatalf("first push: n=%d err=%v", n, err)
	}

	// Later push of session a only: r2 removed in the app, r3 and r4 logged.
	edited := climbingSession("a", day, "r1", "r3", "r4")
	edited.Gym = "Renamed"
	if n, err := db.ReplaceClimbingSessions(ctx, climbingTestUser, []ClimbingSession{edited}); err != nil || n != 3 {
		t.Fatalf("second push: n=%d err=%v", n, err)
	}

	got, err := db.QueryClimbingSessions(ctx, day.Add(-7*24*time.Hour), day.Add(time.Hour), climbingTestUser)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ExternalID != "a" || got[1].ExternalID != "b" {
		t.Fatalf("want sessions a then b, got %+v", got)
	}
	if got[0].Gym != "Renamed" || len(got[0].Ascents) != 3 {
		t.Fatalf("session a not replaced: %+v", got[0])
	}
	// Hardest first: r4 carries the highest rank.
	if got[0].Ascents[0].RouteID != "r4" || got[0].Ascents[2].RouteID != "r1" {
		t.Fatalf("ascents not ordered by grade_rank: %+v", got[0].Ascents)
	}
	if len(got[1].Ascents) != 1 || got[1].Ascents[0].RouteID != "r9" {
		t.Fatalf("session b must be untouched: %+v", got[1])
	}
}

func TestClimbingPushIsAtomic(t *testing.T) {
	db := climbingTestDB(t)
	ctx := context.Background()
	day := time.Date(2026, 10, 18, 8, 48, 0, 0, time.UTC)

	if _, err := db.ReplaceClimbingSessions(ctx, climbingTestUser, []ClimbingSession{
		climbingSession("a", day, "r1"),
	}); err != nil {
		t.Fatal(err)
	}
	// A result the CHECK constraint rejects, in the second session of the push.
	bad := climbingSession("c", day.Add(time.Hour), "r5")
	bad.Ascents[0].Result = "sent"
	if _, err := db.ReplaceClimbingSessions(ctx, climbingTestUser, []ClimbingSession{
		climbingSession("a", day), bad,
	}); err == nil {
		t.Fatal("expected the CHECK constraint to reject the push")
	}

	got, err := db.QueryClimbingSessions(ctx, day.Add(-time.Hour), day.Add(2*time.Hour), climbingTestUser)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(got[0].Ascents) != 1 {
		t.Fatalf("a failed push must change nothing, got %+v", got)
	}
}
