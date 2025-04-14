package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	blogPkg "github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_blog/pb"
	userPkg "github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_user/pb"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/constants"
	"github.com/the-monkeys/the_monkeys/microservices/rabbitmq"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_cache/internal/db"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_cache/internal/models"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_cache/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type UserDbConn struct {
	log        *logrus.Logger
	config     *config.Config
	blogClient blogPkg.BlogServiceClient
	userClient userPkg.UserServiceClient
}

func NewUserDb(log *logrus.Logger, config *config.Config) *UserDbConn {
	cc, err := grpc.NewClient(config.Microservices.TheMonkeysUser, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logrus.Errorf("cannot dial to grpc user server: %v", err)
	}
	logrus.Infof("✅ the monkeys cache server is dialing to user rpc server at: %v", config.Microservices.TheMonkeysUser)

	blogCon, err := grpc.NewClient(config.Microservices.TheMonkeysBlog, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logrus.Errorf("cannot dial to blog server: %v", err)
	}

	logrus.Infof("✅ the monkeys cache server is dialing to the blog rpc server at: %v", config.Microservices.TheMonkeysBlog)

	return &UserDbConn{
		log:        log,
		config:     config,
		blogClient: blogPkg.NewBlogServiceClient(blogCon),
		userClient: userPkg.NewUserServiceClient(cc),
	}
}

func ConsumeFromQueue(conn rabbitmq.Conn, conf *config.Config, log *logrus.Logger) {
	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Infoln("Received termination signal. Closing connection and exiting gracefully.")
		conn.Channel.Close()
		os.Exit(0)
	}()

	msgs, err := conn.Channel.Consume(
		conf.RabbitMQ.Queues[5], // queue
		"",                      // consumer
		true,                    // auto-ack
		false,                   // exclusive
		false,                   // no-local
		false,                   // no-wait
		nil,                     // args
	)
	if err != nil {
		logrus.Errorf("Failed to register a consumer: %v", err)
		return
	}

	cacheServer := service.NewCacheServer(log)
	context := context.Background()

	userCon := NewUserDb(log, conf)
	redis, err := db.RedisConn(conf, log)
	if err != nil {
		log.Errorf("Failed to connect to redis: %v", err)
		return
	}

	for d := range msgs {
		user := models.TheMonkeysMessage{}
		if err = json.Unmarshal(d.Body, &user); err != nil {
			logrus.Errorf("Failed to unmarshal user from rabbitMQ: %v", err)
			return
		}

		switch user.Action {
		case constants.BLOG_CREATE:

		case constants.BLOG_UPDATE:

		case constants.BLOG_PUBLISH:
			userCon.log.Infof("User published a blog: %+v", user)
			// TODO: Update Users profile published and draft blogs

			// Update feed
			feed := userCon.Feed(500, 0)

			// Redis cache
			feedJSON, err := json.Marshal(feed)
			if err != nil {
				userCon.log.Errorf("Failed to marshal feed: %v", err)
				return
			}

			status := redis.Set(context, fmt.Sprintf(constants.Feed, 500, 0), feedJSON, time.Hour*24*30).Err()
			if status != nil {
				userCon.log.Errorf("Failed to set feed in cache: %v", status)
			} else {
				fmt.Println("Feed successfully set in cache")
			}

			// Inbuilt cache
			cacheServer.Set(context, fmt.Sprintf(constants.Feed, 500, 0), feedJSON, time.Hour*24*30)

		case constants.BLOG_DELETE:

		case constants.PROFILE_UPDATE:

		default:
			log.Errorf("Unknown action: %s", user.Action)
		}

	}
}

func (u *UserDbConn) GetUserPublishedBlogs(username string, limit, offset int32) (interface{}, error) {
	// Get the account_id from the username
	userInfo, err := u.userClient.GetUserDetails(context.Background(), &userPkg.UserDetailReq{
		AccountId: username,
	})
	if err != nil {
		return nil, err
	}

	stream, err := u.blogClient.GetBlogs(context.Background(), &blogPkg.GetBlogsReq{
		AccountId: userInfo.AccountId,
		IsDraft:   false,
		Limit:     limit,
		Offset:    offset,
	})

	if err != nil {
		return nil, err
	}

	var allBlogs []map[string]interface{}
	for {
		blog, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		var blogMaps []map[string]interface{}
		if err := json.Unmarshal(blog.Value, &blogMaps); err != nil {
			return nil, err
		}
		allBlogs = append(allBlogs, blogMaps...)
	}

	for _, blog := range allBlogs {
		blogID, ok := blog["blog_id"].(string)
		if !ok {
			logrus.Errorf("BlogId is either missing or not a string: %v", blog)
			continue
		}

		likeCount, _ := u.GetNoOfLikeCounts(blogID)
		blog["like_count"] = likeCount

		bookmarkCount, _ := u.GetNoOfBookmarkCounts(blogID)
		blog["bookmark_count"] = bookmarkCount

	}

	responseBlogs := map[string]interface{}{
		"blogs": allBlogs,
	}

	return responseBlogs, nil
}

func (u *UserDbConn) Feed(limit, offset int32) interface{} {
	stream, err := u.blogClient.GetBlogsMetadata(context.Background(), &blogPkg.FeedReq{
		Limit:  limit,
		Offset: offset,
	})

	if err != nil {
		logrus.Errorf("cannot get the blogs by tags, error: %v", err)
		return nil
	}

	var allBlogs []map[string]interface{}
	var totalBlogs int // Store total number of blogs

	for {
		blog, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil
		}

		// Unmarshal into a map since response structure has changed
		var blogMap map[string]interface{}
		if err := json.Unmarshal(blog.Value, &blogMap); err != nil {
			logrus.Errorf("cannot unmarshal the blog, error: %v", err)
			return nil
		}

		// Extract "total_blogs" if present
		if total, ok := blogMap["total_blogs"].(float64); ok { // JSON numbers default to float64
			totalBlogs = int(total)
		}

		// Extract the "blogs" array safely
		blogsData, ok := blogMap["blogs"]
		if !ok {
			return nil
		}

		// Convert blogsData to []map[string]interface{}
		blogList, ok := blogsData.([]interface{})
		if !ok {
			return nil
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

		likeCount, _ := u.GetNoOfLikeCounts(blogID)
		blog["like_count"] = likeCount

		bookmarkCount, _ := u.GetNoOfBookmarkCounts(blogID)
		blog["bookmark_count"] = bookmarkCount
	}

	// Final response including total blogs count
	responseBlogs := map[string]interface{}{
		"total_blogs": totalBlogs,
		"blogs":       allBlogs,
	}

	return responseBlogs
}
