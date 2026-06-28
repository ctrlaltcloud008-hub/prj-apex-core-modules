package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/ctrlaltcloud008-hub/prj-apex-core-modules/pkg/models"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
)

var (
	ErrStageNotFound           = errors.New("video stage not found")
	ErrStageTransitionConflict = errors.New("video stage transition conflicts with persisted state")
)

type LifeCycleEventParams struct {
	FromStatus models.Status
	ToStatus   models.Status
	Actor      string
	Reason     string
	Details    any
}

type StageRecordParams struct {
	VideoID     string
	Stage       models.Status
	Attempt     int64
	StartedAt   spanner.NullTime
	CompletedAt spanner.NullTime
	DurationMs  spanner.NullInt64
	Outcome     spanner.NullString
	ErrorID     spanner.NullString
	Actor       string
}

// StageTransitionParams describes the stage-record half of a video status
// transition. The caller remains responsible for changing videos.status,
// appending the lifecycle event, and writing any outbox entries in the same
// Spanner transaction.
type StageTransitionParams struct {
	VideoID        string
	FromStage      models.Status
	FromAttempt    int64
	ToStage        models.Status
	ToAttempt      int64
	TransitionedAt time.Time
	Outcome        string
	ErrorID        spanner.NullString
	Actor          string
}

type stageTransaction interface {
	ReadRow(context.Context, string, spanner.Key, []string) (*spanner.Row, error)
	BufferWrite([]*spanner.Mutation) error
}

type lifecycleEventInsert struct {
	EventSeq int64
	Params   LifeCycleEventParams
}

func AppendLifecycleEvents(ctx context.Context, txn *spanner.ReadWriteTransaction, videoID string, events ...LifeCycleEventParams) error {
	if len(events) == 0 {
		return nil
	}

	baseSeq, err := currentLifecycleEventSeq(ctx, txn, videoID)
	if err != nil {
		return err
	}

	assigned := assignLifecycleEventSeq(baseSeq, events)
	mutations := make([]*spanner.Mutation, 0, len(assigned)+1)
	for _, events := range assigned {
		mutations = append(mutations, spanner.Insert("video_lifecycle_events",
			[]string{"video_id", "event_seq", "from_status", "to_status", "actor", "reason", "details", "created_at"},
			[]any{videoID, events.EventSeq, events.Params.FromStatus, events.Params.ToStatus, events.Params.Actor, events.Params.Reason, events.Params.Details, spanner.CommitTimestamp},
		))
	}

	return txn.BufferWrite(mutations)
}

// currentLifecycleEventSeq scans the video's interleaved lifecycle events for the
// current max sequence. This is a local range scan within the parent row's split
// (a handful of rows per video) — deliberately not cached on the videos row, since
// a write-back mutation would conflict with callers that insert or update the
// videos row in the same transaction.
func currentLifecycleEventSeq(ctx context.Context, txn *spanner.ReadWriteTransaction, videoID string) (int64, error) {

	stmt := spanner.Statement{
		SQL: `SELECT IFNULL(MAX(event_seq), 0) as last_seq
			FROM video_lifecycle_events
			WHERE video_id = @videoID`,
		Params: map[string]any{
			"videoID": videoID,
		},
	}

	iter := txn.Query(ctx, stmt)
	defer iter.Stop()

	row, err := iter.Next()
	if err == iterator.Done {
		return 0, nil
	}

	if err != nil {
		return 0, fmt.Errorf("failed to query current lifecycle event sequence: %w", err)
	}

	var lastSeq int64
	if err := row.Columns(&lastSeq); err != nil {
		return 0, fmt.Errorf("failed to read current lifecycle event sequence: %w", err)
	}

	return lastSeq, nil
}

func assignLifecycleEventSeq(baseSeq int64, events []LifeCycleEventParams) []lifecycleEventInsert {
	assigned := make([]lifecycleEventInsert, 0, len(events))
	nextSeq := baseSeq
	for _, event := range events {
		nextSeq++
		assigned = append(assigned, lifecycleEventInsert{
			EventSeq: nextSeq,
			Params:   event,
		})
	}
	return assigned
}

func InsertVideoStageRecord(ctx context.Context, txn *spanner.ReadWriteTransaction, params StageRecordParams) error {
	mutation := spanner.Insert("video_stages",
		[]string{"video_id", "stage", "attempt", "started_at", "completed_at", "duration_ms", "outcome", "error_id", "actor"},
		[]any{params.VideoID, params.Stage, params.Attempt, params.StartedAt, params.CompletedAt, params.DurationMs, params.Outcome, params.ErrorID, params.Actor},
	)

	return txn.BufferWrite([]*spanner.Mutation{mutation})
}

