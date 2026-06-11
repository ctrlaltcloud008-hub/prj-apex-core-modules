package quota

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const hourlyUploadTTL = time.Hour

// incrHourlyScript atomically increments the hourly counter and sets a 1-hour
// TTL only on first creation, preventing TTL resets on subsequent increments.
var incrHourlyScript = redis.NewScript(`
local count = redis.call('INCR', KEYS[1])
if count == 1 then
  redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return count
`)

func concurrentKey(userID string) string {
	return fmt.Sprintf("quota:concurrent:%s", userID)
}

func hourlyKey(userID string) string {
	return fmt.Sprintf("quota:hourly:%s", userID)
}

// IncrConcurrentUploads increments the in-flight upload counter for a user and
// returns the new count. Call DecrConcurrentUploads when the upload reaches a
// terminal status, or immediately if the limit check fails.
func IncrConcurrentUploads(ctx context.Context, rdb *redis.Client, userID string) (int64, error) {
	count, err := rdb.Incr(ctx, concurrentKey(userID)).Result()
	if err != nil {
		return 0, fmt.Errorf("incr concurrent uploads for user %q: %w", userID, err)
	}
	return count, nil
}

// DecrConcurrentUploads decrements the in-flight upload counter for a user.
// It clamps the counter at zero to guard against double-decrement bugs.
func DecrConcurrentUploads(ctx context.Context, rdb *redis.Client, userID string) error {
	key := concurrentKey(userID)
	val, err := rdb.Decr(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("decr concurrent uploads for user %q: %w", userID, err)
	}
	if val < 0 {
		rdb.Set(ctx, key, 0, 0)
	}
	return nil
}

// IncrHourlyUploads increments the per-user hourly upload counter and returns
// the new count. A 1-hour TTL is set atomically on first creation only.
func IncrHourlyUploads(ctx context.Context, rdb *redis.Client, userID string) (int64, error) {
	result, err := incrHourlyScript.Run(
		ctx, rdb,
		[]string{hourlyKey(userID)},
		int(hourlyUploadTTL.Seconds()),
	).Int64()
	if err != nil {
		return 0, fmt.Errorf("incr hourly uploads for user %q: %w", userID, err)
	}
	return result, nil
}
