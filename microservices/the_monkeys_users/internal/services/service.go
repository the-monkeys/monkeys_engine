package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

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
				return nil, status.Errorf(codes.NotFound, "user %s doesn't exist", req.Username)
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
			Topics:    userProfile.Interests,
		}, nil

	}

	_, err := us.dbConn.CheckIfUsernameExist(req.Username)
	if err != nil {
		us.log.Errorf("error while fetching the private profile for user %s, err: %v", req.Username, err)
		if err == sql.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, "user %s doesn't exist", req.Username)
		}
		return nil, status.Errorf(codes.Internal, "cannot get the user profile")
	}

	userDetails, err := us.dbConn.GetMyProfile(req.Username)
	if err != nil {
		us.log.Errorf("error while fetching the profile for user %s, err: %v", req.Username, err)
		if err == sql.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, "profile for user: %s doesn't exist", req.Username)
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
		Topics:        userDetails.Interests,
	}, err
}

func (us *UserSvc) GetUserActivities(ctx context.Context, req *pb.UserActivityReq) (*pb.UserActivityResp, error) {
	logrus.Infof("Retrieving activities for: %v", req.UserName)
	// Check if username exits or not
	user, err := us.dbConn.CheckIfUsernameExist(req.UserName)
	if err != nil {
		us.log.Errorf("error while checking if the username exists for user %s, err: %v", req.UserName, err)
		if err == sql.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, "user %s doesn't exist", req.UserName)
		}
		return nil, status.Errorf(codes.Internal, "cannot get the user profile")
	}

	return us.dbConn.GetUserActivities(user.Id)
}

func (us *UserSvc) UpdateUserProfile(ctx context.Context, req *pb.UpdateUserProfileReq) (*pb.UpdateUserProfileRes, error) {
	us.log.Infof("user %s is updating the profile.", req.Username)
	us.log.Infof("req: %+v", req)

	// Check if the user exists
	userDetails, err := us.dbConn.CheckIfUsernameExist(req.Username)
	if err != nil {
		us.log.Errorf("error while checking if the username exists for user %s, err: %v", req.Username, err)
		if err == sql.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, "user %s doesn't exist", req.Username)
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
				return nil, status.Errorf(codes.NotFound, "user %s doesn't exist", req.Username)
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
		AccountId: userDetails.AccountId,
	}

	userLog.IpAddress, userLog.Client = utils.IpClientConvert(req.Ip, req.Client)

	go cache.AddUserLog(us.dbConn, userLog, constants.UpdateProfile, constants.ServiceUser, constants.EventForgotPassword, us.log)

	return &pb.UpdateUserProfileRes{
		Username: dbUserInfo.Username,
	}, err
}

// TODO: Design a pipeline
// 1. Delete all the blogs of the user
// 2. Delete all the comments of the user
// 3. Delete all the likes of the user
// 4. Delete all the user interests
// 5. Delete the topics of the user
// 6. Send User a mail
func (us *UserSvc) DeleteUserAccount(ctx context.Context, req *pb.DeleteUserProfileReq) (*pb.DeleteUserProfileRes, error) {
	us.log.Infof("user %s has requested to delete the  profile.", req.Username)

	// Check if username exits or not
	user, err := us.dbConn.CheckIfUsernameExist(req.Username)
	if err != nil {
		us.log.Errorf("error while checking if the username exists for user %s, err: %v", req.Username, err)
		if err == sql.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, "user %s doesn't exist", req.Username)
		}
		return nil, status.Errorf(codes.Internal, "cannot get the user profile")
	}

	// Run delete user query
	err = us.dbConn.DeleteUserProfile(req.Username)
	if err != nil {
		us.log.Errorf("could not delete the user profile: %v", err)
		return nil, status.Errorf(codes.Internal, "cannot delete the user")
	}

	bx, err := json.Marshal(models.TheMonkeysMessage{
		Username:  user.Username,
		AccountId: user.AccountId,
		Action:    constants.USER_ACCOUNT_DELETE,
	})

	if err != nil {
		us.log.Errorf("failed to marshal message, error: %v", err)
	}

	go func() {
		err = us.qConn.PublishMessage(us.config.RabbitMQ.Exchange, us.config.RabbitMQ.RoutingKeys[0], bx)
		if err != nil {
			us.log.Errorf("failed to publish message for user: %s, error: %v", user.Username, err)
		}
	}()

	// TODO: Asynchronously delete the blogs from the blog service
	go func() {
		err = us.qConn.PublishMessage(us.config.RabbitMQ.Exchange, us.config.RabbitMQ.RoutingKeys[3], bx)
		if err != nil {
			us.log.Errorf("failed to publish message for user: %s, error: %v", user.Username, err)
		}
	}()
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

