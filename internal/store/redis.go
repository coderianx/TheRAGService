package store

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"
)

var RedisClient = redis.NewClient(&redis.Options{
	Addr:     "localhost:6379",
	Password: "",
	DB:       0,
})

func ConnectRedis() {
	if err := RedisClient.Ping(context.Background()).Err(); err != nil {
		log.Fatal(err)
	}
}
