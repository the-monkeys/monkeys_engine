package consumer

import (
	"encoding/json"
	"os"
	"os/signal"
	"syscall"

	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/constants"
	"github.com/the-monkeys/the_monkeys/logger"
	"github.com/the-monkeys/the_monkeys/microservices/rabbitmq"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_notification/internal/database"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_notification/internal/models"
	"go.uber.org/zap"
)

func ConsumeFromQueue(conn rabbitmq.Conn, conf config.RabbitMQ, log *zap.SugaredLogger, db database.NotificationDB) {

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.ZapSugar().Debug("Received termination signal. Closing connection and exiting gracefully.")
		if err := conn.Channel.Close(); err != nil {
			logger.ZapSugar().Errorf("Failed to close RabbitMQ channel: %v", err)
		}
		os.Exit(0)
	}()

	// Consume from both queue[0] and queue[2] in separate goroutines
	go consumeQueue(conn, conf.Queues[4], log, db)

	// Keep the main function running to allow goroutines to process messages
	select {}
}

func consumeQueue(conn rabbitmq.Conn, queueName string, log *zap.SugaredLogger, db database.NotificationDB) {
	msgs, err := conn.Channel.Consume(
		queueName, // queue
		"",        // consumer
		true,      // auto-ack
		false,     // exclusive
		false,     // no-local
		false,     // no-wait
		nil,       // args
	)
	if err != nil {
		logger.ZapSugar().Errorf("Failed to register a consumer for queue %s: %v", queueName, err)
		return
	}

	for d := range msgs {
		user := models.TheMonkeysMessage{}
		if err := json.Unmarshal(d.Body, &user); err != nil {
			logger.ZapSugar().Errorf("Failed to unmarshal user from RabbitMQ: %v", err)
			continue
		}

		handleUserAction(user, log, db)
	}
}

func handleUserAction(user models.TheMonkeysMessage, log *zap.SugaredLogger, db database.NotificationDB) {
	switch user.Action {
	case constants.USER_REGISTER:
		log.Debugf("Received user registration notification: %s", user.Username)
		err := db.CreateNotification(user.AccountId, constants.AccountCreated, "Welcome to The Monkeys!", user.BlogId, user.AccountId, "Browser")
		if err != nil {
			log.Errorf("Failed to create notification for user registration: %v", err)
		}

	case constants.BLOG_LIKE:
		log.Debugf("Received blog like notification: %s", user.Username)
		err := db.CreateNotification(user.AccountId, constants.BlogLiked, user.Notification, user.BlogId, user.AccountId, "Browser")
		if err != nil {
			log.Errorf("Failed to create notification for blog like: %v", err)
		}

	case constants.USER_FOLLOWED:
		log.Debugf("Received user follow notification: %s", user.Username)

		dbUser, err := db.CheckIfUsernameExist(user.NewUsername)
		if err != nil {
			log.Errorf("Failed to check if username exists: %v", err)
			return
		}

		err = db.CreateNotification(dbUser.AccountId, constants.NewFollower, user.Notification, user.BlogId, user.AccountId, "Browser")
		if err != nil {
			log.Errorf("Failed to create notification for blog like: %v", err)
		}

	default:
		log.Errorf("Unknown action: %s", user.Action)
	}
}
