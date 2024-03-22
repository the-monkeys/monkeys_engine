package services

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_user/pb"
	"github.com/the-monkeys/the_monkeys/constants"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_users/internal/database"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_users/internal/models"
)

type UserSvc struct {
	dbConn database.UserDb
	log    *logrus.Logger
	pb.UnimplementedUserServiceServer
}

func NewUserSvc(dbConn database.UserDb, log *logrus.Logger) *UserSvc {
	return &UserSvc{
		dbConn: dbConn,
		log:    log,
	}
}

func (us *UserSvc) GetUserProfile(ctx context.Context, req *pb.UserProfileReq) (*pb.UserProfileRes, error) {
	us.log.Infof("user %v has requested profile info.", req.Email)
	if !req.IsPrivate {
		userProfile, err := us.dbConn.GetUserProfile(req.UserName)
		if err != nil {
			us.log.Errorf("the user doesn't exists: %v", err)
			return nil, err

		}
		return &pb.UserProfileRes{
			Username:  userProfile.UserName,
			FirstName: userProfile.FirstName,
			LastName:  userProfile.LastName,
			Bio:       userProfile.Bio.String,
			AvatarUrl: userProfile.AvatarUrl.String,
		}, nil

	}

	_, err := us.dbConn.CheckIfUsernameExist(req.UserName)
	if err != nil {
		us.log.Errorf("the user doesn't exists: %v", err)
		return nil, err
	}

	userDetails, err := us.dbConn.GetMyProfile(req.UserName)
	if err != nil {
		us.log.Errorf("error while finding the user profile: %v", err)
		return nil, err
	}

	us.log.Infof("GEt profile: userDetails, %+v", userDetails)
	return &pb.UserProfileRes{
		ProfileId:   userDetails.AccountId,
		Username:    userDetails.Username,
		FirstName:   userDetails.FirstName,
		LastName:    userDetails.LastName,
		DateOfBirth: userDetails.DateOfBirth.Time.String(),
		Bio:         userDetails.Bio.String,
		AvatarUrl:   userDetails.AvatarUrl.String,
		// CreatedAt:     userDetails.CreatedAt.,
		// UpdatedAt:     userDetails.UpdatedAt,
		Address: userDetails.Address.String,
		// ContactNumber: userDetails.ContactNumber.String,
		UserStatus: userDetails.UserStatus,
	}, err
}

func (us *UserSvc) GetUserActivities(ctx context.Context, req *pb.UserActivityReq) (*pb.UserActivityRes, error) {
	logrus.Infof("Trying to fetch user activities for: %v", req.Email)

	return &pb.UserActivityRes{}, nil
}
func (us *UserSvc) UpdateUserProfile(ctx context.Context, req *pb.UpdateUserProfileReq) (*pb.UpdateUserProfileRes, error) {
	us.log.Infof("user %s is updating the profile.", req.CurrentUsername)

	// Check if the user exists
	_, err := us.dbConn.CheckIfUsernameExist(req.CurrentUsername)
	if err != nil {
		us.log.Errorf("the user doesn't exists: %v", err)
		return nil, err
	}

	// Check if the method isPartial true
	var dbUserInfo *models.UserProfileRes
	if req.Partial {
		// If isPartial is true fetch the remaining data from the db
		dbUserInfo, err = us.dbConn.GetMyProfile(req.CurrentUsername)
		if err != nil {
			us.log.Errorf("error while finding the user profile: %v", err)
			return nil, err
		}
	}

	// Map the user
	mappedDBUser := MapUserUpdateData(req, dbUserInfo)
	if err != nil {
		return nil, err
	}

	us.log.Infof("mappedDBUser: %+v\n", mappedDBUser)
	// Update the user
	err = us.dbConn.UpdateUserProfile(req.CurrentUsername, mappedDBUser)
	if err != nil {
		return nil, err
	}

	return &pb.UpdateUserProfileRes{
		Username: mappedDBUser.Username,
	}, err
}

// MapUserUpdateData maps the user update request data to the database model.
func MapUserUpdateData(req *pb.UpdateUserProfileReq, dbUserInfo *models.UserProfileRes) *models.UserProfileRes {
	if req.Username != "" {
		dbUserInfo.Username = req.Username
	}
	if req.FirstName != "" {
		dbUserInfo.FirstName = req.FirstName
	}
	if req.LastName != "" {
		dbUserInfo.LastName = req.LastName
	}
	if req.Bio != "" {
		dbUserInfo.Bio.String = req.Bio
	}
	if req.DateOfBirth != "" {
		time, _ := time.Parse(constants.DateTimeFormat, req.DateOfBirth)
		dbUserInfo.DateOfBirth.Time = time
	}
	if req.Address != "" {
		dbUserInfo.Address.String = req.Address
	}
	if req.ContactNumber != "0" {
		dbUserInfo.ContactNumber.String = req.ContactNumber
	}

	return dbUserInfo
}
