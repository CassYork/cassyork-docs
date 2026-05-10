package blob

import (
	"fmt"
	"net/url"
	"strings"

	appcfg "cassyork.dev/platform/internal/config"
)

// ParseS3URI extracts bucket and object key from logical artifact URIs (s3://bucket/key/parts).
func ParseS3URI(raw string) (bucket, key string, err error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "s3" || u.Host == "" {
		return "", "", fmt.Errorf("blob: not an s3:// artifact URI")
	}
	key = strings.TrimPrefix(u.Path, "/")
	if key == "" {
		return "", "", fmt.Errorf("blob: empty object key")
	}
	return u.Host, key, nil
}

// ValidateArtifactURI checks the URI targets the configured bucket and returns the object key.
func ValidateArtifactURI(cfg appcfg.ObjectStorage, uri string) (string, error) {
	bucket, key, err := ParseS3URI(uri)
	if err != nil {
		return "", err
	}
	if bucket != cfg.Bucket {
		return "", fmt.Errorf("blob: artifact bucket does not match OBJECT_STORAGE_BUCKET")
	}
	return key, nil
}

// IsPlaceholderPendingKey is true for metadata-only registrations (pending/<doc_id> with no filename).
func IsPlaceholderPendingKey(key string) bool {
	parts := strings.Split(strings.Trim(key, "/"), "/")
	return len(parts) == 2 && parts[0] == "pending" && strings.HasPrefix(parts[1], "doc_")
}