func (us *UserSvc) GetUserDetails(ctx context.Context, req *pb.UserDetailReq) (*pb.UserDetailsResp, error) {
	var (
		userInfo *models.TheMonkeysUser
		err      error
	)

	switch {
	case req.AccountId != "":
		us.log.Infof("Profile info has been requested for account id: %s.", req.AccountId)
		userInfo, err = us.dbConn.CheckIfAccIdExist(req.AccountId)
	case req.Username != "":
		us.log.Infof("Profile info has been requested for username: %s.", req.Username)
		userInfo, err = us.dbConn.CheckIfUsernameExist(req.Username)
	default:
		return nil, status.Errorf(codes.InvalidArgument, "either AccountId or Username must be provided")
	}

	if err != nil {
		us.log.Errorf("Error fetching profile info: %v", err)
		if err == sql.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, "user not found")
		}
		return nil, status.Errorf(codes.Internal, "cannot get the user profile")
	}

	return &pb.UserDetailsResp{
		Username:  userInfo.Username,
		FirstName: userInfo.FirstName,
		LastName:  userInfo.LastName,
		AccountId: userInfo.AccountId,
		Bio:       userInfo.Bio.String,
		Location:  userInfo.Location.String,
	}, nil
}

func (us *UserSvc) FollowTopics(ctx context.Context, req *pb.TopicActionReq) (*pb.TopicActionRes, error) {
	if len(req.Topic) == 0 {
		us.log.Errorf("user %s has entered no topic", req.Username)
		return nil, status.Errorf(codes.InvalidArgument, "there is no topic")
	}

	for i := range req.Topic {
		req.Topic[i] = strings.TrimSpace(req.Topic[i])
	}

	err := us.dbConn.AddUserInterest(req.Topic, req.Username)
	if err != nil {
		us.log.Errorf("Failed to update user interest for user %s, error: %v", req.Username, err)
		return nil, status.Errorf(codes.Internal, "Failed to update user interest")
	}

	// Check if the user exists
	dbUserInfo, err := us.dbConn.CheckIfUsernameExist(req.Username)
	if err != nil {
		us.log.Errorf("error while checking if the username exists for user %s, err: %v", req.Username, err)
		if err == sql.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, "user %s doesn't exist", req.Username)
		}
		return nil, status.Errorf(codes.Internal, "cannot get the user profile")
	}

	userLog := &models.UserLogs{
		AccountId: dbUserInfo.AccountId,
		IpAddress: req.Ip,
		Client:    req.Client,
	}

	userLog.IpAddress, userLog.Client = utils.IpClientConvert(req.Ip, req.Client)

	go cache.AddUserLog(us.dbConn, userLog, fmt.Sprintf(constants.FollowedTopics, req.Topic), constants.ServiceUser, constants.EventFollowTopics, us.log)

	return &pb.TopicActionRes{
		Status:  http.StatusOK,
		Message: fmt.Sprintf("user's interest in the topics %v is updated successfully", req.Topic),
	}, nil
}

