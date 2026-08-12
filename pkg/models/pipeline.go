package models

// This file is the wire contract between pipeline services. Every type here is
// serialised into an outbox payload by one service and deserialised by another,
// so the producer and consumer must agree on the JSON tags exactly. Keeping the
// definitions in one place is the point: when these lived in each service's
// internal/models they had already drifted (the split service's LoudnessParams
// carried no JSON tags at all, so it would have marshalled as "MeasuredI"
// where the worker expected "measured_i").
//
// Topic ownership:
//
//	video.received              ingestion    -> probe
//	video.validated             probe        -> split
//	video.split                 split        -> orchestrator
//	transcode.normal|priority   orchestrator -> worker
//	transcode.completed         worker       -> orchestrator
//	rendition.stitch.requested  orchestrator -> stitch
//	rendition.stitched          stitch       -> orchestrator
//	video.transcoded            orchestrator -> packager

// RenditionSpec is one rung of the ABR ladder. It is stored on
// videos.rendition_ladder as JSON and embedded in every encode work item.
type RenditionSpec struct {
	Name              string `json:"name"`
	Width             int    `json:"width"`
	Height            int    `json:"height"`
	TargetBitrateKbps int64  `json:"target_bitrate_kbps"`
	Codec             string `json:"codec"`
	IsHDR             bool   `json:"is_hdr"`
}

// LoudnessParams are the loudnorm pass-1 measurements taken over the whole
// source by the split service.
//
// Loudness is deliberately measured once for the entire source rather than per
// chunk: every chunk encode replays these values with linear=true so all chunks
// receive one identical gain. Measuring per chunk would normalise each chunk to
// its own level and produce an audible step at every stitch boundary.
type LoudnessParams struct {
	MeasuredI      float64 `json:"measured_i"`
	MeasuredTP     float64 `json:"measured_tp"`
	MeasuredLRA    float64 `json:"measured_lra"`
	MeasuredThresh float64 `json:"measured_thresh"`
}

// Chunk is one stream-copy segment of the source, written once by the split
// service and encoded independently for every rendition. Durations vary because
// the copy cuts on source keyframes; StartMs/DurationMs are authoritative and
// the stitch service validates encoded output against them.
type Chunk struct {
	Idx        int64  `json:"idx"`
	StartMs    int64  `json:"start_ms"`
	DurationMs int64  `json:"duration_ms"`
	GCSURI     string `json:"gcs_uri"`
}

// VideoSplitPayload is emitted on video.split once the source has been chunked
// and the video_chunks rows are committed. The orchestrator reads the ladder and
// the chunk rows from Spanner, so the payload only identifies the video.
type VideoSplitPayload struct {
	VideoID    string `json:"video_id"`
	ChunkCount int64  `json:"chunk_count"`
}

// TranscodeJobRequestedPayload is one unit of encode work: a single chunk of a
// single rendition. SourceGCSURI points at the chunk written by the split
// service, never at the original upload.
type TranscodeJobRequestedPayload struct {
	VideoID         string         `json:"video_id"`
	RenditionName   string         `json:"rendition_name"`
	ChunkIdx        int64          `json:"chunk_idx"`
	Attempt         int64          `json:"attempt"`
	SourceGCSURI    string         `json:"source_gcs_uri"`
	OutputGCSPrefix string         `json:"output_gcs_prefix"`
	HasAudio        bool           `json:"has_audio"`
	Loudness        LoudnessParams `json:"loudness"`
	RenditionSpec
}

// TranscodeJobCompletedPayload reports the outcome of one chunk encode.
type TranscodeJobCompletedPayload struct {
	VideoID         string `json:"video_id"`
	RenditionName   string `json:"rendition_name"`
	ChunkIdx        int64  `json:"chunk_idx"`
	Attempt         int64  `json:"attempt"`
	Status          string `json:"status"`
	WorkerID        string `json:"worker_id"`
	OutputGCSPrefix string `json:"output_gcs_prefix"`
	ErrorMessage    string `json:"error_message,omitempty"`
}

// RenditionStitchRequestedPayload is emitted once every chunk job for one
// rendition has COMPLETED.
type RenditionStitchRequestedPayload struct {
	VideoID       string `json:"video_id"`
	RenditionName string `json:"rendition_name"`
}

// RenditionStitchedPayload is emitted after a rendition is assembled
// (Failed=false) or fails validation (Failed=true). The orchestrator counts
// successes toward the video-transcoded transition and treats a failure as a
// rendition-level failure.
type RenditionStitchedPayload struct {
	VideoID       string `json:"video_id"`
	RenditionName string `json:"rendition_name"`
	PlaylistURI   string `json:"playlist_uri,omitempty"`
	Failed        bool   `json:"failed,omitempty"`
	Error         string `json:"error,omitempty"`
}

// VideoTranscodedPayload is emitted once every rendition has been stitched. It
// is the packager's trigger to build the master playlist.
type VideoTranscodedPayload struct {
	VideoID string `json:"video_id"`
}

// VideoPackagedPayload is emitted once the master playlist exists and the video
// is playable end to end.
type VideoPackagedPayload struct {
	VideoID     string `json:"video_id"`
	PlaybackURI string `json:"playback_uri"`
}
