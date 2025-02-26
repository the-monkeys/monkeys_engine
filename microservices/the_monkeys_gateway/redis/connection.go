package redis

import (
	"context"
	"fmt"
	"log"
	"time"

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

func loadBlogMetadata(blogID string, title string, author string, publishedDate string) {
	key := fmt.Sprintf("blog:%s:metadata", blogID)
	err := rdb.HMSet(ctx, key, map[string]interface{}{
		"title":         title,
		"author":        author,
		"publishedDate": publishedDate,
	}).Err()
	if err != nil {
		log.Printf("Error loading blog metadata: %v", err)
	}
}

func addBlogContent(blogID string, content string) {
	key := fmt.Sprintf("blog:%s:content", blogID)
	err := rdb.Set(ctx, key, content, 0).Err()
	if err != nil {
		log.Printf("Error adding blog content: %v", err)
	}
}

func addUserProfile(userID string, name string, email string, bio string) {
	key := fmt.Sprintf("user:%s:profile", userID)
	err := rdb.HMSet(ctx, key, map[string]interface{}{
		"name":  name,
		"email": email,
		"bio":   bio,
	}).Err()
	if err != nil {
		log.Printf("Error adding user profile: %v", err)
	}
}

func fetchAllData(blogID string, userID string) {
	metadata, _ := rdb.HGetAll(ctx, fmt.Sprintf("blog:%s:metadata", blogID)).Result()
	content, _ := rdb.Get(ctx, fmt.Sprintf("blog:%s:content", blogID)).Result()
	userProfile, _ := rdb.HGetAll(ctx, fmt.Sprintf("user:%s:profile", userID)).Result()

	fmt.Println("\n--- Blog Metadata ---")
	fmt.Println(metadata)
	fmt.Println("\n--- Blog Content ---")
	fmt.Println(content)
	fmt.Println("\n--- User Profile ---")
	fmt.Println(userProfile)
}

func main() {
	blogID1 := "101"
	blogID2 := "102"
	userID := "user123"

	loadBlogMetadata(blogID1, "Redis in Go", "Alice", "2025-02-20")
	loadBlogMetadata(blogID2, "Golang Concurrency", "Bob", "2025-02-18")

	addBlogContent(blogID1, "This is an introduction to Redis in Go using go-redis/v9...")
	addBlogContent(blogID2, "Goroutines and channels are key to concurrency in Go...")

	addUserProfile(userID, "Alice Doe", "alice@example.com", "Software Engineer & Golang Enthusiast")

	time.Sleep(1 * time.Second)
	fetchAllData(blogID1, userID)
}
