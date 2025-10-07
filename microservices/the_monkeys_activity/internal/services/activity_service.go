package services

import (
	"context"

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
}

// NewActivityServiceServer creates a new ActivityServiceServer
func NewActivityServiceServer(cfg *config.Config, logger *zap.SugaredLogger, db *database.ActivityDB) *ActivityServiceServer {
	return &ActivityServiceServer{
		config: cfg,
		logger: logger,
		db:     db,
	}
}

// TrackActivity records a user activity event
func (s *ActivityServiceServer) TrackActivity(ctx context.Context, req *pb.TrackActivityRequest) (*pb.TrackActivityResponse, error) {
	s.logger.Debugw("TrackActivity called",
		"user_id", req.GetUserId(),
		"account_id", req.GetAccountId(),
		"category", req.GetCategory(),
		"action", req.GetAction(),
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
