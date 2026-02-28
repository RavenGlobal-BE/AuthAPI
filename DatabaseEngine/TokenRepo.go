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

func (tr *TokenRepo) InsertToken(ctx context.Context, key string, structure map[string]interface{}, expiry time.Duration) bool {
	hash := sha256.New()
	hash.Write([]byte(key))

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

// Blacklists the refresh token
func (tr *TokenRepo) BlacklistToken(ctx context.Context, key string) bool {
	hash := sha256.New()
	hash.Write([]byte(key))
	hashedToken := hex.EncodeToString(hash.Sum(nil))

	current, err := tr.redis.client.HGet(ctx, hashedToken, "blacklisted").Result()
	if err != nil {
		return false // token doesn't exist
	}

	var newValue string
	if current == "0" {
		newValue = "1"
	} else {
		newValue = "0"
	}

	return tr.redis.client.HSet(ctx, hashedToken, "blacklisted", newValue).Err() == nil
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
func (tr *TokenRepo) InsertAuthCode(ctx context.Context, code string, data map[string]interface{}) error {
	key := "auth_code:" + code

	if err := tr.redis.client.HSet(ctx, key, data).Err(); err != nil {
		return err
	}

	return tr.redis.client.Expire(ctx, key, 60*time.Second).Err()
}

// Reads and immediately deletes an auth code from Redis (single-use).
// Returns an error if the code doesn't exist or has already expired.
func (tr *TokenRepo) GetAuthCode(ctx context.Context, code string) (map[string]string, error) {
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

// Adds a token's jti to the revocation blocklist (access token).
func (tr *TokenRepo) RevokeToken(ctx context.Context, jti string, ttl time.Duration) error {
	return tr.redis.client.Set(ctx, "revoked:"+jti, "1", ttl).Err()
}

// Returns true if the token's jti has been revoked.
func (tr *TokenRepo) IsRevoked(ctx context.Context, jti string) bool {
	result, _ := tr.redis.client.Exists(ctx, "revoked:"+jti).Result()
	return result > 0
}
