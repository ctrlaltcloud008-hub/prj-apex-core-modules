package quota

import (
	"context"
	"fmt"

	"cloud.google.com/go/spanner"
	"github.com/ctrlaltcloud008-hub/prj-apex-core-modules/pkg/models"

	"google.golang.org/api/iterator"
)

func CheckConcurrentLimit(ctx context.Context, txn *spanner.ReadWriteTransaction, userID string) (int64, error) {

	stmt := spanner.Statement{
		SQL: `SELECT COUNT(1) as active_uploads
				FROM videos
				WHERE user_id = @userID AND status = @status`,
		Params: map[string]any{
			"userID": userID,
			"status": models.StatusUploading,
		},
	}

	iter := txn.Query(ctx, stmt)
	defer iter.Stop()

	row, err := iter.Next()
	if err == iterator.Done {
		return 0, nil
	}

	if err != nil {
		return 0, fmt.Errorf("query active uploads for concurrent limit check: %w", err)
	}

	var activeUploads int64
	if err := row.ColumnByName("active_uploads", &activeUploads); err != nil {
		return 0, fmt.Errorf("read active uploads count: %w", err)
	}

	return activeUploads, nil
}

func CheckHourlyRateLimit(ctx context.Context, txn *spanner.ReadWriteTransaction, userID string) (int64, error) {

	stmt := spanner.Statement{
		SQL: `SELECT COUNT(1) as uploads_last_hour
				FROM videos
				WHERE user_id = @userID AND status = @status AND created_at >= TIMESTAMP_SUB(CURRENT_TIMESTAMP(), INTERVAL 1 HOUR)`,
		Params: map[string]any{
			"userID": userID,
			"status": models.StatusUploading,
		},
	}

	iter := txn.Query(ctx, stmt)
	defer iter.Stop()

	row, err := iter.Next()
	if err == iterator.Done {
		return 0, nil
	}

	if err != nil {
		return 0, fmt.Errorf("query uploads in last hour for rate limit check: %w", err)
	}

	var uploadsLastHour int64
	if err := row.ColumnByName("uploads_last_hour", &uploadsLastHour); err != nil {
		return 0, fmt.Errorf("read uploads last hour count: %w", err)
	}

	return uploadsLastHour, nil
}

func CheckStorageQuota(ctx context.Context, txn *spanner.ReadWriteTransaction, userID string) (int64, error) {
	stmt := spanner.Statement{
		SQL: `SELECT IFNULL(SUM(file_size_bytes), 0) as total_storage_used
				FROM videos
				WHERE user_id = @userID AND status IN (@status_failed, @status_expired)`,
		Params: map[string]any{
			"userID":         userID,
			"status_failed":  models.StatusFailed,
			"status_expired": models.StatusExpired,
		},
	}

	iter := txn.Query(ctx, stmt)
	defer iter.Stop()

	row, err := iter.Next()
	if err == iterator.Done {
		return 0, nil
	}

	if err != nil {
		return 0, fmt.Errorf("query total storage used for storage quota check: %w", err)
	}

	var totalStorageUsed int64
	if err := row.ColumnByName("total_storage_used", &totalStorageUsed); err != nil {
		return 0, fmt.Errorf("read total storage used: %w", err)
	}

	return totalStorageUsed, nil
}
