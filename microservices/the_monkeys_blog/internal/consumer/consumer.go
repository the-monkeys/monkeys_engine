package consumer

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/constants"
	"github.com/the-monkeys/the_monkeys/microservices/rabbitmq"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_blog/internal/database"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_blog/internal/models"
)

func ConsumeFromQueue(conn rabbitmq.Conn, conf config.RabbitMQ, log *logrus.Logger, db database.ElasticsearchStorage) {

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
	go consumeQueue(conn, conf.Queues[3], log, db)

	// Keep the main function running to allow goroutines to process messages
	select {}
}

func consumeQueue(conn rabbitmq.Conn, queueName string, log *logrus.Logger, db database.ElasticsearchStorage) {
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
		user := models.InterServiceMessage{}
		if err := json.Unmarshal(d.Body, &user); err != nil {
			logrus.Errorf("Failed to unmarshal user from RabbitMQ: %v", err)
			continue
		}

		handleUserAction(user, log, db)
	}
}

func handleUserAction(user models.InterServiceMessage, log *logrus.Logger, db database.ElasticsearchStorage) {
	switch user.Action {
	case constants.USER_ACCOUNT_DELETE:
		log.Infof("Deleting all blogs for user: %s", user.AccountId)
		resp, err := db.DeleteBlogsByOwnerAccountID(context.Background(), user.AccountId)
		if err != nil {
			log.Errorf("Failed to delete blogs for user: %s, error: %v", user.AccountId, err)
			return
		}
		log.Infof("Deleted blogs for user: %s, response: %v", user.AccountId, resp.StatusCode)

	default:
		log.Errorf("Unknown action by: %s", user.AccountId)
	}
}
