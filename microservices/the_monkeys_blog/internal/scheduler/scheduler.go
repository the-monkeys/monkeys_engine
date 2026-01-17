package scheduler

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/constants"
	"github.com/the-monkeys/the_monkeys/microservices/rabbitmq"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_blog/internal/database"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_blog/internal/models"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_blog/internal/seo"
	"go.uber.org/zap"
)

const (
	// DefaultPollingInterval is the interval between scheduler checks
	DefaultPollingInterval = 30 * time.Second
	// MaxRetries for publishing a blog
	MaxRetries = 3
	// RetryDelay between retries
	RetryDelay = 5 * time.Second
)

// Scheduler handles the automatic publishing of scheduled blogs
type Scheduler struct {
	db         database.ElasticsearchStorage
	seoManager seo.SEOManager
	qConn      rabbitmq.Conn
	config     *config.Config
	logger     *zap.SugaredLogger
	stopCh     chan struct{}
	wg         sync.WaitGroup
	mu         sync.Mutex
	running    bool
}

// NewScheduler creates a new blog scheduler
func NewScheduler(
	db database.ElasticsearchStorage,
	seoManager seo.SEOManager,
	qConn rabbitmq.Conn,
	cfg *config.Config,
	logger *zap.SugaredLogger,
) *Scheduler {
	return &Scheduler{
		db:         db,
		seoManager: seoManager,
		qConn:      qConn,
		config:     cfg,
		logger:     logger,
		stopCh:     make(chan struct{}),
	}
}

// Start begins the scheduler background worker
func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		s.logger.Warn("Scheduler is already running")
		return
	}
	s.running = true
	s.mu.Unlock()

	s.wg.Add(1)
	go s.run()

	s.logger.Info("📅 Blog Scheduler started - checking for scheduled blogs every 30 seconds")
}

// Stop gracefully stops the scheduler
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.mu.Unlock()

	close(s.stopCh)
	s.wg.Wait()
	s.logger.Info("📅 Blog Scheduler stopped")
}

// run is the main scheduler loop
func (s *Scheduler) run() {
	defer s.wg.Done()

	ticker := time.NewTicker(DefaultPollingInterval)
	defer ticker.Stop()

	// Run immediately on start
	s.processScheduledBlogs()

	for {
		select {
		case <-s.stopCh:
			s.logger.Debug("Scheduler received stop signal")
			return
		case <-ticker.C:
			s.processScheduledBlogs()
		}
	}
}

// processScheduledBlogs fetches and publishes all due scheduled blogs
func (s *Scheduler) processScheduledBlogs() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Get current time in UTC for consistent comparison
	// Note: schedule_time is stored in UTC in Elasticsearch
	currentTime := time.Now().UTC()

	s.logger.Debugf("Scheduler: checking for scheduled blogs due at %s", currentTime.Format(time.RFC3339))

	// Fetch due scheduled blogs
	dueBlogs, err := s.db.GetDueScheduledBlogs(ctx, currentTime)
	if err != nil {
		s.logger.Errorf("Scheduler: failed to fetch due scheduled blogs: %v", err)
		return
	}

	if len(dueBlogs) == 0 {
		s.logger.Debug("Scheduler: no scheduled blogs due")
		return
	}

	s.logger.Infof("Scheduler: found %d scheduled blogs ready to publish", len(dueBlogs))

	// Process each blog
	for _, blog := range dueBlogs {
		blogId, _ := blog["blog_id"].(string)
		accountId, _ := blog["owner_account_id"].(string)

		if blogId == "" {
			s.logger.Warn("Scheduler: skipping blog with empty blog_id")
			continue
		}

		// Publish with retry logic
		if err := s.publishBlogWithRetry(ctx, blogId, accountId, blog); err != nil {
			s.logger.Errorf("Scheduler: failed to publish blog %s after %d retries: %v", blogId, MaxRetries, err)
			// TODO: Consider moving to a dead-letter queue or sending alert
			continue
		}

		s.logger.Infof("Scheduler: successfully published scheduled blog %s", blogId)
	}
}

// publishBlogWithRetry attempts to publish a blog with exponential backoff retry
func (s *Scheduler) publishBlogWithRetry(ctx context.Context, blogId, accountId string, blog map[string]interface{}) error {
	var lastErr error

	for attempt := 1; attempt <= MaxRetries; attempt++ {
		err := s.publishBlog(ctx, blogId, accountId, blog)
		if err == nil {
			return nil
		}

		lastErr = err
		s.logger.Warnf("Scheduler: publish attempt %d/%d failed for blog %s: %v", attempt, MaxRetries, blogId, err)

		if attempt < MaxRetries {
			// Exponential backoff
			backoffDuration := RetryDelay * time.Duration(attempt)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoffDuration):
				// Continue to next retry
			}
		}
	}

	return lastErr
}

// publishBlog handles the actual publishing of a single blog
func (s *Scheduler) publishBlog(ctx context.Context, blogId, accountId string, blog map[string]interface{}) error {
	// Update blog status in Elasticsearch
	_, err := s.db.PublishScheduledBlog(ctx, blogId)
	if err != nil {
		return err
	}

	// Extract tags from blog
	var tags []string
	if tagsInterface, ok := blog["tags"].([]interface{}); ok {
		for _, t := range tagsInterface {
			if tag, ok := t.(string); ok {
				tags = append(tags, tag)
			}
		}
	}

	// Send RabbitMQ message for downstream services
	msg := models.InterServiceMessage{
		AccountId:  accountId,
		BlogId:     blogId,
		Action:     constants.BLOG_PUBLISH,
		BlogStatus: constants.BlogStatusPublished,
		Tags:       tags,
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		s.logger.Errorf("Scheduler: failed to marshal RabbitMQ message for blog %s: %v", blogId, err)
		// Don't fail the publish - blog is already published in ES
	} else {
		if err := s.qConn.PublishMessage(s.config.RabbitMQ.Exchange, s.config.RabbitMQ.RoutingKeys[1], msgBytes); err != nil {
			s.logger.Errorf("Scheduler: failed to publish RabbitMQ message for blog %s: %v", blogId, err)
			// Don't fail the publish - blog is already published in ES
		}
	}

	// Handle SEO (async, non-blocking)
	go func() {
		slug := ""
		if s, ok := blog["slug"].(string); ok {
			slug = s
		}
		if slug == "" {
			slug = "blog-" + blogId
		}

		if err := s.seoManager.HandleSEOForBlog(context.Background(), blogId, slug); err != nil {
			s.logger.Errorf("Scheduler: SEO handling failed for blog %s: %v", blogId, err)
		}
	}()

	return nil
}
