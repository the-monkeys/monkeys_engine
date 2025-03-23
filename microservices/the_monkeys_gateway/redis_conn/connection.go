package redis_conn

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/config"
)

var ctx = context.Background()
var rdb *redis.Client

func RedisConn(config *config.Config) (*redis.Client, error) {
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

	logrus.Infof("✅ the monkeys gateway is connected to redis at: %v", config.Redis.Host)
	return rdb, nil
}
