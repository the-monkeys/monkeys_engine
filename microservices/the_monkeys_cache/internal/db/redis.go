package db

import (
	"context"

	"github.com/redis/go-redis/v9"
	"github.com/the-monkeys/the_monkeys/config"
	"go.uber.org/zap"
)

var ctx = context.Background()
var rdb *redis.Client

func RedisConn(config *config.Config, log *zap.SugaredLogger) (*redis.Client, error) {
	rdb = redis.NewClient(&redis.Options{
		Addr:         config.Redis.Host,
		Password:     config.Redis.Password,
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 2,
	})

	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
		return nil, err
	}

	log.Infof("✅ the monkeys gateway is connected to redis at: %v", config.Redis.Host)
	return rdb, nil
}
