package consumer

import (
	"encoding/json"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/constants"
	"github.com/the-monkeys/the_monkeys/microservices/rabbitmq"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_notification/internal/database"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_notification/internal/models"
	"go.uber.org/zap"
)

func ConsumeFromQueue(mgr *rabbitmq.ConnManager, conf config.RabbitMQ, log *zap.SugaredLogger, db database.NotificationDB) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Debug("Notification consumer: received termination signal, exiting")
		os.Exit(0)
	}()

	go consumeQueue(mgr, conf.Queues[4], log, db)
	select {}
}

func consumeQueue(mgr *rabbitmq.ConnManager, queueName string, log *zap.SugaredLogger, db database.NotificationDB) {
	backoff := time.Second

	for {
		msgs, err := mgr.Channel().Consume(
			queueName,
			"",
			true, // auto-ack
			false,
			false,
			false,
			nil,
		)
		if err != nil {
			log.Errorf("Notification consumer: failed to register on queue '%s', reconnecting in %v: %v", queueName, backoff, err)
			time.Sleep(backoff)
			if backoff *= 2; backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			mgr.Reconnect()
			continue
		}

		backoff = time.Second
		log.Info("Notification consumer: registered on queue: ", queueName)

		for d := range msgs {
			user := models.TheMonkeysMessage{}
			if err := json.Unmarshal(d.Body, &user); err != nil {
				log.Errorf("Failed to unmarshal user from RabbitMQ: %v", err)
				continue
			}
			handleUserAction(user, log, db)
		}

		log.Warn("Notification consumer: channel closed, reconnecting...")
		mgr.Reconnect()
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
