package databaseengine

import (
	"context"
	"fmt"
	"time"
)

type RateRepo struct {
	redis *RedisClient
}

func NewRateRepo(redis *RedisClient) *RateRepo {
	return &RateRepo{redis: redis}
}

func (rr *RateRepo) CheckLimit(ctx context.Context, address string) (int64, error) {
	key := fmt.Sprintf("ratelimit:%s", address) // Key format: "ratelimit:<IP_ADDRESS>" to track attempts per IP address

	count, err := rr.redis.client.Get(ctx, key).Int64()
	if err != nil {
		if err.Error() == "redis: nil" {
			return 0, nil
		}
		return 0, err
	}

	return count, nil
}

func (rr *RateRepo) Increment(ctx context.Context, address string, window time.Duration) (int64, error) {
	key := fmt.Sprintf("ratelimit:%s", address)

	count, err := rr.redis.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}

	// Set expiry only on the first attempt (count == 1) so the window doesn't reset on every request
	if count == 1 {
		rr.redis.client.Expire(ctx, key, window)
	}

	return count, nil
}
