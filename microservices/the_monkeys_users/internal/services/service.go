package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_user/pb"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/constants"
	"github.com/the-monkeys/the_monkeys/microservices/rabbitmq"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_users/internal/cache"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_users/internal/database"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_users/internal/models"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_users/internal/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	timestamp "google.golang.org/protobuf/types/known/timestamppb"
)

type UserSvc struct {
	dbConn database.UserDb
	log    *logrus.Logger
	config *config.Config
	qConn  rabbitmq.Conn
	pb.UnimplementedUserServiceServer
}

func NewUserSvc(dbConn database.UserDb, log *logrus.Logger, config *config.Config, qConn rabbitmq.Conn) *UserSvc {
	return &UserSvc{
		dbConn: dbConn,
		log:    log,
		config: config,
		qConn:  qConn,
	}
}

func (us *UserSvc) GetUserProfile(ctx context.Context, req *pb.UserProfileReq) (*pb.UserProfileRes, error) {
	us.log.Infof("profile info has been requested for user: %s.", req.Username)
	if !req.IsPrivate {
		userProfile, err := us.dbConn.GetUserProfile(req.Username)
		if err != nil {
			us.log.Errorf("error while fetching the public profile for user %s, err: %v", req.Username, err)
			if err == sql.ErrNoRows {
				return nil, status.Errorf(codes.NotFound, fmt.Sprintf("user %s doesn't exist", req.Username))
			}
			return nil, status.Errorf(codes.Internal, "cannot get the user profile")
		}
		return &pb.UserProfileRes{
			Username:  userProfile.UserName,
			FirstName: userProfile.FirstName,
			LastName:  userProfile.LastName,
			Bio:       userProfile.Bio.String,
			AvatarUrl: userProfile.AvatarUrl.String,
			CreatedAt: timestamp.New(userProfile.CreatedAt.Time),
			Address:   userProfile.Address.String,
			Linkedin:  userProfile.LinkedIn.String,
			Instagram: userProfile.Instagram.String,
			Twitter:   userProfile.Twitter.String,
			Github:    userProfile.Github.String,
		}, nil

	}

	_, err := us.dbConn.CheckIfUsernameExist(req.Username)
	if err != nil {
		us.log.Errorf("error while fetching the private profile for user %s, err: %v", req.Username, err)
		if err == sql.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, fmt.Sprintf("user %s doesn't exist", req.Username))
		}
		return nil, status.Errorf(codes.Internal, "cannot get the user profile")
	}

	userDetails, err := us.dbConn.GetMyProfile(req.Username)
	if err != nil {
		us.log.Errorf("error while fetching the profile for user %s, err: %v", req.Username, err)
		if err == sql.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, fmt.Sprintf("profile for user: %s doesn't exist", req.Username))
		}
		return nil, status.Errorf(codes.Internal, "cannot get the user profile")
	}

	return &pb.UserProfileRes{
		AccountId:     userDetails.AccountId,
		Username:      userDetails.Username,
		FirstName:     userDetails.FirstName,
		LastName:      userDetails.LastName,
		DateOfBirth:   userDetails.DateOfBirth.Time.String(),
		Bio:           userDetails.Bio.String,
		AvatarUrl:     userDetails.AvatarUrl.String,
		CreatedAt:     timestamp.New(userDetails.CreatedAt.Time),
		UpdatedAt:     timestamp.New(userDetails.UpdatedAt.Time),
		Address:       userDetails.Address.String,
		ContactNumber: userDetails.ContactNumber.String,
		UserStatus:    userDetails.UserStatus,
		Linkedin:      userDetails.LinkedIn.String,
		Instagram:     userDetails.Instagram.String,
		Twitter:       userDetails.Twitter.String,
		Github:        userDetails.Github.String,
	}, err
}

func (us *UserSvc) GetUserActivities(ctx context.Context, req *pb.UserActivityReq) (*pb.UserActivityResp, error) {
	logrus.Infof("Retrieving activities for: %v", req.UserName)
	// Check if username exits or not
	user, err := us.dbConn.CheckIfUsernameExist(req.UserName)
	if err != nil {
		us.log.Errorf("error while checking if the username exists for user %s, err: %v", req.UserName, err)
		if err == sql.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, fmt.Sprintf("user %s doesn't exist", req.UserName))
		}
		return nil, status.Errorf(codes.Internal, "cannot get the user profile")
	}

	return us.dbConn.GetUserActivities(user.Id)
}

