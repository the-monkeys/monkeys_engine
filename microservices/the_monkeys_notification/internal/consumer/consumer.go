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
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_notification/internal/database"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_notification/internal/models"
)

func ConsumeFromQueue(conn rabbitmq.Conn, conf config.RabbitMQ, log *logrus.Logger, db database.NotificationDB) {

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logrus.Infoln("Received termination signal. Closing connection and exiting gracefully.")
		conn.Channel.Close()
		os.Exit(0)
	}()

	// Consume from both queue[0] and queue[2] in separate goroutines
	go consumeQueue(conn, conf.Queues[4], log, db)

	// Keep the main function running to allow goroutines to process messages
	select {}
}

func consumeQueue(conn rabbitmq.Conn, queueName string, log *logrus.Logger, db database.NotificationDB) {
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
		logrus.Errorf("Failed to register a consumer for queue %s: %v", queueName, err)
		return
	}

	for d := range msgs {
		user := models.TheMonkeysMessage{}
		if err := json.Unmarshal(d.Body, &user); err != nil {
			logrus.Errorf("Failed to unmarshal user from RabbitMQ: %v", err)
			continue
		}

		handleUserAction(user, log, db)
	}
}

func handleUserAction(user models.TheMonkeysMessage, log *logrus.Logger, db database.NotificationDB) {
	fmt.Printf("user: %+v\n", user)

	switch user.Action {
	case constants.USER_REGISTER:
		log.Infof("Received user registration notification: %s", user.Username)
		err := db.CreateNotification(user.AccountId, constants.AccountCreated, "Welcome to The Monkeys!", user.BlogId, user.AccountId, "Browser")
		if err != nil {
			log.Errorf("Failed to create notification for user registration: %v", err)
		}
	case constants.BLOG_LIKE:
		log.Infof("Received blog like notification: %s", user.Username)
		err := db.CreateNotification(user.AccountId, constants.BlogLiked, user.Notification, user.BlogId, user.AccountId, "Browser")
		if err != nil {
			log.Errorf("Failed to create notification for blog like: %v", err)
		}
	default:
		log.Errorf("Unknown action: %s", user.Action)
	}
}
