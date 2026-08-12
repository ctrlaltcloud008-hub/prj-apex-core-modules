package hls

import (
	"strings"
	"testing"
)

func TestBuildMasterOrdersByBandwidthAscending(t *testing.T) {
	// Deliberately supplied highest-first to prove the builder reorders.
	out, err := BuildMaster([]Variant{
		{Name: "720p", PlaylistURI: "720p/rendition.m3u8", Width: 1280, Height: 720, PeakBandwidthBps: 3_000_000, Codecs: "avc1.64001f"},
		{Name: "360p", PlaylistURI: "360p/rendition.m3u8", Width: 640, Height: 360, PeakBandwidthBps: 800_000, Codecs: "avc1.64001f"},
		{Name: "480p", PlaylistURI: "480p/rendition.m3u8", Width: 854, Height: 480, PeakBandwidthBps: 1_400_000, Codecs: "avc1.64001f"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := string(out)
	// A player with no bandwidth estimate starts on the first variant, so the
	// cheapest rung must lead.
	first := strings.Index(got, "360p/rendition.m3u8")
	mid := strings.Index(got, "480p/rendition.m3u8")
	last := strings.Index(got, "720p/rendition.m3u8")
	if !(first < mid && mid < last) {
		t.Errorf("variants not ascending by bandwidth:\n%s", got)
	}

	for _, want := range []string{
		"#EXTM3U", "#EXT-X-VERSION:7", "#EXT-X-INDEPENDENT-SEGMENTS",
		"BANDWIDTH=800000", "RESOLUTION=640x360", `CODECS="avc1.64001f"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("master playlist missing %q:\n%s", want, got)
		}
	}
}

// An absent CODECS costs a player one probe; a wrong one makes it refuse the
// stream. When the init segment could not be parsed the attribute is dropped.
func TestBuildMasterOmitsEmptyCodecs(t *testing.T) {
	out, err := BuildMaster([]Variant{
		{Name: "360p", PlaylistURI: "360p/rendition.m3u8", Width: 640, Height: 360, PeakBandwidthBps: 800_000},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(string(out), "CODECS") {
		t.Errorf("expected no CODECS attribute:\n%s", out)
	}
}

func TestBuildMasterRejectsInvalid(t *testing.T) {
	if _, err := BuildMaster(nil); err == nil {
		t.Error("empty variant list should error")
	}
	if _, err := BuildMaster([]Variant{{Name: "x", PeakBandwidthBps: 1}}); err == nil {
		t.Error("missing playlist URI should error")
	}
	if _, err := BuildMaster([]Variant{{Name: "x", PlaylistURI: "u"}}); err == nil {
		t.Error("zero bandwidth should error")
	}
}

func TestBuildRenditionRaisesTargetDurationToLongestSegment(t *testing.T) {
	// Configured target is 6s but a segment overran to 7.5s; RFC 8216 requires
	// TARGETDURATION >= ceil(max EXTINF), so it must be raised to 8.
	chunks := []Chunk{{
		RelDir: "00000/a1",
		Media: Media{InitURI: "init.mp4", Segments: []Segment{
			{DurationSec: 6.0, URI: "seg_0000.m4s"},
			{DurationSec: 7.5, URI: "seg_0001.m4s"},
		}},
	}}

	out, totalMs := BuildRendition(chunks, 6)
	if !strings.Contains(string(out), "#EXT-X-TARGETDURATION:8") {
		t.Errorf("expected TARGETDURATION 8:\n%s", out)
	}
	if totalMs != 13500 {
		t.Errorf("totalMs = %d, want 13500", totalMs)
	}
	if !strings.Contains(string(out), "00000/a1/seg_0000.m4s") {
		t.Errorf("segment URIs not prefixed with chunk dir:\n%s", out)
	}
}
