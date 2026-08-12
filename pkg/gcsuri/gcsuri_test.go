package gcsuri

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		uri, bucket, object string
		wantErr             bool
	}{
		{uri: "gs://b/o", bucket: "b", object: "o"},
		{uri: "gs://b/a/b/c.m3u8", bucket: "b", object: "a/b/c.m3u8"},
		{uri: "gs://b", bucket: "b", object: ""},
		// A prefix and the same prefix with a trailing slash must agree, or the
		// stitch service's relative-path arithmetic drifts by one segment.
		{uri: "gs://b/pre/fix/", bucket: "b", object: "pre/fix"},
		{uri: "https://b/o", wantErr: true},
		{uri: "gs://", wantErr: true},
		{uri: "gs:///o", wantErr: true},
	}

	for _, c := range cases {
		bucket, object, err := Parse(c.uri)
		if c.wantErr {
			if err == nil {
				t.Errorf("Parse(%q) = (%q,%q), want error", c.uri, bucket, object)
			}
			continue
		}
		if err != nil {
			t.Errorf("Parse(%q) unexpected error: %v", c.uri, err)
			continue
		}
		if bucket != c.bucket || object != c.object {
			t.Errorf("Parse(%q) = (%q,%q), want (%q,%q)", c.uri, bucket, object, c.bucket, c.object)
		}
	}
}

func TestBuildRoundTrips(t *testing.T) {
	bucket, object, err := Parse(Build("my-bucket", "a/b/c.m4s"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bucket != "my-bucket" || object != "a/b/c.m4s" {
		t.Errorf("round trip = (%q,%q)", bucket, object)
	}
}
