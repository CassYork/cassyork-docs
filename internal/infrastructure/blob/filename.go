package blob

import (
	"path"
	"strings"
	"unicode"
)

// SanitizeUploadFilename reduces path traversal and odd characters in multipart filenames.
func SanitizeUploadFilename(name string) string {
	base := path.Base(strings.TrimSpace(name))
	if base == "." || base == "/" || base == "" {
		return "upload.bin"
	}
	var b strings.Builder
	for _, r := range base {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			b.WriteRune(r)
		case r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := b.String()
	if out == "" || out == "." {
		return "upload.bin"
	}
	const maxLen = 200
	if len(out) <= maxLen {
		return out
	}
	ext := path.Ext(out)
	stem := strings.TrimSuffix(out, ext)
	n := maxLen - len(ext)
	if n < 0 {
		n = 0
	}
	if len(stem) > n {
		stem = stem[:n]
	}
	if stem == "" {
		return "upload" + ext
	}
	return stem + ext
}
