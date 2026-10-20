package climbing

import (
	"context"
	"fmt"
	"io"

	"github.com/claude/freereps/internal/ingest"
	"github.com/claude/freereps/internal/storage"
)

// Provider stores pushed climbing sessions.
type Provider struct {
	db *storage.DB
}

// NewProvider creates a climbing ingest provider.
func NewProvider(db *storage.DB) *Provider {
	return &Provider{db: db}
}

// Ingest parses a payload and replaces the sessions it names.
func (p *Provider) Ingest(ctx context.Context, r io.Reader, userID int) (*ingest.Result, error) {
	sessions, err := Parse(r)
	if err != nil {
		return nil, err
	}
	result := &ingest.Result{ClimbingSessionsReceived: len(sessions)}
	for _, s := range sessions {
		result.ClimbingAscentsReceived += len(s.Ascents)
	}
	n, err := p.db.ReplaceClimbingSessions(ctx, userID, sessions)
	if err != nil {
		return result, fmt.Errorf("storing climbing sessions: %w", err)
	}
	result.ClimbingAscentsInserted = n
	return result, nil
}
