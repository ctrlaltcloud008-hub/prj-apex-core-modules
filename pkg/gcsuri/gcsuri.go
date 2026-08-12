// Package gcsuri parses and builds gs:// object URIs.
//
// Pipeline services pass GCS locations to each other as gs:// strings inside
// event payloads and Spanner columns, then need the bucket and object back to
// call the storage API. Every service that does this had its own copy of the
// same parser.
package gcsuri

import (
	"fmt"
	"strings"
)

const scheme = "gs://"

// Parse splits "gs://bucket/path/to/object" into its bucket and object parts.
// A URI naming only a bucket yields an empty object. Any trailing slash is
// trimmed so a prefix and the same prefix with a slash parse identically.
func Parse(uri string) (bucket, object string, err error) {
	if !strings.HasPrefix(uri, scheme) {
		return "", "", fmt.Errorf("invalid gcs URI (must start with %s): %s", scheme, uri)
	}

	trimmed := strings.TrimPrefix(uri, scheme)
	idx := strings.IndexByte(trimmed, '/')
	if idx < 0 {
		if trimmed == "" {
			return "", "", fmt.Errorf("invalid gcs URI (no bucket): %s", uri)
		}
		return trimmed, "", nil
	}

	bucket = trimmed[:idx]
	if bucket == "" {
		return "", "", fmt.Errorf("invalid gcs URI (no bucket): %s", uri)
	}
	return bucket, strings.TrimRight(trimmed[idx+1:], "/"), nil
}

// Build renders a bucket and object back into a gs:// URI.
func Build(bucket, object string) string {
	return scheme + bucket + "/" + strings.TrimLeft(object, "/")
}