func (us *UserSvc) UnFollowTopics(ctx context.Context, req *pb.TopicActionReq) (*pb.TopicActionRes, error) {
	if len(req.Topic) == 0 {
		us.log.Errorf("user %s has entered no topic", req.Username)
		return nil, status.Errorf(codes.InvalidArgument, "there is no topic")
	}

	for i := range req.Topic {
		req.Topic[i] = strings.TrimSpace(req.Topic[i])
	}

	err := us.dbConn.RemoveUserInterest(req.Topic, req.Username)
	if err != nil {
		us.log.Errorf("Failed to remove user interest for user %s, error: %v", req.Username, err)
		return nil, status.Errorf(codes.Internal, "Failed to update user interest")
	}

	// Check if the user exists
	dbUserInfo, err := us.dbConn.CheckIfUsernameExist(req.Username)
	if err != nil {
		us.log.Errorf("error while checking if the username exists for user %s, err: %v", req.Username, err)
		if err == sql.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, "user %s doesn't exist", req.Username)
		}
		return nil, status.Errorf(codes.Internal, "cannot get the user profile")
	}

	userLog := &models.UserLogs{
		AccountId: dbUserInfo.AccountId,
		IpAddress: req.Ip,
		Client:    req.Client,
	}

	userLog.IpAddress, userLog.Client = utils.IpClientConvert(req.Ip, req.Client)

	go cache.AddUserLog(us.dbConn, userLog, fmt.Sprintf(constants.UnFollowedTopics, req.Topic), constants.ServiceUser, constants.EventUnFollowTopics, us.log)

	return &pb.TopicActionRes{
		Status:  http.StatusOK,
		Message: fmt.Sprintf("user's un-followed the topics %v is updated successfully", req.Topic),
	}, nil
}

func (us *UserSvc) InviteCoAuthor(ctx context.Context, req *pb.CoAuthorAccessReq) (*pb.CoAuthorAccessRes, error) {
	us.log.Infof("user %s has requested to invite %s as a co-author.", req.BlogOwnerUsername, req.Username)
	resp, err := us.dbConn.CheckIfUsernameExist(req.Username)
	if err != nil {
		logrus.Errorf("error while checking if the username exists for user %s, err: %v", req.Username, err)
		if err == sql.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, "user %s doesn't exist", req.Username)
		}
		return nil, status.Errorf(codes.Internal, "something went wrong")
	}

	// Invite the co-author
	if err := us.dbConn.AddPermissionToAUser(req.BlogId, resp.Id, req.BlogOwnerUsername, constants.RoleEditor); err != nil {
		logrus.Errorf("error while inviting the co-author: %v", err)
		return nil, status.Errorf(codes.Internal, "something went wrong")
	}

	usa, _ := us.dbConn.CheckIfUsernameExist(req.BlogOwnerUsername)
	userLog := &models.UserLogs{
		AccountId: usa.AccountId,
		IpAddress: req.Ip,
		Client:    req.Client,
	}

	userLog.IpAddress, userLog.Client = utils.IpClientConvert(req.Ip, req.Client)

	go cache.AddUserLog(us.dbConn, userLog, fmt.Sprintf(constants.InvitedAsACoAuthor, req.Username, req.BlogId), constants.ServiceUser, constants.EventInviteCoAuthor, us.log)

	return &pb.CoAuthorAccessRes{
		Message: fmt.Sprintf("%s has been invited as a co-author", req.Username),
	}, nil
}

func (us *UserSvc) RevokeCoAuthorAccess(ctx context.Context, req *pb.CoAuthorAccessReq) (*pb.CoAuthorAccessRes, error) {
	us.log.Infof("user %s has requested to invite %s as a co-author.", req.BlogOwnerUsername, req.Username)
	resp, err := us.dbConn.CheckIfUsernameExist(req.Username)
	if err != nil {
		logrus.Errorf("error while checking if the username exists for user %s, err: %v", req.Username, err)
		if err == sql.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, "user %s doesn't exist", req.Username)
		}
		return nil, status.Errorf(codes.Internal, "something went wrong")
	}

	// Invite the co-author
	if err := us.dbConn.RevokeBlogPermissionFromAUser(req.BlogId, resp.Id, constants.RoleEditor); err != nil {
		logrus.Errorf("error while inviting the co-author: %v", err)
		return nil, status.Errorf(codes.Internal, "something went wrong")
	}

	usa, _ := us.dbConn.CheckIfUsernameExist(req.BlogOwnerUsername)

	userLog := &models.UserLogs{
		AccountId: usa.AccountId,
		IpAddress: req.Ip,
		Client:    req.Client,
	}

	userLog.IpAddress, userLog.Client = utils.IpClientConvert(req.Ip, req.Client)

	go cache.AddUserLog(us.dbConn, userLog, fmt.Sprintf(constants.RevokedCoAuthorRequest, req.Username, req.BlogId), constants.ServiceUser, constants.EventRemoveCoAuthor, us.log)

	return &pb.CoAuthorAccessRes{
		Message: fmt.Sprintf("%s has been removed from co-author", req.Username),
	}, nil
}

