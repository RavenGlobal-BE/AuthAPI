package databaseengine

/* This token engine utilizes Redis database 0. KEEP THAT IN MIND WHEN TRYING TO DEBUG */

type TokenRepo struct {
	redis *RedisClient
}

func NewTokenRepo(db *RedisClient) *TokenRepo {
	return &TokenRepo{redis: db}
}

func (Tr *TokenRepo) Init() error {
	Tr.redis = CreateRedisClient("localhost:6379", "", 0)
	return nil
}