// TransitionVideoStage atomically completes the current stage record and
// starts the next one. Replaying an already-persisted transition is a no-op.
func TransitionVideoStage(ctx context.Context, txn *spanner.ReadWriteTransaction, params StageTransitionParams) error {
	return transitionVideoStage(ctx, txn, params)
}

func transitionVideoStage(ctx context.Context, txn stageTransaction, params StageTransitionParams) error {
	if err := validateStageTransition(params); err != nil {
		return err
	}

	startedAt, completedAt, currentExists, err := readStageTiming(
		ctx, txn, params.VideoID, params.FromStage, params.FromAttempt,
	)
	if err != nil {
		return fmt.Errorf("read current stage: %w", err)
	}
	if !currentExists {
		return fmt.Errorf("%w: video_id=%s stage=%s attempt=%d",
			ErrStageNotFound, params.VideoID, params.FromStage, params.FromAttempt)
	}

	_, _, nextExists, err := readStageTiming(
		ctx, txn, params.VideoID, params.ToStage, params.ToAttempt,
	)
	if err != nil {
		return fmt.Errorf("read next stage: %w", err)
	}

	if completedAt.Valid {
		if nextExists {
			return nil
		}
		return fmt.Errorf("%w: current stage is complete but next stage is missing",
			ErrStageTransitionConflict)
	}
	if nextExists {
		return fmt.Errorf("%w: next stage exists while current stage is incomplete",
			ErrStageTransitionConflict)
	}
	if params.TransitionedAt.Before(startedAt) {
		return fmt.Errorf("transition time %s precedes stage start %s",
			params.TransitionedAt.Format(time.RFC3339Nano), startedAt.Format(time.RFC3339Nano))
	}

	mutations := []*spanner.Mutation{
		spanner.Update("video_stages",
			[]string{"video_id", "stage", "attempt", "completed_at", "duration_ms", "outcome", "error_id"},
			[]any{
				params.VideoID, string(params.FromStage), params.FromAttempt,
				params.TransitionedAt, params.TransitionedAt.Sub(startedAt).Milliseconds(),
				params.Outcome, params.ErrorID,
			},
		),
		spanner.Insert("video_stages",
			[]string{"video_id", "stage", "attempt", "started_at", "completed_at", "duration_ms", "outcome", "error_id", "actor"},
			[]any{
				params.VideoID, params.ToStage, params.ToAttempt,
				spanner.NullTime{Time: params.TransitionedAt, Valid: true},
				spanner.NullTime{}, spanner.NullInt64{}, spanner.NullString{}, spanner.NullString{}, params.Actor,
			},
		),
	}

	if err := txn.BufferWrite(mutations); err != nil {
		return fmt.Errorf("buffer stage transition: %w", err)
	}
	return nil
}

func readStageTiming(
	ctx context.Context,
	txn stageTransaction,
	videoID string,
	stage models.Status,
	attempt int64,
) (time.Time, spanner.NullTime, bool, error) {
	row, err := txn.ReadRow(ctx, "video_stages", spanner.Key{videoID, string(stage), attempt},
		[]string{"started_at", "completed_at"})
	if err != nil {
		if spanner.ErrCode(err) == codes.NotFound {
			return time.Time{}, spanner.NullTime{}, false, nil
		}
		return time.Time{}, spanner.NullTime{}, false, err
	}

	var startedAt time.Time
	var completedAt spanner.NullTime
	if err := row.Columns(&startedAt, &completedAt); err != nil {
		return time.Time{}, spanner.NullTime{}, false, err
	}
	return startedAt, completedAt, true, nil
}

func validateStageTransition(params StageTransitionParams) error {
	switch {
	case params.VideoID == "":
		return errors.New("video_id is required")
	case params.FromStage == "":
		return errors.New("from_stage is required")
	case params.FromAttempt < 1:
		return errors.New("from_attempt must be positive")
	case params.ToStage == "":
		return errors.New("to_stage is required")
	case params.ToAttempt < 1:
		return errors.New("to_attempt must be positive")
	case params.TransitionedAt.IsZero():
		return errors.New("transitioned_at is required")
	case params.Outcome == "":
		return errors.New("outcome is required")
	case params.Actor == "":
		return errors.New("actor is required")
	default:
		return nil
	}
}
