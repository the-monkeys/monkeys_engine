package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/streadway/amqp"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_activity/pb"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/microservices/rabbitmq"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_activity/internal/database"
	"go.uber.org/zap"
)

const (
	// Queue name for activity tracking messages
	ActivityTrackingQueue = "activity_tracking_queue"
	// Routing key for activity tracking messages
	ActivityTrackingRoutingKey = "activity.track"
)

// ConsumeActivityMessages starts consuming activity tracking messages from RabbitMQ
// Following the same pattern as the users service
func ConsumeActivityMessages(conn rabbitmq.Conn, cfg *config.Config, log *zap.SugaredLogger, db database.ActivityDatabase) {
	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Debug("Received termination signal. Closing connection and exiting gracefully.")
		if err := conn.Channel.Close(); err != nil {
			log.Errorf("Error closing RabbitMQ channel: %v", err)
		}
		os.Exit(0)
	}()

	// Find the activity_tracking_queue index in the config
	var queueIndex = -1
	for i, queue := range cfg.RabbitMQ.Queues {
		if queue == ActivityTrackingQueue {
			queueIndex = i
			break
		}
	}

	if queueIndex == -1 {
		log.Errorf("activity_tracking_queue not found in RabbitMQ configuration")
		return
	}

	log.Infow("starting to consume from activity tracking queue", "queue", ActivityTrackingQueue, "index", queueIndex)

	msgs, err := conn.Channel.Consume(
		cfg.RabbitMQ.Queues[queueIndex], // queue - activity_tracking_queue
		"activity-consumer",             // consumer
		false,                           // auto-ack (manual ack for reliability)
		false,                           // exclusive
		false,                           // no-local
		false,                           // no-wait
		nil,                             // args
	)
	if err != nil {
		log.Errorf("Failed to register activity consumer: %v", err)
		return
	}

	log.Infow("activity consumer started successfully, waiting for messages")

	for msg := range msgs {

		log.Debugw("received activity tracking message", "body", string(msg.Body))

		var activityReq pb.TrackActivityRequest

		if err := json.Unmarshal(msg.Body, &activityReq); err != nil {
			log.Errorw("failed to unmarshal activity tracking message",
				"error", err,
				"message_body", string(msg.Body))
			msg.Nack(false, false) // Don't requeue malformed messages
			continue
		}

		log.Infow("processing activity tracking message",
			"user_id", activityReq.UserId,
			"action", activityReq.Action,
			"category", activityReq.Category.String())

		// Fix empty client_info.ip_address field to prevent Elasticsearch validation errors
		if activityReq.ClientInfo == nil {
			activityReq.ClientInfo = &pb.ClientInfo{
				IpAddress: "127.0.0.1", // Default to localhost for empty IP
			}
			log.Debugw("created missing client_info with default IP", "user_id", activityReq.UserId, "default_ip", "127.0.0.1")
		} else if activityReq.ClientInfo.IpAddress == "" {
			activityReq.ClientInfo.IpAddress = "127.0.0.1" // Default to localhost for empty IP
			log.Debugw("fixed empty client_info.ip_address field", "user_id", activityReq.UserId, "default_ip", "127.0.0.1")
		}

		// Store the activity in Elasticsearch
		if err := storeActivity(db, &activityReq, log); err != nil { //todo: storing activity data
			log.Errorw("failed to store activity in database",
				"error", err,
				"user_id", activityReq.UserId,
				"action", activityReq.Action)
			// Don't requeue for persistent validation errors, just discard
			msg.Nack(false, false)
			continue
		}

		log.Debugw("successfully processed and stored activity",
			"user_id", activityReq.UserId,
			"action", activityReq.Action)

		// Acknowledge the message
		msg.Ack(false)
	}
}

// storeActivity saves the activity tracking data to Elasticsearch
func storeActivity(db database.ActivityDatabase, req *pb.TrackActivityRequest, log *zap.SugaredLogger) error {
	ctx := context.Background()
	log.Debugw("storing activity in database",
		"user_id", req.UserId,
		"action", req.Action,
		"timestamp", time.Now().Format(time.RFC3339))

	// Use the existing SaveActivity method from the ActivityDatabase interface
	activityID, err := db.SaveActivity(ctx, req)

	if err != nil {
		return fmt.Errorf("failed to save activity in Elasticsearch: %w", err)
	}

	log.Infow("activity successfully stored",
		"activity_id", activityID,
		"user_id", req.UserId,
		"action", req.Action)

	return nil
}

