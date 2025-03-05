package redis

import (
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_blog/pb"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/internal/blog_client"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

func PostgresConn(cfg *config.Config) (*sql.DB, error) {
	url := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.Postgresql.PrimaryDB.DBUsername,
		cfg.Postgresql.PrimaryDB.DBPassword,
		cfg.Postgresql.PrimaryDB.DBHost,
		cfg.Postgresql.PrimaryDB.DBPort,
		cfg.Postgresql.PrimaryDB.DBName,
	)
	db, err := sql.Open("postgres", url)
	if err != nil {
		logrus.Fatalf("Cannot connect to PostgreSQL, error: %+v", err)
		return nil, err
	}

	// Configure connection pooling
	db.SetMaxOpenConns(25)                 // Maximum number of open connections
	db.SetMaxIdleConns(10)                 // Maximum number of idle connections
	db.SetConnMaxLifetime(5 * time.Minute) // Connection lifetime limit

	if err = db.Ping(); err != nil {
		logrus.Errorf("Ping test failed for PostgreSQL, error: %+v", err)
		return nil, err
	}

	return db, nil

}

func NewESConn(url, username, password string) (*elasticsearch.Client, error) {
	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{url},
		Username:  username,
		Password:  password,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // Disable SSL certificate verification (for testing)
			},
			MaxIdleConnsPerHost:   10, // Set the maximum number of idle connections per host
			MaxIdleConns:          10, // Set the maximum number of idle connections
			IdleConnTimeout:       90, // Set the maximum amount of time an idle connection will remain idle before closing itself
			TLSHandshakeTimeout:   10, // Set the maximum amount of time waiting to wait for a TLS handshake
			ExpectContinueTimeout: 1,  // Set the maximum amount of time to wait for an HTTP/1.1 100-continue response
		},
	})
	if err != nil {
		return nil, err
	}

	// Perform a simple operation to check the connection
	req := esapi.PingRequest{}
	res, err := req.Do(context.Background(), client)
	if err != nil || res.IsError() {
		return nil, err
	}
	defer res.Body.Close()

	logrus.Infof("✅ Elasticsearch connection established successfully")
	return client, nil
}

// Call Multiple APis of User service or Blog service and update the cache data for one month
func LoadFeedMetaTOCache(context context.Context, bc *blog_client.BlogServiceClient, rdb *redis.Client) {
	stream, err := bc.Client.GetBlogsMetadata(context, &pb.FeedReq{
		Limit:  500,
		Offset: 0,
	})

	if err != nil {
		logrus.Errorf("cannot get the blogs by tags, error: %v", err)
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				return
			case codes.Internal:
				return
			default:
				return
			}
		}
	}

	var allBlogs []map[string]interface{}
	var totalBlogs int // Store total number of blogs

	for {
		blog, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			if status, ok := status.FromError(err); ok {
				switch status.Code() {
				case codes.NotFound:
					return
				case codes.Internal:
					return
				default:
					return
				}
			}
		}

		// Unmarshal into a map since response structure has changed
		var blogMap map[string]interface{}
		if err := json.Unmarshal(blog.Value, &blogMap); err != nil {
			logrus.Errorf("cannot unmarshal the blog, error: %v", err)
			return
		}

		// Extract "total_blogs" if present
		if total, ok := blogMap["total_blogs"].(float64); ok { // JSON numbers default to float64
			totalBlogs = int(total)
		}

		// Extract the "blogs" array safely
		blogsData, ok := blogMap["blogs"]
		if !ok {
			return
		}

		// Convert blogsData to []map[string]interface{}
		blogList, ok := blogsData.([]interface{})
		if !ok {
			return
		}

		// Convert []interface{} to []map[string]interface{}
		for _, b := range blogList {
			if blogEntry, valid := b.(map[string]interface{}); valid {
				allBlogs = append(allBlogs, blogEntry)
			}
		}
	}

	// Add additional metadata (like & bookmark count) for each blog
	for _, blog := range allBlogs {
		blogID, ok := blog["blog_id"].(string)
		if !ok {
			logrus.Errorf("BlogId is either missing or not a string: %v", blog)
			continue
		}

		likeCount, _ := bc.UserCli.GetNoOfLikeCounts(blogID)
		blog["like_count"] = likeCount

		bookmarkCount, _ := bc.UserCli.GetNoOfBookmarkCounts(blogID)
		blog["bookmark_count"] = bookmarkCount
	}

	// Final response including total blogs count
	responseBlogs := map[string]interface{}{
		"total_blogs": totalBlogs,
		"blogs":       allBlogs,
	}

	// TODO: Add the responseBlogs to Redis cache
	rdb.Set(context, "feed", responseBlogs, 30*24*time.Hour)
}
