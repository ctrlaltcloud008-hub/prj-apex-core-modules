package lifecycle

import (
	"context"
	"fmt"

	"cloud.google.com/go/spanner"
	"github.com/ctrlaltcloud008-hub/prj-apex-core-modules/pkg/models"
	"google.golang.org/api/iterator"
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
