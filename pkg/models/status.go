package models

import "fmt"

type Status string

const (
	StatusUploading    Status = "UPLOADING"
	StatusValidating   Status = "VALIDATING"
	StatusValidated    Status = "VALIDATED"
	StatusTranscoding  Status = "TRANSCODING"
	StatusTranscoded   Status = "TRANSCODED"
	StatusFanOut       Status = "FAN_OUT"
	StatusThumbnailing Status = "THUMBNAILING"
	StatusTranscribing Status = "TRANSCRIBING"
	StatusModerating   Status = "MODERATING"
	StatusPublishGate  Status = "PUBLISH_GATE"
	StatusReady        Status = "READY"
	StatusRejected     Status = "REJECTED"
	StatusFailed       Status = "FAILED"
	StatusExpired      Status = "EXPIRED"
)

func (s Status) Validate() error {

	switch s {
	case StatusUploading, StatusValidating, StatusValidated, StatusTranscoding, StatusTranscoded,
		StatusFanOut, StatusThumbnailing, StatusTranscribing, StatusModerating, StatusPublishGate,
		StatusReady, StatusRejected, StatusFailed, StatusExpired:
		return nil
	default:
		return fmt.Errorf("invalid status (%q)", s)
	}
}

func (s Status) IsTerminal() bool {
	switch s {
	case StatusReady, StatusRejected, StatusFailed, StatusExpired:
		return true
	default:
		return false
	}
}
