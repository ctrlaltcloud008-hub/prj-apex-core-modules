package spanner

import (
	"context"
	"time"

	"cloud.google.com/go/spanner"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
)

type Config struct {
	NumChannels           int
	EnableEndToEndTracing bool
	DatabaseRole          string
	DisableRouteToLeader  bool
}

func DefaultConfig() Config {
	return Config{
		NumChannels:           4,
		EnableEndToEndTracing: true,
	}
}

type ReadOnlyOption func(*roConfig)

type roConfig struct {
	staleness time.Duration
}

func WithMaxStaleness(staleness time.Duration) ReadOnlyOption {
	return func(cfg *roConfig) {
		cfg.staleness = staleness
	}
}

func HealthCheck(ctx context.Context, client *spanner.Client) error {
	iter := client.Single().Query(ctx, spanner.NewStatement("SELECT 1"))
	defer iter.Stop()

	_, err := iter.Next()
	return err
}

func RunRW(
	ctx context.Context,
	client *spanner.Client,
	f func(ctx context.Context, tx *spanner.ReadWriteTransaction) error) (spanner.CommitResponse, error) {
	resp, err := client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		return f(ctx, tx)
	}, spanner.TransactionOptions{})

	return resp, err
}

func RunRO(
	ctx context.Context,
	client *spanner.Client,
	f func(ctx context.Context, tx *spanner.ReadOnlyTransaction) error,
	opts ...ReadOnlyOption) error {

	var cfg roConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	var txn *spanner.ReadOnlyTransaction
	if cfg.staleness > 0 {
		txn = client.ReadOnlyTransaction().WithTimestampBound(spanner.MaxStaleness(cfg.staleness))
	} else {
		txn = client.ReadOnlyTransaction()
	}

	defer txn.Close()

	return f(ctx, txn)
}

func IsAborted(err error) bool {
	return spanner.ErrCode(err) == codes.Aborted
}
