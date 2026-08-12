package hls

import (
	"fmt"
	"sort"
	"strings"
)

// Variant is one rung of the ABR ladder as it appears in the master playlist.
//
// PeakBandwidthBps is what BANDWIDTH must carry: RFC 8216 defines it as the peak
// segment bit rate, not the average, and players use it to decide whether a rung
// is safe on the current connection. AvgBandwidthBps is optional and omitted
// when zero.
type Variant struct {
	Name             string
	PlaylistURI      string
	Width            int
	Height           int
	PeakBandwidthBps int64
	AvgBandwidthBps  int64
	Codecs           string
}

// BuildMaster renders the master playlist. Variants are emitted lowest-bandwidth
// first: a player with no bandwidth estimate yet starts on the first variant
// listed, so leading with the cheapest rung gives the fastest, most reliable
// start and lets ABR climb from there.
//
// Codecs is omitted for any variant whose value is empty rather than guessed. A
// wrong CODECS string is worse than an absent one — players reject a stream they
// believe they cannot decode, while a missing attribute merely costs them a
// probe of the first segment.
func BuildMaster(variants []Variant) ([]byte, error) {
	if len(variants) == 0 {
		return nil, fmt.Errorf("master playlist needs at least one variant")
	}

	ordered := make([]Variant, len(variants))
	copy(ordered, variants)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].PeakBandwidthBps != ordered[j].PeakBandwidthBps {
			return ordered[i].PeakBandwidthBps < ordered[j].PeakBandwidthBps
		}
		return ordered[i].Height < ordered[j].Height
	})

	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	b.WriteString("#EXT-X-VERSION:7\n")
	b.WriteString("#EXT-X-INDEPENDENT-SEGMENTS\n")

	for _, v := range ordered {
		if v.PlaylistURI == "" {
			return nil, fmt.Errorf("variant %q has no playlist URI", v.Name)
		}
		if v.PeakBandwidthBps <= 0 {
			return nil, fmt.Errorf("variant %q has non-positive bandwidth", v.Name)
		}

		attrs := []string{fmt.Sprintf("BANDWIDTH=%d", v.PeakBandwidthBps)}
		if v.AvgBandwidthBps > 0 {
			attrs = append(attrs, fmt.Sprintf("AVERAGE-BANDWIDTH=%d", v.AvgBandwidthBps))
		}
		if v.Width > 0 && v.Height > 0 {
			attrs = append(attrs, fmt.Sprintf("RESOLUTION=%dx%d", v.Width, v.Height))
		}
		if v.Codecs != "" {
			attrs = append(attrs, fmt.Sprintf("CODECS=%q", v.Codecs))
		}

		fmt.Fprintf(&b, "#EXT-X-STREAM-INF:%s\n%s\n", strings.Join(attrs, ","), v.PlaylistURI)
	}

	return []byte(b.String()), nil
}
