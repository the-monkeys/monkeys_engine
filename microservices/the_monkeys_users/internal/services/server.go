package services

import (
	"context"
	"errors"

	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_user/pb"
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
	if !req.IsPrivate {
		return nil, errors.New("cannot find the private profile")
	}

	us.log.Infof("user %v has requested profile info.", req.Email)
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
		Address:       userDetails.Address.String,
		ContactNumber: userDetails.ContactNumber.Int64,
		UserStatus:    userDetails.UserStatus,
	}, err
}

func (us *UserSvc) GetUserActivities(ctx context.Context, req *pb.UserActivityReq) (*pb.UserActivityRes, error) {
	logrus.Infof("Trying to fetch user activities for: %v", req.Email)

	return &pb.UserActivityRes{}, nil
}
func (us *UserSvc) UpdateUserProfile(ctx context.Context, req *pb.UpdateUserProfileReq) (*pb.UpdateUserProfileRes, error) {
	us.log.Infof("user %s is updating the profile.", req.Username)

	// Check if the user exists
	_, err := us.dbConn.CheckIfEmailExist(req.Email)
	if err != nil {
		us.log.Errorf("the user doesn't exists: %v", err)
		return nil, err
	}

	// Check if the method isPartial true
	var userDetails *models.UserProfileRes
	if req.Partial == true {
		// If isPartial is true fetch the remaining data from the db
		userDetails, err = us.dbConn.GetMyProfile(req.Email)
		if err != nil {
			us.log.Errorf("error while finding the user profile: %v", err)
			return nil, err
		}
	}

	// Map the user
	_, err = MapUserUpdateData(req, userDetails)
	if err != nil {
		return nil, err
	}

	// Update the user

	return nil, nil
}

// MapUserUpdateData maps the user update request data to the database model.
func MapUserUpdateData(req *pb.UpdateUserProfileReq, userDetails *models.UserProfileRes) (models.UserAccount, error) {
	// Map the request data to the database model
	userModel := models.UserAccount{
		Email: req.Email,
	}

	// Check if the first name is provided
	if req.FirstName != "" {
		userModel.FirstName = req.FirstName
	}

	// Check if the last name is provided
	if req.LastName != "" {
		userModel.LastName = req.LastName
	}

	// Check if the date of birth is provided
	if req.DateOfBirth != "" {

	}

	// Check if the bio is provided
	if req.Bio != "" {
	}

	// // Check if the avatar URL is provided
	// if req.AvatarUrl != "" {
	// 	userModel.AvatarUrl = req.AvatarUrl
	// }

	// Check if the address is provided
	if req.Address != "" {
	}

	// Check if the contact number is provided
	if req.ContactNumber != "" {
	}

	return userModel, nil
}
