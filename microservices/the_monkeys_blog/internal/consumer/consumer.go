package consumer

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"syscall"

	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/constants"
	"github.com/the-monkeys/the_monkeys/microservices/rabbitmq"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_blog/internal/database"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_blog/internal/models"
	"go.uber.org/zap"
)

func ConsumeFromQueue(conn rabbitmq.Conn, conf config.RabbitMQ, log *zap.SugaredLogger, db database.ElasticsearchStorage) {

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Debug("Graceful shutdown initiated - closing RabbitMQ connections")
		if err := conn.Channel.Close(); err != nil {
			log.Errorf("RabbitMQ channel closure failed: %v", err)
		}
		os.Exit(0)
	}()

	// Consume from both queue[0] and queue[2] in separate goroutines
	go consumeQueue(conn, conf.Queues[3], log, db)

	// Keep the main function running to allow goroutines to process messages
	select {}
}

func consumeQueue(conn rabbitmq.Conn, queueName string, log *zap.SugaredLogger, db database.ElasticsearchStorage) {
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
		log.Errorf("Queue consumer registration failed for '%s': %v", queueName, err)
		return
	}

	for d := range msgs {
		user := models.InterServiceMessage{}
		if err := json.Unmarshal(d.Body, &user); err != nil {
			log.Errorf("Message deserialization failed: %v", err)
			continue
		}

		handleUserAction(user, log, db)
	}
}

func handleUserAction(user models.InterServiceMessage, log *zap.SugaredLogger, db database.ElasticsearchStorage) {
	switch user.Action {
	case constants.USER_ACCOUNT_DELETE:
		log.Debug("Processing user account deletion request")
		resp, err := db.DeleteBlogsByOwnerAccountID(context.Background(), user.AccountId)
		if err != nil {
			log.Errorf("Blog deletion operation failed: %v", err)
			return
		}
		// Check if response is nil (no blogs found to delete)
		if resp == nil {
			log.Info("User account deletion completed - no associated blogs found")
		} else {
			log.Infof("User account deletion completed successfully - blogs removed (status: %d)", resp.StatusCode)
		}

	default:
		log.Warnf("Received unsupported action type: %s", user.Action)
	}
}
