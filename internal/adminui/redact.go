package adminui

import (
	"net/url"
	"strings"
)

// RedactDatabaseURL returns a display-safe database URL (password masked via net/url.Redacted).
func RedactDatabaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "(not configured)"
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" {
		return "(unable to parse — check DATABASE_URL / POSTGRES_URL)"
	}
	return u.Redacted()
}

// MaskCredential shows a shortened opaque credential for display only.
func MaskCredential(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "(empty)"
	}
	if len(s) <= 8 {
		return "••••••••"
	}
	return s[:4] + "••••••••" + s[len(s)-2:]
}
