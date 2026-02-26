package databaseengine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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

func (tr *TokenRepo) GetTokenInfo(ctx context.Context, refreshToken string) (map[string]string, error) {
	hash := sha256.New()
	hash.Write([]byte(refreshToken))

	hashedToken := hex.EncodeToString(hash.Sum(nil))

	result, err := tr.redis.client.HGetAll(ctx, hashedToken).Result()
	if err != nil {
		return nil, err
	}

	return result, nil
}

// Stores an auth code in Redis with a 60s TTL.
// Key format: auth_code:<code>
func (tr *TokenRepo) InsertAuthCode(ctx context.Context, code string, data map[string]interface{}) error {
	key := "auth_code:" + code

	if err := tr.redis.client.HSet(ctx, key, data).Err(); err != nil {
		return err
	}

	return tr.redis.client.Expire(ctx, key, 60*time.Second).Err()
}

// Reads and immediately deletes an auth code from Redis (single-use).
// Returns an error if the code doesn't exist or has already expired.
func (tr *TokenRepo) UseAuthCode(ctx context.Context, code string) (map[string]string, error) {
	key := "auth_code:" + code

	result, err := tr.redis.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, errors.New("auth code not found or expired")
	}

	tr.redis.client.Del(ctx, key)

	return result, nil
}
