package consumer

import (
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
		logrus.Infoln("Received termination signal. Closing connection and exiting gracefully.")
		conn.Channel.Close()
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

		logrus.Infof("user: %+v\n", user)

		userLog := &models.UserLogs{
			AccountId: user.UserAccountId,
			IpAddress: user.IpAddress,
			Client:    user.Client,
		}
		fmt.Printf("userLog: %+v\n", userLog)
		userLog.IpAddress, userLog.Client = utils.IpClientConvert(userLog.IpAddress, userLog.Client)

		switch user.Action {
		case constants.BLOG_CREATE:
			log.Infof("Creating blog: %v", user)
			if err := userCon.dbConn.AddBlogWithId(user); err != nil {
				userCon.log.Errorf("Error creating blog: %v", err)
			}

			go cache.AddUserLog(userCon.dbConn, userLog, fmt.Sprintf(constants.CreateBlog, user.BlogId), constants.ServiceBlog, constants.EventCreatedBlog, userCon.log)

		case constants.BLOG_PUBLISH:
			if err := userCon.dbConn.UpdateBlogStatusToPublish(user.BlogId, user.BlogStatus); err != nil {
				logrus.Errorf("Can't update blog status to publish: %v", err)
			}

			go cache.AddUserLog(userCon.dbConn, userLog, fmt.Sprintf(constants.PublishBlog, user.BlogId), constants.ServiceBlog, constants.EventPublishedBlog, userCon.log)

		case constants.BLOG_DELETE:
			if err := userCon.dbConn.DeleteBlogAndReferences(user.BlogId); err != nil {
				logrus.Errorf("Can't delete blog %s from user service: %v", user.BlogId, err)
			}

			go cache.AddUserLog(userCon.dbConn, userLog, fmt.Sprintf(constants.DeleteBlog, user.BlogId), constants.ServiceBlog, constants.EventDeleteBlog, userCon.log)
		case constants.BLOG_UPDATE:
			// TODO: Add blog id and user id
		default:
			logrus.Errorf("Unknown action: %s", user.Action)
		}

	}
}
