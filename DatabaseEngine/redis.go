package databaseengine

import (
	"context"

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

func SetupRedisSchema(client *redis.Client, ctx context.Context) error { //Strucureert de redis databases zodat er keys & values erin gegooid kunnen worden
	return nil
}
