package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/ctrlaltcloud008-hub/prj-apex-core-modules/pkg/models"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeStageTransaction struct {
	rows      map[string]*spanner.Row
	mutations []*spanner.Mutation
}

func (f *fakeStageTransaction) ReadRow(_ context.Context, _ string, key spanner.Key, _ []string) (*spanner.Row, error) {
	row, ok := f.rows[stageKey(key[0].(string), key[1].(string), key[2].(int64))]
	if !ok {
		return nil, status.Error(codes.NotFound, "stage not found")
	}
	return row, nil
}

func (f *fakeStageTransaction) BufferWrite(mutations []*spanner.Mutation) error {
	f.mutations = append(f.mutations, mutations...)
	return nil
}

func TestTransitionVideoStageCompletesCurrentAndStartsNext(t *testing.T) {
	startedAt := time.Date(2026, 6, 28, 6, 1, 56, 0, time.UTC)
	transitionedAt := startedAt.Add(32 * time.Second)
	currentRow, err := spanner.NewRow(
		[]string{"started_at", "completed_at"},
		[]any{startedAt, spanner.NullTime{}},
	)
	if err != nil {
		t.Fatal(err)
	}

	tx := &fakeStageTransaction{rows: map[string]*spanner.Row{
		stageKey("video-1", string(models.StatusValidating), 1): currentRow,
	}}
	params := StageTransitionParams{
		VideoID:        "video-1",
		FromStage:      models.StatusValidating,
		FromAttempt:    1,
		ToStage:        models.StatusValidated,
		ToAttempt:      1,
		TransitionedAt: transitionedAt,
		Outcome:        "SUCCEEDED",
		Actor:          "probe",
	}

	if err := transitionVideoStage(context.Background(), tx, params); err != nil {
		t.Fatalf("transitionVideoStage() error = %v", err)
	}

	want := []*spanner.Mutation{
		spanner.Update("video_stages",
			[]string{"video_id", "stage", "attempt", "completed_at", "duration_ms", "outcome", "error_id"},
			[]any{"video-1", string(models.StatusValidating), int64(1), transitionedAt, int64(32000), "SUCCEEDED", spanner.NullString{}},
		),
		spanner.Insert("video_stages",
			[]string{"video_id", "stage", "attempt", "started_at", "completed_at", "duration_ms", "outcome", "error_id", "actor"},
			[]any{"video-1", models.StatusValidated, int64(1), spanner.NullTime{Time: transitionedAt, Valid: true}, spanner.NullTime{}, spanner.NullInt64{}, spanner.NullString{}, spanner.NullString{}, "probe"},
		),
	}
	if !reflect.DeepEqual(tx.mutations, want) {
		t.Fatalf("mutations mismatch\n got: %#v\nwant: %#v", tx.mutations, want)
	}
}

func TestTransitionVideoStageIsIdempotentAfterCompletedTransition(t *testing.T) {
	startedAt := time.Date(2026, 6, 28, 6, 1, 56, 0, time.UTC)
	completedAt := spanner.NullTime{Time: startedAt.Add(32 * time.Second), Valid: true}
	currentRow, _ := spanner.NewRow([]string{"started_at", "completed_at"}, []any{startedAt, completedAt})
	nextRow, _ := spanner.NewRow([]string{"started_at", "completed_at"}, []any{completedAt.Time, spanner.NullTime{}})
	tx := &fakeStageTransaction{rows: map[string]*spanner.Row{
		stageKey("video-1", string(models.StatusValidating), 1): currentRow,
		stageKey("video-1", string(models.StatusValidated), 1):  nextRow,
	}}

	err := transitionVideoStage(context.Background(), tx, StageTransitionParams{
		VideoID: "video-1", FromStage: models.StatusValidating, FromAttempt: 1,
		ToStage: models.StatusValidated, ToAttempt: 1, TransitionedAt: completedAt.Time,
		Outcome: "SUCCEEDED", Actor: "probe",
	})
	if err != nil {
		t.Fatalf("transitionVideoStage() error = %v", err)
	}
	if len(tx.mutations) != 0 {
		t.Fatalf("idempotent transition buffered %d mutations", len(tx.mutations))
	}
}

func TestTransitionVideoStageRejectsTransitionBeforeStart(t *testing.T) {
	startedAt := time.Date(2026, 6, 28, 6, 1, 56, 0, time.UTC)
	currentRow, _ := spanner.NewRow([]string{"started_at", "completed_at"}, []any{startedAt, spanner.NullTime{}})
	tx := &fakeStageTransaction{rows: map[string]*spanner.Row{
		stageKey("video-1", string(models.StatusValidating), 1): currentRow,
	}}

	err := transitionVideoStage(context.Background(), tx, StageTransitionParams{
		VideoID: "video-1", FromStage: models.StatusValidating, FromAttempt: 1,
		ToStage: models.StatusValidated, ToAttempt: 1, TransitionedAt: startedAt.Add(-time.Second),
		Outcome: "SUCCEEDED", Actor: "probe",
	})
	if err == nil {
		t.Fatal("transitionVideoStage() error = nil, want transition-before-start error")
	}
	if len(tx.mutations) != 0 {
		t.Fatalf("invalid transition buffered %d mutations", len(tx.mutations))
	}
}

func TestTransitionVideoStageRejectsMissingCurrentStage(t *testing.T) {
	tx := &fakeStageTransaction{rows: map[string]*spanner.Row{}}
	err := transitionVideoStage(context.Background(), tx, StageTransitionParams{
		VideoID: "video-1", FromStage: models.StatusValidating, FromAttempt: 1,
		ToStage: models.StatusValidated, ToAttempt: 1, TransitionedAt: time.Now(),
		Outcome: "SUCCEEDED", Actor: "probe",
	})
	if err == nil || !errors.Is(err, ErrStageNotFound) {
		t.Fatalf("transitionVideoStage() error = %v, want ErrStageNotFound", err)
	}
}

func stageKey(videoID, stage string, attempt int64) string {
	return fmt.Sprintf("%s/%s/%d", videoID, stage, attempt)
}
