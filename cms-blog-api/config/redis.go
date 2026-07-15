package config

import (
	"context"
	"log"
	"os"
	"strconv"

	"github.com/redis/go-redis/v9"
)

var RedisClient *redis.Client

var Ctx = context.Background()

func ConnectRedis() {

	db, _ := strconv.Atoi(os.Getenv("REDIS_DB"))

	RedisClient = redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDR"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       db,
	})

	_, err := RedisClient.Ping(Ctx).Result()

	if err != nil {
		log.Fatal("❌ Redis Connection Failed :", err)
	}

	log.Println("✅ Redis Connected")
}