package rabbitmq

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/streadway/amqp"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/logger"
)

var log = logger.ZapForService("rabbitmq")

// Conn represents a RabbitMQ connection with a channel.
type Conn struct {
	Connection *amqp.Connection
	Channel    *amqp.Channel
	confirmCh  chan amqp.Confirmation // single shared confirm listener
	pubMu      *sync.Mutex            // serialize publish+confirm to avoid confirmation mismatch
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

	// Set up Dead Letter Exchange and Queue if configured.
	// The DLX captures messages that are rejected (nacked without requeue) by consumers,
	// giving operators visibility into processing failures.
	if conf.DLXExchange != "" {
		if err := setupDeadLetterInfrastructure(connection.Channel, conf); err != nil {
			connection.Close()
			return Conn{}, fmt.Errorf("failed to set up dead letter infrastructure: %w", err)
		}
	}

	for i, queue := range conf.Queues {
		log.Debugf("Creating a queue: %s", queue)

		// If this is the user-service queue (queue2) and DLX is configured,
		// declare it with dead-letter routing so rejected messages go to the DLQ.
		args := amqp.Table(nil)
		if conf.DLXExchange != "" && conf.DLQKey != "" && queue == conf.Queues[1] {
			args = amqp.Table{
				"x-dead-letter-exchange":    conf.DLXExchange,
				"x-dead-letter-routing-key": conf.DLQKey,
			}
			log.Debugf("Queue %s configured with DLX: exchange=%s, routing_key=%s", queue, conf.DLXExchange, conf.DLQKey)
		}

		_, err = connection.Channel.QueueDeclare(queue, true, false, false, false, args)
		if err != nil {
			connection.Close()
			return Conn{}, fmt.Errorf("failed to declare queue %s: %w", queue, err)
		}

		log.Debugf("Binding the queue %s with exchange %s using routing key %s", queue, conf.Exchange, conf.RoutingKeys[i])
		err = connection.Channel.QueueBind(queue, conf.RoutingKeys[i], conf.Exchange, false, nil)
		if err != nil {
			connection.Close()
			return Conn{}, fmt.Errorf("failed to bind queue: %w", err)
		}
	}

	// Enable publisher confirms so PublishReliable can verify broker receipt.
	// Register a single shared confirm listener — calling NotifyPublish multiple
	// times leaks listeners and causes confirmations to be delivered to the wrong one.
	if err := connection.Channel.Confirm(false); err != nil {
		log.Warnf("Failed to enable publisher confirms (non-fatal): %v", err)
	} else {
		connection.confirmCh = make(chan amqp.Confirmation, 256)
		connection.pubMu = &sync.Mutex{}
		connection.Channel.NotifyPublish(connection.confirmCh)
		log.Debug("Publisher confirms enabled with shared confirm listener")
	}

	return connection, nil
}

// setupDeadLetterInfrastructure creates the DLX exchange and DLQ queue with binding.
func setupDeadLetterInfrastructure(ch *amqp.Channel, conf config.RabbitMQ) error {
	log.Debugf("Creating dead letter exchange: %s", conf.DLXExchange)
	if err := ch.ExchangeDeclare(conf.DLXExchange, "direct", true, false, false, false, nil); err != nil {
		return fmt.Errorf("failed to declare DLX exchange %s: %w", conf.DLXExchange, err)
	}

	dlqQueue := conf.DLQQueue
	if dlqQueue == "" {
		dlqQueue = "dead_letter_queue"
	}

	log.Debugf("Creating dead letter queue: %s", dlqQueue)
	if _, err := ch.QueueDeclare(dlqQueue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("failed to declare DLQ queue %s: %w", dlqQueue, err)
	}

	dlqKey := conf.DLQKey
	if dlqKey == "" {
		dlqKey = "dlq"
	}

	log.Debugf("Binding DLQ %s to DLX %s with routing key %s", dlqQueue, conf.DLXExchange, dlqKey)
	if err := ch.QueueBind(dlqQueue, dlqKey, conf.DLXExchange, false, nil); err != nil {
		return fmt.Errorf("failed to bind DLQ: %w", err)
	}

	return nil
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
// Messages use persistent delivery mode to survive broker restarts.
func (c Conn) PublishMessage(exchangeName, routingKey string, message []byte) error {
	err := c.Channel.Publish(exchangeName, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent, // Survive broker restarts
		Body:         message,
	})
	if err != nil {
		return fmt.Errorf("error publishing message: %w", err)
	}
	log.Debug("Message published")
	return nil
}

