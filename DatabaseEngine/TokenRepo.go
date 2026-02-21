package databaseengine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

/* This token engine utilizes Redis database 0. KEEP THAT IN MIND WHEN TRYING TO DEBUG */

type TokenRepo struct {
	redis *RedisClient
}

func NewTokenRepo(db *RedisClient) *TokenRepo {
	return &TokenRepo{redis: db}
}

func (tr *TokenRepo) InsertToken(ctx context.Context, structure map[string]interface{}, expiry time.Duration) bool {
	hash := sha256.New()
	hash.Write([]byte(structure["refresh_token"].(string)))

	hashedToken := hex.EncodeToString(hash.Sum(nil))

	err := tr.redis.client.HSet(ctx, hashedToken, structure).Err()
	if err != nil {
		return false
	}

	err = tr.redis.client.Expire(ctx, hashedToken, expiry).Err()
	if err != nil {
		return false
	}

	return true
}
