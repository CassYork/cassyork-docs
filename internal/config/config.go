package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Settings is shared runtime wiring (transport, stores, observability).
// It deliberately avoids naming specific vendors (Postgres, S3, etc.).
type Settings struct {
	Database      Database
	ObjectStorage ObjectStorage
	Temporal
	Telemetry
}

// Database holds an opaque URL for the primary transactional store.
type Database struct {
	URL string
}

// ObjectStorage configures blob/artifact addressing (S3-compatible APIs, etc.).
type ObjectStorage struct {
	// Scheme is the URI scheme used in Document.storage_uri (e.g. "s3"). Empty defaults to "s3".
	Scheme          string
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	UsePathStyle    bool
}

type Temporal struct {
	Address   string
	Namespace string
}

type Telemetry struct {
	OtelExporterEndpoint string
}

// Load reads environment variables. Legacy keys (POSTGRES_URL, S3_*) remain supported as fallbacks.
func Load() Settings {
	return Settings{
		Database: Database{
			URL: databaseURL(),
		},
		ObjectStorage: ObjectStorage{
			Scheme:          firstNonEmpty(os.Getenv("OBJECT_STORAGE_SCHEME"), os.Getenv("S3_SCHEME")),
			Endpoint:        firstNonEmpty(os.Getenv("OBJECT_STORAGE_ENDPOINT"), os.Getenv("S3_ENDPOINT"), "http://127.0.0.1:9000"),
			Region:          firstNonEmpty(os.Getenv("OBJECT_STORAGE_REGION"), os.Getenv("S3_REGION"), "us-east-1"),
			Bucket:          firstNonEmpty(os.Getenv("OBJECT_STORAGE_BUCKET"), os.Getenv("S3_BUCKET"), "cassyork-documents"),
			AccessKeyID:     firstNonEmpty(os.Getenv("OBJECT_STORAGE_ACCESS_KEY_ID"), os.Getenv("S3_ACCESS_KEY"), "minio"),
			SecretAccessKey: firstNonEmpty(os.Getenv("OBJECT_STORAGE_SECRET_ACCESS_KEY"), os.Getenv("S3_SECRET_KEY"), "minio12345"),
			UsePathStyle:    objectStorageUsePathStyle(),
		},
		Temporal: Temporal{
			Address:   getenv("TEMPORAL_ADDRESS", "localhost:7233"),
			Namespace: getenv("TEMPORAL_NAMESPACE", "default"),
		},
		Telemetry: Telemetry{
			OtelExporterEndpoint: getenv("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
		},
	}
}

// ArtifactURI builds a logical URI for an object key under the configured bucket (scheme://bucket/path).
func (o ObjectStorage) ArtifactURI(elem ...string) string {
	scheme := o.Scheme
	if scheme == "" {
		scheme = "s3"
	}
	parts := make([]string, 0, len(elem))
	for _, e := range elem {
		e = strings.Trim(e, "/")
		if e != "" {
			parts = append(parts, e)
		}
	}
	key := strings.Join(parts, "/")
	return fmt.Sprintf("%s://%s/%s", scheme, o.Bucket, key)
}

func databaseURL() string {
	if v := strings.TrimSpace(os.Getenv("DATABASE_URL")); v != "" {
		return v
	}
	return getenv("POSTGRES_URL", "postgres://temporal:temporal@localhost:5432/cassyork?sslmode=disable")
}

func objectStorageUsePathStyle() bool {
	if v := os.Getenv("OBJECT_STORAGE_USE_PATH_STYLE"); v != "" {
		return parseBoolOr(v, true)
	}
	return getbool("S3_USE_PATH_STYLE", true)
}

func getenv(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func getbool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return parseBoolOr(v, def)
}

func parseBoolOr(v string, def bool) bool {
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}
