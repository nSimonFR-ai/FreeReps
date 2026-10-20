package server

import (
	"net/http"
	"time"
)

// handleClimbingIngest stores sessions pushed by a climbing logbook client
// (server/specs/climbing-ingest.md). A malformed payload is a 400; nothing of
// it is stored.
func (s *Server) handleClimbingIngest(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}

	start := time.Now()
	result, err := s.climbing.Ingest(r.Context(), r.Body, uid)
	durationMs := int(time.Since(start).Milliseconds())
	if err != nil {
		s.log.Error("climbing ingest error", "error", err)
		status := http.StatusBadRequest
		if result != nil {
			// Parsed, so the failure was the database's.
			status = http.StatusInternalServerError
			go s.logImport(uid, "climbing", result, err, durationMs)
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}

	go s.logImport(uid, "climbing", result, nil, durationMs)
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleQueryClimbingSessions(w http.ResponseWriter, r *http.Request) {
	start, end, err := parseTimeRange(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}
	sessions, err := s.db.QueryClimbingSessions(r.Context(), start, end, uid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, sessions)
}
