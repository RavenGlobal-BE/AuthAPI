package databaseengine

import (
	"context"
	"time"

	"crypto/sha256"
	"encoding/hex"

	"github.com/redis/go-redis/v9"
)

type RedisClient struct {
	client *redis.Client
}

func CreateRedisClient(host string, password string, database int) *RedisClient {
	client := redis.NewClient(&redis.Options{
		Addr:     host,
		Password: password,
		DB:       database, // Default database used -> Contains all session data for the users
	})

	return &RedisClient{client: client}
}

func ConnectRedis() *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0, // Default database used -> Contains all session data for the users
	})

	return client
}

func SetupRedisSchema(client *redis.Client, ctx context.Context) error { //Strucureert de redis databases zodat er keys & values erin gegooid kunnen worden
	return nil
}

func SetInRedis(client *redis.Client, ctx context.Context, structure map[string]interface{}, expiry time.Duration) bool {
	hash := sha256.New()
	hash.Write([]byte(structure["refresh_token"].(string)))

	hashedToken := hex.EncodeToString(hash.Sum(nil))

	err := client.HSet(ctx, hashedToken, structure).Err()
	if err != nil {
		return false
	}

	err = client.Expire(ctx, hashedToken, expiry).Err()
	if err != nil {
		return false
	}

	return true
}