func (us *UserSvc) GetBlogsByUserIds(ctx context.Context, req *pb.BlogsByUserIdsReq) (*pb.BlogsByUserNameRes, error) {
	us.log.Infof("fetching blogs for user: %s", req.AccountId)

	switch req.Type {
	case "colab":
		resp, err := us.dbConn.GetCoAuthorBlogsByAccountId(req.AccountId)
		if err != nil {
			us.log.Errorf("error while fetching blogs for user %s, err: %v", req.Username, err)
			if err == sql.ErrNoRows {
				return nil, status.Errorf(codes.NotFound, "blogs for user %s doesn't exist", req.Username)
			}

			return nil, status.Errorf(codes.Internal, "something went wrong")
		}

		return resp, nil

	case "bookmark":
		resp, err := us.dbConn.GetBookmarkBlogsByAccountId(req.AccountId)
		if err != nil {
			us.log.Errorf("error while fetching blogs for user %s, err: %v", req.Username, err)
			if err == sql.ErrNoRows {
				return nil, status.Errorf(codes.NotFound, "blogs for user %s doesn't exist", req.Username)
			}

			return nil, status.Errorf(codes.Internal, "something went wrong")
		}

		return resp, nil
	default:
		return nil, status.Errorf(codes.Internal, "We don't support this operation")
	}
}

func (us *UserSvc) CreateNewTopics(ctx context.Context, req *pb.CreateTopicsReq) (*pb.CreateTopicsRes, error) {
	us.log.Infof("fetching co-authors for user: %s", req.Username)
	if len(req.Topics) == 0 {
		us.log.Errorf("user %s has entered no topic", req.Username)
		return nil, status.Errorf(codes.InvalidArgument, "there is no topic")
	}

	err := us.dbConn.CreateNewTopics(req.Topics, req.Category, req.Username)
	if err != nil {
		us.log.Errorf("error while fetching co-authors for user %s, err: %v", req.Username, err)
		if err == sql.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, "co-authors for user %s doesn't exist", req.Username)
		}

		return nil, status.Errorf(codes.Internal, "something went wrong")
	}

	usa, _ := us.dbConn.CheckIfUsernameExist(req.Username)

	userLog := &models.UserLogs{
		AccountId: usa.AccountId,
		IpAddress: req.Ip,
		Client:    req.Client,
	}
	userLog.IpAddress, userLog.Client = utils.IpClientConvert(req.Ip, req.Client)
	go cache.AddUserLog(us.dbConn, userLog, fmt.Sprintf(constants.CreatedTopics, req.Topics), constants.ServiceUser, constants.EventCreateTopics, us.log)

	return &pb.CreateTopicsRes{
		Status:  http.StatusOK,
		Message: fmt.Sprintf("topics %v has been created successfully", req.Topics),
	}, nil
}

func (us *UserSvc) BookMarkBlog(ctx context.Context, req *pb.BookMarkReq) (*pb.BookMarkRes, error) {
	user, err := us.dbConn.CheckIfUsernameExist(req.Username)
	if err != nil {
		logrus.Errorf("error while checking if the username exists for user %s, err: %v", req.Username, err)
		if err == sql.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, "user %s doesn't exist", req.Username)
		}
		return nil, status.Errorf(codes.Internal, "cannot get the user profile")
	}

	err = us.dbConn.BookMarkABlog(req.BlogId, user.Id)
	if err != nil {
		logrus.Errorf("error while bookmarking the blog: %v", err)
		return nil, status.Errorf(codes.Internal, "something went wrong")
	}

	userLog := &models.UserLogs{
		AccountId: user.AccountId,
		IpAddress: req.Ip,
		Client:    req.Client,
	}
	userLog.IpAddress, userLog.Client = utils.IpClientConvert(req.Ip, req.Client)
	go cache.AddUserLog(us.dbConn, userLog, fmt.Sprintf(constants.BookMarkedBlog, req.BlogId), constants.ServiceUser, constants.EventBookMarkBlog, us.log)

	return &pb.BookMarkRes{
		Status:  http.StatusOK,
		Message: fmt.Sprintf("blog %v has been bookmarked successfully", req.BlogId),
	}, nil
}