func (us *UserSvc) UpdateUserProfile(ctx context.Context, req *pb.UpdateUserProfileReq) (*pb.UpdateUserProfileRes, error) {
	us.log.Infof("user %s is updating the profile.", req.Username)
	us.log.Infof("req: %+v", req)

	// Check if the user exists
	_, err := us.dbConn.CheckIfUsernameExist(req.Username)
	if err != nil {
		us.log.Errorf("error while checking if the username exists for user %s, err: %v", req.Username, err)
		if err == sql.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, fmt.Sprintf("user %s doesn't exist", req.Username))
		}
		return nil, status.Errorf(codes.Internal, "cannot get the user profile")
	}

	// Check if the method isPartial true
	var dbUserInfo = &models.UserProfileRes{}
	if req.Partial {
		// If isPartial is true fetch the remaining data from the db
		dbUserInfo, err = us.dbConn.GetMyProfile(req.Username)
		if err != nil {
			us.log.Errorf("error while fetching the profile for user %s, err: %v", req.Username, err)
			if err == sql.ErrNoRows {
				return nil, status.Errorf(codes.NotFound, fmt.Sprintf("user %s doesn't exist", req.Username))
			}
			return nil, status.Errorf(codes.Internal, "cannot get the user profile")
		}
		// Map the user
		dbUserInfo = utils.MapUserUpdateDataPatch(req, dbUserInfo)
	} else {
		dbUserInfo = utils.MapUserUpdateDataPut(req, dbUserInfo)
	}

	// Update the user
	err = us.dbConn.UpdateUserProfile(req.Username, dbUserInfo)
	if err != nil {
		us.log.Errorf("error while updating the profile for user %s, err: %v", req.Username, err)
		return nil, status.Errorf(codes.Internal, "cannot update the user profile")
	}

	userLog := &models.UserLogs{
		AccountId: dbUserInfo.AccountId,
	}

	userLog.IpAddress, userLog.Client = utils.IpClientConvert(req.Ip, req.Client)

	go cache.AddUserLog(us.dbConn, userLog, constants.UpdateProfile, constants.ServiceUser, constants.EventForgotPassword, us.log)

	return &pb.UpdateUserProfileRes{
		Username: dbUserInfo.Username,
	}, err
}

func (us *UserSvc) DeleteUserProfile(ctx context.Context, req *pb.DeleteUserProfileReq) (*pb.DeleteUserProfileRes, error) {
	us.log.Infof("user %s has requested to delete the  profile.", req.Username)

	// Check if username exits or not
	user, err := us.dbConn.CheckIfUsernameExist(req.Username)
	if err != nil {
		us.log.Errorf("error while checking if the username exists for user %s, err: %v", req.Username, err)
		if err == sql.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, fmt.Sprintf("user %s doesn't exist", req.Username))
		}
		return nil, status.Errorf(codes.Internal, "cannot get the user profile")
	}

	// Run delete user query
	err = us.dbConn.DeleteUserProfile(req.Username)
	if err != nil {
		us.log.Errorf("could not delete the user profile: %v", err)
		return nil, status.Errorf(codes.Internal, "cannot delete the user")
	}

	userLog := &models.UserLogs{
		AccountId: user.AccountId,
	}
	userLog.IpAddress, userLog.Client = utils.IpClientConvert(req.Ip, req.Client)

	go cache.AddUserLog(us.dbConn, userLog, constants.UpdateProfile, constants.ServiceUser, constants.EventForgotPassword, us.log)

	// Return the response
	return &pb.DeleteUserProfileRes{
		Success: "user has been deleted successfully",
		Status:  "200",
	}, nil

}

func (us *UserSvc) GetAllTopics(context.Context, *pb.GetTopicsRequests) (*pb.GetTopicsResponse, error) {
	us.log.Info("getting all the topics")

	res, err := us.dbConn.GetAllTopicsFromDb()
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			us.log.Errorf("cannot find the topics in the database: %v", err)
		}
		us.log.Errorf("error while querying the topics: %v", err)
		return nil, errors.New("error while querying the topics")
	}

	return res, err
}

func (us *UserSvc) GetAllCategories(ctx context.Context, req *pb.GetAllCategoriesReq) (*pb.GetAllCategoriesRes, error) {
	us.log.Info("getting all the Description and Categories")

	res, err := us.dbConn.GetAllCategories()
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			us.log.Errorf("no Categories and Description found in the database: %v", err)
			return nil, errors.New("no Categories found")
		}
		us.log.Errorf("error while querying the Categories: %v", err)
		return nil, errors.New("error while querying the categories")
	}

	return res, nil
}