// ActivityConsumer handles consuming activity tracking messages from RabbitMQ
type ActivityConsumer struct {
	config   *config.Config
	logger   *zap.SugaredLogger
	db       database.ActivityDatabase
	qConn    rabbitmq.Conn
	ctx      context.Context
	cancel   context.CancelFunc
	stopChan chan struct{}
}

// NewActivityConsumer creates a new ActivityConsumer instance
func NewActivityConsumer(cfg *config.Config, logger *zap.SugaredLogger, db database.ActivityDatabase) (*ActivityConsumer, error) {
	ctx, cancel := context.WithCancel(context.Background())

	consumer := &ActivityConsumer{
		config:   cfg,
		logger:   logger,
		db:       db,
		ctx:      ctx,
		cancel:   cancel,
		stopChan: make(chan struct{}),
	}

	// Initialize RabbitMQ connection
	if err := consumer.initializeRabbitMQ(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize RabbitMQ: %w", err)
	}

	return consumer, nil
}

// initializeRabbitMQ sets up the RabbitMQ connection and declares the activity tracking queue
func (ac *ActivityConsumer) initializeRabbitMQ() error {
	ac.logger.Infow("starting RabbitMQ initialization for activity consumer")

	// Get RabbitMQ configuration
	rabbitMQConfig := ac.config.RabbitMQ
	ac.logger.Debugw("RabbitMQ config",
		"host", rabbitMQConfig.Host,
		"port", rabbitMQConfig.Port,
		"username", rabbitMQConfig.Username,
		"exchange", rabbitMQConfig.Exchange)

	// Establish basic RabbitMQ connection
	connString := fmt.Sprintf("amqp://%s:%s@%s:%s/%s",
		rabbitMQConfig.Username,
		rabbitMQConfig.Password,
		rabbitMQConfig.Host,
		rabbitMQConfig.Port,
		rabbitMQConfig.VirtualHost)

	ac.logger.Debugw("connecting to RabbitMQ", "connection_string",
		fmt.Sprintf("amqp://%s:***@%s:%s/%s", rabbitMQConfig.Username, rabbitMQConfig.Host, rabbitMQConfig.Port, rabbitMQConfig.VirtualHost))

	conn, err := amqp.DialConfig(connString, amqp.Config{
		Heartbeat: 10 * time.Second,
	})
	if err != nil {
		ac.logger.Errorw("failed to connect to RabbitMQ", "error", err)
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ac.logger.Infow("successfully connected to RabbitMQ")

	ch, err := conn.Channel()
	if err != nil {
		if cerr := conn.Close(); cerr != nil {
			ac.logger.Errorw("failed to close connection after channel error", "error", cerr)
		}
		return fmt.Errorf("failed to open channel: %w", err)
	}

	ac.qConn = rabbitmq.Conn{
		Connection: conn,
		Channel:    ch,
	}

	// Declare exchange if it doesn't exist
	ac.logger.Debugw("declaring exchange", "exchange", rabbitMQConfig.Exchange)
	err = ch.ExchangeDeclare(rabbitMQConfig.Exchange, "direct", true, false, false, false, nil)
	if err != nil {
		ac.logger.Errorw("failed to declare exchange", "error", err, "exchange", rabbitMQConfig.Exchange)
		ac.qConn.Close()
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	// Declare activity tracking queue
	ac.logger.Debugw("declaring activity tracking queue", "queue", ActivityTrackingQueue)
	_, err = ch.QueueDeclare(ActivityTrackingQueue, true, false, false, false, nil)
	if err != nil {
		ac.logger.Errorw("failed to declare activity tracking queue", "error", err, "queue", ActivityTrackingQueue)
		ac.qConn.Close()
		return fmt.Errorf("failed to declare activity tracking queue: %w", err)
	}

	// Bind activity tracking queue to exchange
	ac.logger.Debugw("binding queue to exchange",
		"queue", ActivityTrackingQueue,
		"routing_key", ActivityTrackingRoutingKey,
		"exchange", rabbitMQConfig.Exchange)
	err = ch.QueueBind(ActivityTrackingQueue, ActivityTrackingRoutingKey, rabbitMQConfig.Exchange, false, nil)
	if err != nil {
		ac.logger.Errorw("failed to bind activity tracking queue", "error", err)
		ac.qConn.Close()
		return fmt.Errorf("failed to bind activity tracking queue: %w", err)
	}

	ac.logger.Infow("successfully connected to RabbitMQ for activity tracking",
		"exchange", rabbitMQConfig.Exchange,
		"queue", ActivityTrackingQueue,
		"routing_key", ActivityTrackingRoutingKey)

	return nil
} // Start begins consuming messages from the activity tracking queue
func (ac *ActivityConsumer) Start() error {
	ac.logger.Infow("starting activity tracking consumer", "queue", ActivityTrackingQueue)

	// Set up message consumption
	msgs, err := ac.qConn.Channel.Consume(
		ActivityTrackingQueue, // queue
		"activity-consumer",   // consumer tag
		false,                 // auto-ack (we want manual ack for reliability)
		false,                 // exclusive
		false,                 // no-local
		false,                 // no-wait
		nil,                   // args
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	// Start consuming messages in a goroutine
	go func() {
		defer func() {
			ac.logger.Info("activity tracking consumer stopped")
			close(ac.stopChan)
		}()

		for {
			select {
			case <-ac.ctx.Done():
				ac.logger.Info("context cancelled, stopping activity consumer")
				return
			case msg, ok := <-msgs:
				if !ok {
					ac.logger.Warn("message channel closed, stopping consumer")
					return
				}
				ac.processMessage(msg)
			}
		}
	}()

	ac.logger.Info("activity tracking consumer started successfully")
	return nil
}

// processMessage handles individual activity tracking messages
func (ac *ActivityConsumer) processMessage(msg amqp.Delivery) {
	start := time.Now()

	// Parse the activity tracking request from the message
	var activityReq pb.TrackActivityRequest
	if err := json.Unmarshal(msg.Body, &activityReq); err != nil {
		ac.logger.Errorw("failed to unmarshal activity tracking message",
			"error", err,
			"message_body", string(msg.Body))
		msg.Nack(false, false) // Don't requeue malformed messages
		return
	}

	// Log the received activity
	platform := "unknown"
	if activityReq.ClientInfo != nil {
		platform = activityReq.ClientInfo.Platform.String()
	}
	ac.logger.Debugw("processing activity tracking message",
		"user_id", activityReq.UserId,
		"action", activityReq.Action,
		"category", activityReq.Category,
		"platform", platform)

	// Store the activity in Elasticsearch
	if err := ac.storeActivity(&activityReq); err != nil {
		ac.logger.Errorw("failed to store activity in database",
			"error", err,
			"user_id", activityReq.UserId,
			"action", activityReq.Action)

		// Decide whether to requeue based on error type
		// For now, we'll requeue all failed messages with a delay
		msg.Nack(false, true) // Requeue the message
		return
	}

	// Successfully processed, acknowledge the message
	if err := msg.Ack(false); err != nil {
		ac.logger.Errorw("failed to acknowledge message", "error", err)
	}

	duration := time.Since(start)
	ac.logger.Debugw("successfully processed activity tracking message",
		"user_id", activityReq.UserId,
		"action", activityReq.Action,
		"processing_time", duration)
}

// storeActivity saves the activity tracking data to Elasticsearch
func (ac *ActivityConsumer) storeActivity(req *pb.TrackActivityRequest) error {
	// Use the existing SaveActivity method from the database
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := ac.db.SaveActivity(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to save activity: %w", err)
	}

	return nil
}

// Stop gracefully shuts down the consumer
func (ac *ActivityConsumer) Stop() error {
	ac.logger.Info("stopping activity tracking consumer")

	// Cancel the context to stop message processing
	ac.cancel()

	// Wait for consumer to stop or timeout
	select {
	case <-ac.stopChan:
		ac.logger.Info("activity consumer stopped gracefully")
	case <-time.After(30 * time.Second):
		ac.logger.Warn("timeout waiting for activity consumer to stop")
	}

	// Close RabbitMQ connection
	ac.qConn.Close()

	return nil
}

// IsHealthy checks if the consumer is healthy and ready to process messages
func (ac *ActivityConsumer) IsHealthy() bool {
	// Check if context is still active and RabbitMQ connection is alive
	return ac.ctx.Err() == nil && ac.qConn.Connection != nil && !ac.qConn.Connection.IsClosed()
}