func (us *UserSvc) RemoveBookMark(ctx context.Context, req *pb.BookMarkReq) (*pb.BookMarkRes, error) {
	user, err := us.dbConn.CheckIfUsernameExist(req.Username)
	if err != nil {
		logrus.Errorf("error while checking if the username exists for user %s, err: %v", req.Username, err)
		if err == sql.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, "user %s doesn't exist", req.Username)
		}
		return nil, status.Errorf(codes.Internal, "cannot get the user profile")
	}

	err = us.dbConn.RemoveBookmarkFromBlog(req.BlogId, user.Id)
	if err != nil {
		logrus.Errorf("error while removing the bookmark from the blog: %v", err)
		return nil, status.Errorf(codes.Internal, "something went wrong")
	}

	userLog := &models.UserLogs{
		AccountId: user.AccountId,
		IpAddress: req.Ip,
		Client:    req.Client,
	}
	userLog.IpAddress, userLog.Client = utils.IpClientConvert(req.Ip, req.Client)
	go cache.AddUserLog(us.dbConn, userLog, fmt.Sprintf(constants.RemoveBookMark, req.BlogId), constants.ServiceUser, constants.EventRemoveBookMark, us.log)

	return &pb.BookMarkRes{
		Status:  http.StatusOK,
		Message: fmt.Sprintf("blog %v has been removed from bookmark successfully", req.BlogId),
	}, nil
}

func (us *UserSvc) FollowUser(ctx context.Context, req *pb.UserFollowReq) (*pb.UserFollowRes, error) {
	us.log.Infof("user %s has requested to follow %s.", req.FollowerUsername, req.Username)

	err := us.dbConn.FollowAUser(req.Username, req.FollowerUsername)
	if err != nil {
		logrus.Errorf("error while following the user: %v", err)
		return nil, status.Errorf(codes.Internal, "something went wrong")
	}

	usa, _ := us.dbConn.CheckIfUsernameExist(req.FollowerUsername)

	userLog := &models.UserLogs{
		AccountId: usa.AccountId,
		IpAddress: req.Ip,
		Client:    req.Client,
	}
	userLog.IpAddress, userLog.Client = utils.IpClientConvert(req.Ip, req.Client)
	go cache.AddUserLog(us.dbConn, userLog, fmt.Sprintf(constants.FollowedUser, req.Username), constants.ServiceUser, constants.EventFollowUser, us.log)

	if req.FollowerUsername != req.Username {
		// Send a notification to the user
		bx, err := json.Marshal(models.TheMonkeysMessage{
			AccountId:    usa.AccountId,
			Username:     req.FollowerUsername,
			NewUsername:  req.Username,
			Action:       constants.USER_FOLLOWED,
			Notification: fmt.Sprintf("%s has followed you", req.FollowerUsername),
		})
		if err != nil {
			logrus.Errorf("failed to marshal message, error: %v", err)
		}

		go func() {
			err = us.qConn.PublishMessage(us.config.RabbitMQ.Exchange, us.config.RabbitMQ.RoutingKeys[4], bx)
			if err != nil {
				logrus.Errorf("failed to publish message for notification service for user: %s, error: %v", req.Username, err)
			}
		}()
	}

	return &pb.UserFollowRes{
		Status:  http.StatusOK,
		Message: fmt.Sprintf("%s has been followed successfully", req.Username),
	}, nil
}

