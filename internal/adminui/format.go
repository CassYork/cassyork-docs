package adminui

import (
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func formatTime(t pgtype.Timestamptz) string {
	if !t.Valid {
		return "—"
	}
	return t.Time.UTC().Format(time.RFC3339)
}

func shorten(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-1] + "…"
}
