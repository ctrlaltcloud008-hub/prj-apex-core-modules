package retry

import (
	"context"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/ctrlaltcloud008-hub/prj-apex-core-modules/pkg/apperror"
)

func RetryWithBackoff(ctx context.Context, maxRetries uint, fn func() error) error {
	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = 1 * time.Second
	bo.MaxInterval = 16 * time.Second
	bo.Multiplier = 2

	_, err := backoff.Retry(ctx, func() (struct{}, error) {
		err := fn()
		if err == nil {
			return struct{}{}, nil
		}
		if apperror.Classify(err) != apperror.Transient {
			return struct{}{}, backoff.Permanent(err)
		}

		return struct{}{}, err
	}, backoff.WithBackOff(bo), backoff.WithMaxTries(uint(maxRetries+1)))

	return err
}
