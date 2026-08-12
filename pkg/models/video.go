package models

import (
	"time"

	"cloud.google.com/go/spanner"
)

type Video struct {
	VideoID       string              `spanner:"video_id"`
	UserID        string              `spanner:"user_id"`
	RequestID     spanner.NullString  `spanner:"request_id"`
	Status        Status              `spanner:"status"`
	UserTier      UserTier            `spanner:"user_tier"`
	SourceBucket  string              `spanner:"source_bucket"`
	SourceObject  string              `spanner:"source_object"`
	GCSGeneration int64               `spanner:"gcs_generation"`
	MimeType      spanner.NullString  `spanner:"mime_type"`
	FileSizeBytes spanner.NullInt64   `spanner:"file_size_bytes"`
	DurationMs    spanner.NullInt64   `spanner:"duration_ms"`
	SourceWidth   spanner.NullInt64   `spanner:"source_width"`
	SourceHeight  spanner.NullInt64   `spanner:"source_height"`
	SourceCodec   spanner.NullString  `spanner:"source_codec"`
	SourceFPS     spanner.NullFloat64 `spanner:"source_fps"`
	IsHDR         spanner.NullBool    `spanner:"is_hdr"`
	HasAudio      spanner.NullBool    `spanner:"has_audio"`

	// ErrorDetails is the one column expected to be NULL on a healthy video: it
	// is only written when a stage fails.
	ErrorDetails spanner.NullJSON `spanner:"error_details"`
	CreatedAt    time.Time        `spanner:"created_at"`
	UpdatedAt    time.Time        `spanner:"updated_at"`
}

type VideoValidatedPayload struct {
	VideoID          string                        `json:"video_id"`
	UserID           string                        `json:"user_id"`
	SourceGCSURI     string                        `json:"source_gcs_uri"`
	TranscodeProfile string                        `json:"transcode_profile"`
	SourceRegion     string                        `json:"source_region"`
	GCSGeneration    int64                         `json:"gcs_generation"`
	Metadata         VideoValidatedMetadataPayload `json:"metadata"`
}

type VideoValidatedMetadataPayload struct {
	DurationMs  int64   `json:"duration_ms"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	Codec       string  `json:"codec"`
	FPS         float64 `json:"fps"`
	IsHDR       bool    `json:"is_hdr"`
	BitrateKbps int64   `json:"bitrate_kbps"`
}
