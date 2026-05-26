package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"

	"cloud.google.com/go/spanner"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
)

const DefaultShardCount = 4

type Entry struct {
	EntryID string
	VideoID string
	Topic   string
	Payload any
}

type Envelope struct {
	Traceparent string `json:"traceparent,omitempty"`
	Tracestate  string `json:"tracestate,omitempty"`
	Data        any    `json:"data"`
}

func BuildEnvelope(ctx context.Context, payload any) Envelope {
	tp, ts := extractTrace(ctx)
	return Envelope{
		Traceparent: tp,
		Tracestate:  ts,
		Data:        payload,
	}
}

func ParseEnvelope(payload []byte) (Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(payload, &env); err != nil {
		return Envelope{}, fmt.Errorf("unmarshal outbox envelope: %w", err)
	}
	return env, nil
}

func Write(ctx context.Context, txn *spanner.ReadWriteTransaction, entries []Entry) error {
	if len(entries) == 0 {
		return fmt.Errorf("no entries to write")
	}

	mutations := make([]*spanner.Mutation, 0, len(entries))
	for i, e := range entries {
		if e.VideoID == "" {
			return fmt.Errorf("entry %d: missing video ID", i)
		}
		if e.Topic == "" {
			return fmt.Errorf("entry %d: missing topic", i)
		}
		if e.Payload == nil {
			return fmt.Errorf("entry %d: missing payload", i)
		}

		entryID := e.EntryID
		if entryID == "" {
			id, err := uuid.NewV7()
			if err != nil {
				return fmt.Errorf("entry %d: failed to generate entry ID: %w", i, err)
			}
			entryID = id.String()
		}

		env := BuildEnvelope(ctx, e.Payload)

		if _, err := json.Marshal(env); err != nil {
			return fmt.Errorf("entry %d: failed to marshal payload: %w", i, err)
		}

		m := spanner.Insert("outbox",
			[]string{"entry_id", "shard_id", "video_id", "topic", "payload", "status", "created_at"},
			[]any{
				entryID,
				shardID(entryID),
				e.VideoID,
				e.Topic,
				spanner.NullJSON{Value: env, Valid: true},
				"PENDING",
				spanner.CommitTimestamp,
			},
		)
		mutations = append(mutations, m)
	}

	return txn.BufferWrite(mutations)
}

func shardID(entryID string) int64 {
	h := fnv.New32a()
	h.Write([]byte(entryID))
	return int64(h.Sum32() % DefaultShardCount)
}

type mapCarrier map[string]string

func (c mapCarrier) Get(key string) string { return c[key] }
func (c mapCarrier) Set(key, value string) { c[key] = value }
func (c mapCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

func extractTrace(ctx context.Context) (string, string) {
	carrier := make(mapCarrier)
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	return carrier["traceparent"], carrier["tracestate"]
}