func (us *UserSvc) UnFollowUser(ctx context.Context, req *pb.UserFollowReq) (*pb.UserFollowRes, error) {
	us.log.Infof("user %s has requested to un-follow %s.", req.FollowerUsername, req.Username)

	err := us.dbConn.UnFollowAUser(req.Username, req.FollowerUsername)
	if err != nil {
		logrus.Errorf("error while un-following the user: %v", err)
		return nil, status.Errorf(codes.Internal, "something went wrong")
	}

	usa, _ := us.dbConn.CheckIfUsernameExist(req.FollowerUsername)

	userLog := &models.UserLogs{
		AccountId: usa.AccountId,
		IpAddress: req.Ip,
		Client:    req.Client,
	}
	userLog.IpAddress, userLog.Client = utils.IpClientConvert(req.Ip, req.Client)
	go cache.AddUserLog(us.dbConn, userLog, fmt.Sprintf(constants.UnFollowUser, req.Username), constants.ServiceUser, constants.EventUnFollowUser, us.log)

	return &pb.UserFollowRes{
		Status:  http.StatusOK,
		Message: fmt.Sprintf("%s has been un-followed successfully", req.Username),
	}, nil
}

func (us *UserSvc) GetFollowers(ctx context.Context, req *pb.UserDetailReq) (*pb.FollowerFollowingResp, error) {
	us.log.Infof("fetching followers for user: %s", req.Username)
	resp, err := us.dbConn.GetFollowers(req.Username)
	if err != nil {
		us.log.Errorf("error while fetching followers for user %s, err: %v", req.Username, err)
		if err == sql.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, "followers for user %s doesn't exist", req.Username)
		}

		return nil, status.Errorf(codes.Internal, "something went wrong")
	}

	var followers []*pb.User
	for _, r := range resp {
		followers = append(followers, &pb.User{
			Username:  r.Username,
			FirstName: r.FirstName,
			LastName:  r.LastName,
		})
	}

	return &pb.FollowerFollowingResp{
		Users: followers,
	}, nil
}

func (us *UserSvc) GetFollowing(ctx context.Context, req *pb.UserDetailReq) (*pb.FollowerFollowingResp, error) {
	us.log.Infof("fetching following for user: %s", req.Username)
	resp, err := us.dbConn.GetFollowings(req.Username)
	if err != nil {
		us.log.Errorf("error while fetching following for user %s, err: %v", req.Username, err)
		if err == sql.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, "following for user %s doesn't exist", req.Username)
		}

		return nil, status.Errorf(codes.Internal, "something went wrong")
	}

	var followings []*pb.User
	for _, r := range resp {
		followings = append(followings, &pb.User{
			Username:  r.Username,
			FirstName: r.FirstName,
			LastName:  r.LastName,
			AccountId: r.AccountId,
		})
	}

	return &pb.FollowerFollowingResp{
		Users: followings,
	}, nil
}

func (us *UserSvc) LikeBlog(ctx context.Context, req *pb.BookMarkReq) (*pb.BookMarkRes, error) {
	user, err := us.dbConn.CheckIfUsernameExist(req.Username)
	if err != nil {
		logrus.Errorf("error while checking if the username exists for user %s, err: %v", req.Username, err)
		if err == sql.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, "user %s doesn't exist", req.Username)
		}
		return nil, status.Errorf(codes.Internal, "cannot get the user profile")
	}

	err = us.dbConn.LikeBlog(req.Username, req.BlogId)
	if err != nil {
		logrus.Errorf("error while liking the blog: %v", err)
		return nil, status.Errorf(codes.Internal, "something went wrong")
	}

	userLog := &models.UserLogs{
		AccountId: user.AccountId,
		IpAddress: req.Ip,
		Client:    req.Client,
	}
	userLog.IpAddress, userLog.Client = utils.IpClientConvert(req.Ip, req.Client)
	go cache.AddUserLog(us.dbConn, userLog, fmt.Sprintf(constants.LikeBlog, req.BlogId), constants.ServiceUser, constants.EventBlogLike, us.log)

	// Send a notification to the user
	blog, err := us.dbConn.GetBlogsByBlogId(req.BlogId)
	if err != nil {
		logrus.Errorf("error while getting the blog: %v", err)
		return nil, status.Errorf(codes.Internal, "something went wrong")
	}

	if blog.AccountId != user.AccountId {
		bx, err := json.Marshal(models.TheMonkeysMessage{
			Username:     blog.Username,
			AccountId:    blog.AccountId,
			Action:       constants.BLOG_LIKE,
			Notification: fmt.Sprintf("%s liked your blog: %s", user.Username, blog.BlogId),
		})
		if err != nil {
			logrus.Errorf("failed to marshal message, error: %v", err)
		}

		go func() {
			err = us.qConn.PublishMessage(us.config.RabbitMQ.Exchange, us.config.RabbitMQ.RoutingKeys[4], bx)
			if err != nil {
				logrus.Errorf("failed to publish message for notification service for user: %s, error: %v", user.Username, err)
			}
			logrus.Infof("message published successfully for user: %s", user.Username)
		}()
	}

	return &pb.BookMarkRes{
		Status:  http.StatusOK,
		Message: fmt.Sprintf("blog %v has been liked successfully", req.BlogId),
	}, nil
}

