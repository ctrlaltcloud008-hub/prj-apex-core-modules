// Package hls parses and assembles HLS playlists (RFC 8216).
//
// The pipeline builds playlists at three levels: the worker's FFmpeg writes one
// media playlist per encoded chunk, the stitch service concatenates a
// rendition's chunk playlists into a single VOD media playlist, and the packager
// writes the master playlist that ties the renditions together into an ABR
// ladder. This is pure text manipulation — no media bytes are ever rewritten.
package hls

import (
	"bufio"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Segment is one media segment reference with its exact EXTINF duration.
type Segment struct {
	DurationSec float64
	URI         string
}

// Media is a parsed media playlist: the init segment from EXT-X-MAP plus each
// EXTINF/URI pair in order.
type Media struct {
	InitURI  string
	Segments []Segment
}

// TotalSec is the summed EXTINF duration of the playlist.
func (m Media) TotalSec() float64 {
	var total float64
	for _, s := range m.Segments {
		total += s.DurationSec
	}
	return total
}

// ParseMedia parses a media playlist, extracting the init segment (EXT-X-MAP)
// and each EXTINF/segment pair.
func ParseMedia(data []byte) (Media, error) {
	var m Media
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	var pendingDur float64
	haveDur := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case line == "":
			continue
		case strings.HasPrefix(line, "#EXT-X-MAP:"):
			m.InitURI = extractURIAttr(line)
		case strings.HasPrefix(line, "#EXTINF:"):
			v := strings.TrimPrefix(line, "#EXTINF:")
			v = strings.TrimSuffix(strings.TrimSpace(v), ",")
			// EXTINF may carry a title after a comma; take the numeric head.
			if idx := strings.IndexByte(v, ','); idx >= 0 {
				v = v[:idx]
			}
			d, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
			if err != nil {
				return Media{}, fmt.Errorf("parse EXTINF %q: %w", line, err)
			}
			pendingDur = d
			haveDur = true
		case strings.HasPrefix(line, "#"):
			continue
		default:
			if !haveDur {
				return Media{}, fmt.Errorf("segment %q without preceding EXTINF", line)
			}
			m.Segments = append(m.Segments, Segment{DurationSec: pendingDur, URI: line})
			haveDur = false
		}
	}
	if err := scanner.Err(); err != nil {
		return Media{}, fmt.Errorf("scan media playlist: %w", err)
	}
	if m.InitURI == "" {
		return Media{}, fmt.Errorf("media playlist has no EXT-X-MAP init segment")
	}
	if len(m.Segments) == 0 {
		return Media{}, fmt.Errorf("media playlist has no segments")
	}
	return m, nil
}

// Chunk pairs a parsed media playlist with the relative path of its output
// directory (relative to the rendition playlist being written).
type Chunk struct {
	RelDir string
	Media  Media
}

// BuildRendition assembles ordered chunks into one VOD media playlist and
// returns the playlist bytes plus the total duration in milliseconds. The
// EXT-X-MAP is re-declared per chunk so playback stays correct even if init
// segments ever diverge; the stitch service separately enforces byte-identity.
//
// targetDurationSec is treated as a floor only: RFC 8216 requires
// EXT-X-TARGETDURATION to be at least the longest EXTINF present, and FFmpeg can
// overshoot the requested segment length when the next keyframe lands late.
// Advertising a value below the real maximum makes strict players reject the
// playlist outright.
func BuildRendition(chunks []Chunk, targetDurationSec int) ([]byte, int64) {
	var b strings.Builder
	var totalSec float64

	targetDuration := targetDurationSec
	for _, c := range chunks {
		for _, seg := range c.Media.Segments {
			if rounded := int(math.Ceil(seg.DurationSec)); rounded > targetDuration {
				targetDuration = rounded
			}
		}
	}

	b.WriteString("#EXTM3U\n")
	b.WriteString("#EXT-X-VERSION:7\n")
	fmt.Fprintf(&b, "#EXT-X-TARGETDURATION:%d\n", targetDuration)
	b.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n")
	b.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n")

	for _, c := range chunks {
		fmt.Fprintf(&b, "#EXT-X-MAP:URI=%q\n", JoinRel(c.RelDir, c.Media.InitURI))
		for _, seg := range c.Media.Segments {
			fmt.Fprintf(&b, "#EXTINF:%.6f,\n", seg.DurationSec)
			b.WriteString(JoinRel(c.RelDir, seg.URI))
			b.WriteByte('\n')
			totalSec += seg.DurationSec
		}
	}
	b.WriteString("#EXT-X-ENDLIST\n")

	return []byte(b.String()), int64(totalSec * 1000)
}

// JoinRel joins a chunk's relative directory with a file name from its media
// playlist, producing a path relative to the playlist being written.
func JoinRel(relDir, name string) string {
	relDir = strings.Trim(relDir, "/")
	if relDir == "" {
		return name
	}
	return relDir + "/" + name
}

func extractURIAttr(line string) string {
	idx := strings.Index(line, "URI=")
	if idx < 0 {
		return ""
	}
	rest := line[idx+len("URI="):]
	rest = strings.TrimSpace(rest)
	if strings.HasPrefix(rest, "\"") {
		if end := strings.IndexByte(rest[1:], '"'); end >= 0 {
			return rest[1 : 1+end]
		}
	}
	// Unquoted attribute: take up to the next comma.
	if end := strings.IndexByte(rest, ','); end >= 0 {
		return rest[:end]
	}
	return rest
}
