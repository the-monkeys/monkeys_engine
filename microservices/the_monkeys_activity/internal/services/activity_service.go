package services

import (
	"context"

	"sync"
	"time"

	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_activity/pb"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_activity/internal/database"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ActivityServiceServer implements the ActivityService gRPC server
type ActivityServiceServer struct {
	pb.UnimplementedActivityServiceServer
	config *config.Config
	logger *zap.SugaredLogger
	db     *database.ActivityDB

	// Cache for periodic analytics
	trendingCache    []*pb.TrendingBlog
	activeUsersCache int64
	advancedCache    *pb.AdvancedAnalytics
	cacheMutex       sync.RWMutex
	lastRefresh      time.Time
}

// NewActivityServiceServer creates a new ActivityServiceServer
func NewActivityServiceServer(cfg *config.Config, logger *zap.SugaredLogger, db *database.ActivityDB) *ActivityServiceServer {
	s := &ActivityServiceServer{
		config: cfg,
		logger: logger,
		db:     db,
	}
	go s.startPeriodicAnalyticsRefresher()
	return s
}

func (s *ActivityServiceServer) startPeriodicAnalyticsRefresher() {
	ticker := time.NewTicker(3 * time.Hour)
	defer ticker.Stop()

	// Initial refresh
	s.refreshAnalytics()

	for range ticker.C {
		s.refreshAnalytics()
	}
}

func (s *ActivityServiceServer) refreshAnalytics() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	s.logger.Info("Refreshing periodic analytics cache...")

	// 1. Trending Blogs (last 24h)
	trending, err := s.db.GetTrendingBlogs(ctx, &pb.GetTrendingBlogsRequest{TimeRange: "24h", Limit: 10})
	if err != nil {
		s.logger.Errorw("failed to refresh trending blogs", "error", err)
	}

	// 2. Active Users (last 3h)
	activeUsers, _, err := s.db.GetActiveUsers(ctx, &pb.GetActiveUsersRequest{TimeRange: "3h"})
	if err != nil {
		s.logger.Errorw("failed to refresh active users", "error", err)
	}

	// 3. Advanced Analytics (last 7d)
	advanced, err := s.db.GetAdvancedAnalytics(ctx, &pb.GetAdvancedAnalyticsRequest{TimeRange: "7d"})
	if err != nil {
		s.logger.Errorw("failed to refresh advanced analytics", "error", err)
	}

	s.cacheMutex.Lock()
	s.trendingCache = trending
	s.activeUsersCache = activeUsers
	s.advancedCache = advanced
	s.lastRefresh = time.Now()
	s.cacheMutex.Unlock()

	s.logger.Info("Periodic analytics cache refreshed successfully")
}

// TrackActivity records a user activity event
func (s *ActivityServiceServer) TrackActivity(ctx context.Context, req *pb.TrackActivityRequest) (*pb.TrackActivityResponse, error) {
	s.logger.Debugw("TrackActivity called",
		"user_id", req.GetUserId(),
		"account_id", req.GetAccountId(),
		"category", req.GetCategory(),
		"action", req.GetAction(),
		"clientInfo", req.GetClientInfo(),
	)

	// Validate required fields
	if req.GetUserId() == "" {
		return &pb.TrackActivityResponse{
			StatusCode: 400,
			Message:    "user_id is required",
			Error: &pb.Error{
				Status:  400,
				Error:   "validation_error",
				Message: "user_id is required",
			},
		}, nil
	}

	if req.GetAccountId() == "" {
		return &pb.TrackActivityResponse{
			StatusCode: 400,
			Message:    "account_id is required",
			Error: &pb.Error{
				Status:  400,
				Error:   "validation_error",
				Message: "account_id is required",
			},
		}, nil
	}

	if req.GetCategory() == pb.ActivityCategory_CATEGORY_UNSPECIFIED {
		return &pb.TrackActivityResponse{
			StatusCode: 400,
			Message:    "category is required",
			Error: &pb.Error{
				Status:  400,
				Error:   "validation_error",
				Message: "category is required",
			},
		}, nil
	}

	if req.GetAction() == "" {
		return &pb.TrackActivityResponse{
			StatusCode: 400,
			Message:    "action is required",
			Error: &pb.Error{
				Status:  400,
				Error:   "validation_error",
				Message: "action is required",
			},
		}, nil
	}

	// Save to Elasticsearch
	activityID, err := s.db.SaveActivity(ctx, req)
	if err != nil {
		s.logger.Errorw("failed to save activity", "error", err)
		return &pb.TrackActivityResponse{
			StatusCode: 500,
			Message:    "internal server error",
			Error: &pb.Error{
				Status:  500,
				Error:   "database_error",
				Message: "failed to save activity",
			},
		}, nil
	}

	s.logger.Infow("activity tracked successfully",
		"activity_id", activityID,
		"user_id", req.GetUserId(),
		"category", req.GetCategory(),
		"action", req.GetAction(),
	)

	return &pb.TrackActivityResponse{
		StatusCode: 200,
		Message:    "activity tracked successfully",
		ActivityId: activityID,
	}, nil
}

// GetUserActivities retrieves user activity history
func (s *ActivityServiceServer) GetUserActivities(ctx context.Context, req *pb.GetUserActivitiesRequest) (*pb.GetUserActivitiesResponse, error) {
	s.logger.Debugw("GetUserActivities called",
		"user_id", req.GetUserId(),
		"account_id", req.GetAccountId(),
		"category", req.GetCategory(),
		"limit", req.GetLimit(),
		"offset", req.GetOffset(),
	)

	// Validate required fields
	if req.GetUserId() == "" {
		return &pb.GetUserActivitiesResponse{
			StatusCode: 400,
			Error: &pb.Error{
				Status:  400,
				Error:   "validation_error",
				Message: "user_id is required",
			},
		}, nil
	}

	// Fetch from Elasticsearch
	activities, totalCount, err := s.db.GetUserActivities(ctx, req)
	if err != nil {
		s.logger.Errorw("failed to get user activities", "error", err)
		return &pb.GetUserActivitiesResponse{
			StatusCode: 500,
			Error: &pb.Error{
				Status:  500,
				Error:   "database_error",
				Message: "failed to retrieve activities",
			},
		}, nil
	}

	return &pb.GetUserActivitiesResponse{
		StatusCode: 200,
		Activities: activities,
		TotalCount: totalCount,
	}, nil
}

// HealthCheck implements health check endpoint
func (s *ActivityServiceServer) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	return &pb.HealthCheckResponse{
		StatusCode:  200,
		Status:      "healthy",
		ServiceName: "activity-service",
		Version:     "1.0.0",
		Timestamp:   timestamppb.Now(),
	}, nil
}
