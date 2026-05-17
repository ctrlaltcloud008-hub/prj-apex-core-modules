package models

import (
	"errors"
	"fmt"
)

type UserTier string

const (
	UserTierFree     UserTier = "FREE"
	UserTierStandard UserTier = "STANDARD"
	UserTierPremium  UserTier = "PREMIUM"
)

var ErrInvalidUserTier = errors.New("invalid user tier")

type TierLimits struct {
	MaxConcurrentUploads     int64
	MaxUploadsPerHour        int64
	MaxFileSizeBytes         int64
	StorageQuotaBytes        int64
	SignedURLExpirationHours int64
}

var TierLimitsMap = map[UserTier]TierLimits{
	UserTierFree: {
		MaxConcurrentUploads:     3,
		MaxUploadsPerHour:        15,
		MaxFileSizeBytes:         5 * 1024 * 1024 * 1024,  // 5 GB
		StorageQuotaBytes:        50 * 1024 * 1024 * 1024, // 50 GB
		SignedURLExpirationHours: 2,
	},
	UserTierStandard: {
		MaxConcurrentUploads:     10,
		MaxUploadsPerHour:        50,
		MaxFileSizeBytes:         20 * 1024 * 1024 * 1024,  // 20 GB
		StorageQuotaBytes:        500 * 1024 * 1024 * 1024, // 500 GB
		SignedURLExpirationHours: 24,
	},
	UserTierPremium: {
		MaxConcurrentUploads:     50,
		MaxUploadsPerHour:        200,
		MaxFileSizeBytes:         100 * 1024 * 1024 * 1024,      // 100 GB
		StorageQuotaBytes:        5 * 1024 * 1024 * 1024 * 1024, // 5 TB
		SignedURLExpirationHours: 24,
	},
}

func GetTierLimits(userTier UserTier) (TierLimits, error) {
	limits, ok := TierLimitsMap[userTier]
	if !ok {
		return TierLimits{}, fmt.Errorf("%w %q", ErrInvalidUserTier, userTier)
	}

	return limits, nil
}