func (us *UserSvc) UnlikeBlog(ctx context.Context, req *pb.BookMarkReq) (*pb.BookMarkRes, error) {
	user, err := us.dbConn.CheckIfUsernameExist(req.Username)
	if err != nil {
		logrus.Errorf("error while checking if the username exists for user %s, err: %v", req.Username, err)
		if err == sql.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, "user %s doesn't exist", req.Username)
		}
		return nil, status.Errorf(codes.Internal, "cannot get the user profile")
	}

	err = us.dbConn.UnlikeBlog(req.Username, req.BlogId)
	if err != nil {
		logrus.Errorf("error while unliking the blog: %v", err)
		return nil, status.Errorf(codes.Internal, "something went wrong")
	}

	userLog := &models.UserLogs{
		AccountId: user.AccountId,
		IpAddress: req.Ip,
		Client:    req.Client,
	}
	userLog.IpAddress, userLog.Client = utils.IpClientConvert(req.Ip, req.Client)
	go cache.AddUserLog(us.dbConn, userLog, fmt.Sprintf(constants.UnlikeBlog, req.BlogId), constants.ServiceUser, constants.EventBlogUnlike, us.log)

	return &pb.BookMarkRes{
		Status:  http.StatusOK,
		Message: fmt.Sprintf("blog %v has been unliked successfully", req.BlogId),
	}, nil
}

func (us *UserSvc) GetIfIFollowedUser(ctx context.Context, req *pb.UserFollowReq) (*pb.UserFollowRes, error) {
	us.log.Debugf("user %s has requested to follow %s.", req.FollowerUsername, req.Username)

	isFollowing, err := us.dbConn.IsUserFollowing(req.FollowerUsername, req.Username)
	if err != nil {
		logrus.Errorf("error while following the user: %v", err)
		return nil, status.Errorf(codes.Internal, "something went wrong")
	}

	return &pb.UserFollowRes{
		Status:      http.StatusOK,
		IsFollowing: isFollowing,
	}, nil
}

func (us *UserSvc) GetIfBlogLiked(ctx context.Context, req *pb.BookMarkReq) (*pb.BookMarkRes, error) {
	us.log.Debugf("user %s has requested to like blog %s.", req.Username, req.BlogId)

	isLiked, err := us.dbConn.IsBlogLikedByUser(req.Username, req.BlogId)
	if err != nil {
		logrus.Errorf("error while liking the blog: %v", err)
		return nil, status.Errorf(codes.Internal, "something went wrong")
	}

	return &pb.BookMarkRes{
		Status:  http.StatusOK,
		IsLiked: isLiked,
	}, nil
}

// GetIfBlogBookMarked checks if a blog is bookmarked by the user
func (s *UserSvc) GetIfBlogBookMarked(ctx context.Context, req *pb.BookMarkReq) (*pb.BookMarkRes, error) {
	s.log.Debugf("Checking if blog %s is bookmarked by user %s", req.BlogId, req.Username)

	isBookmark, err := s.dbConn.IsBlogBookmarkedByUser(req.Username, req.BlogId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to check if blog is bookmarked")
	}
	// Return the bookmarked status
	return &pb.BookMarkRes{
		Status:     http.StatusOK,
		Message:    "Blog bookmark status retrieved successfully",
		BookMarked: isBookmark,
	}, nil
}