// PublishReliable publishes a message with publisher confirms.
// It waits for the broker to acknowledge receipt, retrying up to maxRetries times
// with exponential backoff. If the broker nacks or all retries fail, returns an error.
func (c Conn) PublishReliable(exchangeName, routingKey string, message []byte, maxRetries int) error {
	if maxRetries <= 0 {
		maxRetries = 3
	}

	backoff := 500 * time.Millisecond

	for attempt := 1; attempt <= maxRetries; attempt++ {
		confirm, err := c.publishAndWaitConfirm(exchangeName, routingKey, message)
		if err != nil {
			log.Errorf("PublishReliable: attempt %d/%d failed to publish: %v", attempt, maxRetries, err)
		} else if confirm.Ack {
			log.Debugf("PublishReliable: message confirmed by broker (attempt %d)", attempt)
			return nil
		} else {
			log.Warnf("PublishReliable: broker nacked message (attempt %d/%d)", attempt, maxRetries)
		}

		if attempt < maxRetries {
			log.Debugf("PublishReliable: retrying in %v", backoff)
			time.Sleep(backoff)
			backoff *= 2
			if backoff > 10*time.Second {
				backoff = 10 * time.Second
			}
		}
	}

	return fmt.Errorf("PublishReliable: exhausted %d retries, message not confirmed", maxRetries)
}

// publishAndWaitConfirm publishes one message and blocks until the broker confirms or 5s timeout.
// Uses the shared confirmCh registered once during GetConn, serialized by pubMu
// to avoid confirmation delivery mismatch across concurrent publishers.
func (c Conn) publishAndWaitConfirm(exchangeName, routingKey string, message []byte) (amqp.Confirmation, error) {
	if c.confirmCh == nil {
		return amqp.Confirmation{}, fmt.Errorf("publisher confirms not enabled")
	}

	// Serialize publish + confirm-wait so each publish gets its own confirmation.
	c.pubMu.Lock()
	defer c.pubMu.Unlock()

	// Drain any stale confirmations from previous PublishMessage calls or
	// other non-reliable publishes that generated confirms we never consumed.
	for {
		select {
		case <-c.confirmCh:
		default:
			goto drained
		}
	}
drained:

	err := c.Channel.Publish(exchangeName, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         message,
	})
	if err != nil {
		return amqp.Confirmation{}, fmt.Errorf("error publishing message: %w", err)
	}

	select {
	case confirm := <-c.confirmCh:
		return confirm, nil
	case <-time.After(5 * time.Second):
		return amqp.Confirmation{}, fmt.Errorf("timed out waiting for broker confirmation")
	}
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

// ConnManager wraps a Conn with a mutex so that reconnection in the consumer
// goroutine is immediately visible to all other goroutines holding the *ConnManager.
// Use *ConnManager everywhere instead of passing Conn by value.
type ConnManager struct {
	mu   sync.RWMutex
	conn Conn
	conf config.RabbitMQ
}

// NewConnManager dials RabbitMQ (with infinite retry) and returns a shared manager.
func NewConnManager(conf config.RabbitMQ) *ConnManager {
	m := &ConnManager{conf: conf}
	m.conn = Reconnect(conf)
	return m
}

// reconnectLocked MUST be called with m.mu held for writing.
func (m *ConnManager) reconnectLocked() {
	m.conn.Close()
	m.conn = Reconnect(m.conf)
}

// Reconnect replaces the current connection. Safe for concurrent callers — only
// one will actually reconnect; the rest will observe the updated conn.
func (m *ConnManager) Reconnect() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reconnectLocked()
}

// Channel returns the current *amqp.Channel. Consumers call this to register
// with Consume(); they must call Reconnect() and re-register when msgs closes.
func (m *ConnManager) Channel() *amqp.Channel {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.conn.Channel
}

// PublishReliable publishes with broker confirmation. On any error it reconnects
// once and retries so a transient connection blip doesn't lose the message.
func (m *ConnManager) PublishReliable(exchange, routingKey string, message []byte, maxRetries int) error {
	m.mu.RLock()
	conn := m.conn
	m.mu.RUnlock()

	err := conn.PublishReliable(exchange, routingKey, message, maxRetries)
	if err == nil {
		return nil
	}

	log.Warnf("ConnManager.PublishReliable: failed, reconnecting and retrying once: %v", err)
	m.mu.Lock()
	m.reconnectLocked()
	conn = m.conn
	m.mu.Unlock()

	return conn.PublishReliable(exchange, routingKey, message, maxRetries)
}

// PublishMessage sends without confirmation. On error it reconnects and retries once.
func (m *ConnManager) PublishMessage(exchange, routingKey string, message []byte) error {
	m.mu.RLock()
	conn := m.conn
	m.mu.RUnlock()

	err := conn.PublishMessage(exchange, routingKey, message)
	if err == nil {
		return nil
	}

	log.Warnf("ConnManager.PublishMessage: failed, reconnecting and retrying once: %v", err)
	m.mu.Lock()
	m.reconnectLocked()
	conn = m.conn
	m.mu.Unlock()

	return conn.PublishMessage(exchange, routingKey, message)
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
