package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/constants"
	"github.com/the-monkeys/the_monkeys/microservices/rabbitmq"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_cache/internal/models"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_cache/internal/service"
)

type UserDbConn struct {
	log    *logrus.Logger
	config *config.Config
}

func NewUserDb(log *logrus.Logger, config *config.Config) *UserDbConn {
	return &UserDbConn{
		log:    log,
		config: config,
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
	for d := range msgs {
		user := models.TheMonkeysMessage{}
		if err = json.Unmarshal(d.Body, &user); err != nil {
			logrus.Errorf("Failed to unmarshal user from rabbitMQ: %v", err)
			return
		}

		log.Debugf("user: %+v\n", user)

		switch user.Action {
		case constants.BLOG_CREATE:

		case constants.BLOG_UPDATE:

		case constants.BLOG_PUBLISH:
			// Set User published blogs in cache
			userPublished, err := userCon.GetUserPublishedBlogs(user.Username)
			if err != nil {
				log.Errorf("Failed to get user published blogs: %v", err)
				return
			}
			cacheServer.Set(context, fmt.Sprintf(constants.UserPublished, user.Username), userPublished, time.Duration(time.Hour*24))

			// Update feed
			feed := userCon.Feed()
			cacheServer.Set(context, constants.UserPublished, feed, time.Duration(time.Hour*24))

		case constants.BLOG_DELETE:

		case constants.PROFILE_UPDATE:

		default:
			log.Errorf("Unknown action: %s", user.Action)
		}

	}
}

func (u *UserDbConn) GetUserPublishedBlogs(username string) (interface{}, error) {
	return nil, nil
}

func (u *UserDbConn) Feed() interface{} {
	return nil
}
