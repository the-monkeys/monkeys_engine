package consumer

import (
	"encoding/json"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/constants"
	"github.com/the-monkeys/the_monkeys/microservices/rabbitmq"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_cache/internal/models"
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

	// userCon := NewUserDb(dbConn, log, conf)
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

		case constants.BLOG_DELETE:

		case constants.PROFILE_UPDATE:

		default:
			log.Errorf("Unknown action: %s", user.Action)
		}

	}
}