// GetBookMarkCounts returns the total number of bookmarks for a blog
func (s *UserSvc) GetBookMarkCounts(ctx context.Context, req *pb.BookMarkReq) (*pb.CountResp, error) {
	s.log.Debugf("Getting bookmark count for blog %s", req.GetBlogId())
	count, err := s.dbConn.CountBlogBookmarks(req.BlogId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to get bookmark count")
	}

	return &pb.CountResp{
		Status: 200,
		Count:  int32(count),
	}, nil
}

// GetLikeCounts returns the total number of likes for a blog
func (s *UserSvc) GetLikeCounts(ctx context.Context, req *pb.BookMarkReq) (*pb.CountResp, error) {
	s.log.Debugf("Getting like count for blog %s", req.GetBlogId())
	count, err := s.dbConn.GetBlogLikeCount(req.BlogId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to get like count")
	}

	return &pb.CountResp{
		Status: 200,
		Count:  int32(count),
	}, nil
}

func (us *UserSvc) SearchUser(stream pb.UserService_SearchUserServer) error {
	us.log.Info("SearchUser stream initiated")

	for {
		// Receive data from the stream
		req, err := stream.Recv()
		if err == io.EOF {
			us.log.Info("SearchUser stream closed by client")
			return nil
		}
		if err != nil {
			us.log.Errorf("Error receiving data from stream: %v", err)
			return status.Errorf(codes.Internal, "Error reading stream: %v", err)
		}

		us.log.Infof("Received search request for username: %s or account_id: %s", req.GetUsername(), req.GetAccountId())

		// Search user based on the provided details
		var users []models.UserAccount
		//if req.GetSearchTerm() == "" {
		//
		//}

		users, err = us.dbConn.FindUsersWithPagination(req.SearchTerm, int(req.Limit), int(req.Offset))
		if err != nil {
			us.log.Errorf("Error searching user: %v", err)
			return status.Errorf(codes.Internal, "Failed to search users: %v", err)
		}

		// Send matching users back to the client
		for _, user := range users {
			resp := &pb.FollowerFollowingResp{
				Users: []*pb.User{
					{
						AccountId: user.AccountId,
						Username:  user.UserName,
						FirstName: user.FirstName,
						LastName:  user.LastName,
						Bio:       user.Bio.String,
						AvatarUrl: user.AvatarUrl.String,
					},
				},
			}
			if err := stream.Send(resp); err != nil {
				us.log.Errorf("Error sending response to stream: %v", err)
				return status.Errorf(codes.Internal, "Error sending response: %v", err)
			}
		}
	}
}

func (us *UserSvc) GetFollowersFollowingCounts(ctx context.Context, req *pb.UserDetailReq) (*pb.FollowerFollowingCountsResp, error) {
	us.log.Infof("fetching followers and following count for user: %s", req.Username)
	followers, following, err := us.dbConn.GetFollowersAndFollowingsCounts(req.Username)
	if err != nil {
		us.log.Errorf("error while fetching followers and following count for user %s, err: %v", req.Username, err)
		if err == sql.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, "followers and following count for user %s doesn't exist", req.Username)
		}

		return nil, status.Errorf(codes.Internal, "something went wrong")
	}

	return &pb.FollowerFollowingCountsResp{
		Followers: int64(followers),
		Following: int64(following),
		Status:    http.StatusOK,
	}, nil
}

func (us *UserSvc) GetBookMarks(ctx context.Context, req *pb.BookMarkReq) (*pb.BookMarkRes, error) {
	us.log.Debugf("fetching bookmarks for user: %s", req.Username)

	resp, err := us.dbConn.GetBookmarkBlogsByUsername(req.Username)
	if err != nil {
		us.log.Errorf("error while fetching bookmarks for user %s, err: %v", req.Username, err)
		if err == sql.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, "bookmarks for user %s doesn't exist", req.Username)
		}

		return nil, status.Errorf(codes.Internal, "something went wrong")
	}

	var blogIds []string
	for _, r := range resp {
		blogIds = append(blogIds, r.BlogId)
	}

	return &pb.BookMarkRes{
		Status:  http.StatusOK,
		BlogIds: blogIds,
	}, nil
}
