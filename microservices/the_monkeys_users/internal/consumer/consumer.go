package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/constants"
	"github.com/the-monkeys/the_monkeys/microservices/rabbitmq"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_users/internal/cache"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_users/internal/database"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_users/internal/models"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_users/internal/utils"
)

type UserDbConn struct {
	dbConn database.UserDb
	log    *logrus.Logger
	config *config.Config
}

func NewUserDb(dbConn database.UserDb, log *logrus.Logger, config *config.Config) *UserDbConn {
	return &UserDbConn{
		dbConn: dbConn,
		log:    log,
		config: config,
	}
}

func ConsumeFromQueue(conn rabbitmq.Conn, conf *config.Config, log *logrus.Logger, dbConn database.UserDb) {
	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Infoln("Received termination signal. Closing connection and exiting gracefully.")
		if err := conn.Channel.Close(); err != nil {
			log.Errorf("Error closing RabbitMQ channel: %v", err)
		}
		os.Exit(0)
	}()

	msgs, err := conn.Channel.Consume(
		conf.RabbitMQ.Queues[1], // queue
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

	userCon := NewUserDb(dbConn, log, conf)
	for d := range msgs {
		user := models.TheMonkeysMessage{}
		if err = json.Unmarshal(d.Body, &user); err != nil {
			logrus.Errorf("Failed to unmarshal user from rabbitMQ: %v", err)
			return
		}

		log.Debugf("user: %+v\n", user)

		userLog := &models.UserLogs{
			AccountId: user.AccountId,
			IpAddress: user.IpAddress,
			Client:    user.Client,
		}
		log.Debugf("userLog: %+v\n", userLog)
		userLog.IpAddress, userLog.Client = utils.IpClientConvert(userLog.IpAddress, userLog.Client)

		switch user.Action {
		case constants.BLOG_CREATE:
			log.Infof("Creating blog: %v", user)
			if err := userCon.dbConn.AddBlogWithId(user); err != nil {
				userCon.log.Errorf("Error creating blog: %v", err)
			}

			go cache.AddUserLog(userCon.dbConn, userLog, fmt.Sprintf(constants.CreateBlog, user.BlogId), constants.ServiceBlog, constants.EventCreatedBlog, userCon.log)

		case constants.BLOG_UPDATE:
			usr, err := userCon.dbConn.GetBlogByBlogId(user.BlogId)
			if err != nil {
				log.Errorf("Error getting blog: %v", err)
			}

			if usr.BlogStatus != user.BlogStatus {
				if err := userCon.dbConn.UpdateBlogStatusToDraft(user.BlogId, user.BlogStatus); err != nil {
					log.Errorf("Can't update blog status to draft: %v", err)
				}

				go cache.AddUserLog(userCon.dbConn, userLog, fmt.Sprintf(constants.MovedBlogToDraft, user.BlogId), constants.ServiceBlog, constants.EventDraftedBlog, userCon.log)
			}

		case constants.BLOG_PUBLISH:
			fmt.Printf("User published a blog: %+v", user)
			if err := userCon.dbConn.UpdateBlogStatusToPublish(user.BlogId, user.BlogStatus); err != nil {
				log.Errorf("Can't update blog status to publish: %v", err)
			}

			// TODO: Add tags like it is created by the User
			for _, tag := range user.Tags {
				if err := userCon.dbConn.InsertTopicWithCategory(context.Background(), tag, "General"); err != nil {
					log.Errorf("Can't update blog status to publish: %v", err)
				}
			}

			go cache.AddUserLog(userCon.dbConn, userLog, fmt.Sprintf(constants.PublishBlog, user.BlogId), constants.ServiceBlog, constants.EventPublishedBlog, userCon.log)

		case constants.BLOG_DELETE:
			if err := userCon.dbConn.DeleteBlogAndReferences(user.BlogId); err != nil {
				log.Errorf("Can't delete blog %s from user service: %v", user.BlogId, err)
			}

			go cache.AddUserLog(userCon.dbConn, userLog, fmt.Sprintf(constants.DeleteBlog, user.BlogId), constants.ServiceBlog, constants.EventDeleteBlog, userCon.log)

		default:
			log.Errorf("Unknown action: %s", user.Action)
		}

	}
}
