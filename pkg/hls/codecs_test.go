package hls

import (
	"os"
	"testing"
)

// The fixture is a real CMAF init segment produced by the transcode worker
// (libx264 -crf 22 + AAC at 128k, 720p). ffprobe reports High profile, level
// 31, AAC-LC for it, which is exactly what the RFC 6381 string below encodes:
// 0x64 = 100 = High, 0x00 constraint flags, 0x1f = 31 = level 3.1.
func TestCodecsFromInitRealSegment(t *testing.T) {
	init, err := os.ReadFile("testdata/init_avc_aac.mp4")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	got := CodecsFromInit(init)
	const want = "avc1.64001f,mp4a.40.2"
	if got != want {
		t.Errorf("CodecsFromInit() = %q, want %q", got, want)
	}
}

// A wrong CODECS string makes players refuse the stream outright, so anything
// unparseable must yield nothing rather than a guess.
func TestCodecsFromInitMalformed(t *testing.T) {
	cases := map[string][]byte{
		"empty":          nil,
		"garbage":        []byte("not an mp4 at all"),
		"truncated box":  {0x00, 0x00, 0x00, 0x20, 'm', 'o', 'o', 'v'},
		"absurd size":    {0xff, 0xff, 0xff, 0xff, 'm', 'o', 'o', 'v', 0x00},
		"zero size loop": {0x00, 0x00, 0x00, 0x00, 'm', 'o', 'o', 'v'},
	}

	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if got := CodecsFromInit(data); got != "" {
				t.Errorf("CodecsFromInit(%s) = %q, want empty", name, got)
			}
		})
	}
}
