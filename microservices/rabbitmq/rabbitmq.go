package rabbitmq

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
	"github.com/the-monkeys/the_monkeys/config"
)

// Conn represents a RabbitMQ connection with a channel.
type Conn struct {
	Channel *amqp.Channel
}

// GetConn establishes a connection to RabbitMQ and returns a Conn struct.
func GetConn(conf config.RabbitMQ) (Conn, error) {
	connString := fmt.Sprintf("%s://%s:%s@%s:%s", conf.Protocol, conf.Username, conf.Password, conf.Host, conf.Port)

	conn, err := amqp.Dial(connString)
	if err != nil {
		return Conn{}, err
	}

	ch, err := conn.Channel()
	if err != nil {
		return Conn{}, err
	}

	connection := Conn{
		Channel: ch,
	}

	if len(conf.Queues) == 0 || len(conf.RoutingKeys) == 0 {
		logrus.Fatalf("Queues or RoutingKeys are not configured properly")
	}

	logrus.Infof("Creating the exchange: %s", conf.Exchange)
	err = connection.Channel.ExchangeDeclare(conf.Exchange, "direct", true, false, false, false, nil)
	if err != nil {
		amqpErr, ok := err.(*amqp.Error)
		if ok && amqpErr.Code == 406 {
			logrus.Warnf("Exchange %s already exists with a different type. Please use a different exchange name or delete the existing exchange.", conf.Exchange)
		} else {
			return Conn{}, err
		}
	}

	for i, queue := range conf.Queues {
		logrus.Infof("Creating a queue: %s", queue)
		_, err = connection.Channel.QueueDeclare(queue, true, false, false, false, nil)
		if err != nil {
			return Conn{}, err
		}

		logrus.Infof("Binding the queue %s with exchange %s using routing key %s", queue, conf.Exchange, conf.RoutingKeys[i])
		err = connection.Channel.QueueBind(queue, conf.RoutingKeys[i], conf.Exchange, false, nil)
		if err != nil {
			return Conn{}, err
		}
	}

	return connection, nil
}

// PublishDefaultProfilePhoto sends a message to the specified exchange with the given routing key.
func (c Conn) PublishDefaultProfilePhoto(exchangeName string, routingKey string, message []byte) {
	err := c.Channel.Publish(exchangeName, routingKey, false, false, amqp.Publishing{
		ContentType: "application/octet-stream",
		Body:        message,
	})
	if err != nil {
		logrus.Errorf("Error publishing message: %v", err)
		return
	}
	logrus.Infoln("Message published")
}

// ReceiveData consumes messages from the specified queue.
func (c Conn) ReceiveData(queueName string) {
	msgs, err := c.Channel.Consume(
		queueName, // queue
		"",        // consumer
		true,      // auto-ack
		false,     // exclusive
		false,     // no-local
		false,     // no-wait
		nil,       // args
	)
	if err != nil {
		logrus.Errorf("Failed to register a consumer: %v", err)
		return
	}

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			logrus.Infof("Received a message: %s", d.Body)
			// Handle your message here
		}
	}()

	logrus.Infoln("Waiting for messages. To exit press CTRL+C")
	<-forever
}
