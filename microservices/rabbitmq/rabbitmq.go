package rabbitmq

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/streadway/amqp"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/logger"
)

var log = logger.ZapForService("rabbitmq")

// / Conn represents a RabbitMQ connection with a channel.
type Conn struct {
	Connection *amqp.Connection
	Channel    *amqp.Channel
}

// GetConn establishes a connection to RabbitMQ and returns a Conn struct.
func GetConn(conf config.RabbitMQ) (Conn, error) {
	connString := fmt.Sprintf("amqp://%s:%s@%s:%s/%s", conf.Username, conf.Password, conf.Host, conf.Port, conf.VirtualHost)

	conn, err := amqp.DialConfig(connString, amqp.Config{
		Heartbeat: 10 * time.Second, // Set the heartbeat interval to 10 seconds
	})
	if err != nil {
		return Conn{}, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		if cerr := conn.Close(); cerr != nil {
			log.Errorf("failed to close connection after channel error: %v", cerr)
		}
		return Conn{}, fmt.Errorf("failed to open a channel: %w", err)
	}

	connection := Conn{
		Connection: conn,
		Channel:    ch,
	}

	if len(conf.Queues) == 0 || len(conf.RoutingKeys) == 0 {
		log.Fatalf("Queues or RoutingKeys are not configured properly")
	}

	log.Debugf("Creating the exchange: %s", conf.Exchange)
	err = connection.Channel.ExchangeDeclare(conf.Exchange, "direct", true, false, false, false, nil)
	if err != nil {
		connection.Close()
		return Conn{}, fmt.Errorf("failed to declare exchange: %w", err)
	}

	for i, queue := range conf.Queues {
		log.Debugf("Creating a queue: %s", queue)
		_, err = connection.Channel.QueueDeclare(queue, true, false, false, false, nil)
		if err != nil {
			connection.Close()
			return Conn{}, fmt.Errorf("failed to declare queue: %w", err)
		}

		log.Debugf("Binding the queue %s with exchange %s using routing key %s", queue, conf.Exchange, conf.RoutingKeys[i])
		err = connection.Channel.QueueBind(queue, conf.RoutingKeys[i], conf.Exchange, false, nil)
		if err != nil {
			connection.Close()
			return Conn{}, fmt.Errorf("failed to bind queue: %w", err)
		}
	}

	return connection, nil
}

// Reconnect attempts to re-establish the RabbitMQ connection
func Reconnect(conf config.RabbitMQ) Conn {
	var qConn Conn
	var err error
	for {
		qConn, err = GetConn(conf)
		if err != nil {
			log.Errorf("cannot connect to RabbitMQ, retrying in 1 second: %v", err)
			time.Sleep(time.Second)
			continue
		}
		log.Debug("Reconnected to RabbitMQ")
		break
	}
	return qConn
}

// PublishMessage sends a message to the specified exchange with the given routing key.
func (c Conn) PublishMessage(exchangeName, routingKey string, message []byte) error {
	err := c.Channel.Publish(exchangeName, routingKey, false, false, amqp.Publishing{
		ContentType: "application/octet-stream",
		Body:        message,
	})
	if err != nil {
		return fmt.Errorf("error publishing message: %w", err)
	}
	log.Debug("Message published")
	return nil
}

// ReceiveData consumes messages from the specified queue.
func (c Conn) ReceiveData(queueName string) error {
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
		return fmt.Errorf("failed to register a consumer: %w", err)
	}

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			log.Debugf("Received a message: %s", d.Body)
			// Handle your message here
		}
	}()

	log.Debug("Waiting for messages. To exit press CTRL+C")
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	forever <- true
	return nil
}

// Close closes the RabbitMQ connection and channel gracefully.
func (c Conn) Close() {
	if c.Channel != nil {
		if err := c.Channel.Close(); err != nil {
			log.Errorf("failed to close channel: %v", err)
		}
	}
	if c.Connection != nil {
		if err := c.Connection.Close(); err != nil {
			log.Errorf("Error closing RabbitMQ connection: %v", err)
		} else {
			log.Debug("RabbitMQ connection closed")
		}
	}
}
