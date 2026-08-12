package hls

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// CodecsFromInit derives the RFC 6381 codecs string for a CMAF init segment —
// the value HLS puts in a master playlist's CODECS attribute, e.g.
// "avc1.640020,mp4a.40.2".
//
// The profile and level have to come from the bitstream itself. They are a
// property of what the encoder actually produced, which depends on resolution,
// rate control and encoder defaults — not something derivable from the ladder
// rung we asked for. Advertising a profile the stream does not match makes
// players refuse it, so an unrecognised sample entry contributes nothing rather
// than a guess.
//
// Returns an empty string when nothing recognisable is found.
func CodecsFromInit(init []byte) string {
	var codecs []string
	seen := map[string]bool{}

	for _, entry := range findSampleEntries(init) {
		var c string
		switch entry.format {
		case "avc1", "avc3":
			c = avcCodec(entry.format, entry.body)
		case "mp4a":
			c = aacCodec(entry.body)
		}
		if c != "" && !seen[c] {
			seen[c] = true
			codecs = append(codecs, c)
		}
	}

	return strings.Join(codecs, ",")
}

type sampleEntry struct {
	format string
	body   []byte
}

// findSampleEntries walks moov > trak > mdia > minf > stbl > stsd and returns
// each sample entry it finds. The walk is defensive: init segments come from
// GCS and a truncated or malformed one must yield no codecs rather than panic.
func findSampleEntries(data []byte) []sampleEntry {
	moov := findBox(data, "moov")
	if moov == nil {
		return nil
	}

	var entries []sampleEntry
	for _, trak := range findBoxes(moov, "trak") {
		mdia := findBox(trak, "mdia")
		if mdia == nil {
			continue
		}
		minf := findBox(mdia, "minf")
		if minf == nil {
			continue
		}
		stbl := findBox(minf, "stbl")
		if stbl == nil {
			continue
		}
		stsd := findBox(stbl, "stsd")
		if len(stsd) < 8 {
			continue
		}

		// stsd is a FullBox (4 bytes version+flags) followed by a 4-byte entry
		// count, then the sample entries themselves.
		for _, e := range parseBoxes(stsd[8:]) {
			entries = append(entries, sampleEntry{format: e.typ, body: e.payload})
		}
	}
	return entries
}

// avcCodec renders "avc1.PPCCLL" from the avcC configuration record, where PP is
// the profile, CC the constraint flags and LL the level, each two hex digits.
func avcCodec(format string, avc1Body []byte) string {
	// A visual sample entry has 78 bytes of fixed fields before its child boxes.
	if len(avc1Body) <= 78 {
		return ""
	}
	avcC := findBox(avc1Body[78:], "avcC")
	if len(avcC) < 4 {
		return ""
	}
	// avcC: [0]=configurationVersion, [1]=AVCProfileIndication,
	// [2]=profile_compatibility, [3]=AVCLevelIndication.
	return fmt.Sprintf("%s.%02x%02x%02x", format, avcC[1], avcC[2], avcC[3])
}

// aacCodec renders the mp4a codec string. AAC-LC is by far the common case and
// the only one this pipeline's encoder produces, so anything that is not
// recognisably MPEG-4 audio contributes nothing.
func aacCodec(mp4aBody []byte) string {
	// An audio sample entry has 28 bytes of fixed fields before its child boxes.
	if len(mp4aBody) <= 28 {
		return ""
	}
	esds := findBox(mp4aBody[28:], "esds")
	if len(esds) < 5 {
		return ""
	}

	// Object type indication 0x40 is MPEG-4 audio; the audio object type then
	// comes from the top 5 bits of the decoder-specific info. AAC-LC is 2.
	body := esds[4:] // skip FullBox version+flags
	for i := 0; i+1 < len(body); i++ {
		if body[i] == 0x04 { // DecoderConfigDescriptor tag
			// Skip the tag's length bytes (each with a continuation high bit).
			j := i + 1
			for j < len(body) && body[j]&0x80 != 0 {
				j++
			}
			j++
			if j < len(body) && body[j] == 0x40 {
				if aot := audioObjectType(body[j:]); aot > 0 {
					return fmt.Sprintf("mp4a.40.%d", aot)
				}
				return "mp4a.40.2"
			}
		}
	}
	return ""
}

// audioObjectType pulls the AudioSpecificConfig's 5-bit object type out of the
// DecoderSpecificInfo (tag 0x05) that follows the decoder config.
func audioObjectType(b []byte) int {
	for i := 0; i+1 < len(b); i++ {
		if b[i] == 0x05 {
			j := i + 1
			for j < len(b) && b[j]&0x80 != 0 {
				j++
			}
			j++
			if j < len(b) {
				return int(b[j] >> 3)
			}
			return 0
		}
	}
	return 0
}

type box struct {
	typ     string
	payload []byte
}

// parseBoxes splits a buffer into its top-level ISO-BMFF boxes.
func parseBoxes(data []byte) []box {
	var boxes []box
	for off := 0; off+8 <= len(data); {
		size := int(binary.BigEndian.Uint32(data[off : off+4]))
		typ := string(data[off+4 : off+8])

		switch {
		case size == 0:
			// Box extends to end of buffer.
			size = len(data) - off
		case size == 1:
			// 64-bit extended size follows the type.
			if off+16 > len(data) {
				return boxes
			}
			size = int(binary.BigEndian.Uint64(data[off+8 : off+16]))
		}
		if size < 8 || off+size > len(data) {
			return boxes
		}

		boxes = append(boxes, box{typ: typ, payload: data[off+8 : off+size]})
		off += size
	}
	return boxes
}

func findBox(data []byte, typ string) []byte {
	for _, b := range parseBoxes(data) {
		if b.typ == typ {
			return b.payload
		}
	}
	return nil
}

func findBoxes(data []byte, typ string) [][]byte {
	var out [][]byte
	for _, b := range parseBoxes(data) {
		if b.typ == typ {
			out = append(out, b.payload)
		}
	}
	return out
}
