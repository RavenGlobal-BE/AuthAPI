package databaseengine

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

func ConnectRedis() *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0, // Default database used
	})

	return client
}

func SetInRedis(client *redis.Client, ctx context.Context, structure map[string]interface{}) bool {
	err := client.HSet(ctx, "sessions", structure).Err()
	if err != nil {
		return false
	}

	err = client.Expire(ctx, "sessions", 24*time.Hour).Err() // Set expiration time for the session
	if err != nil {
		return false
	}

	return true
}
