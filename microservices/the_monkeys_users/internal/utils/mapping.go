package utils

import (
	"time"

	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_user/pb"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_users/internal/models"
)

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
	parsedTime, err := time.Parse("2006-01-02", req.DateOfBirth)
	if err != nil {
		logrus.Errorf("couldn't parse date of birth to time.Time: %v", err)
	} else {
		logrus.Infof("Parsed date of birth: %v", parsedTime)
		dbUserInfo.DateOfBirth.Time = parsedTime
	}
	if req.Address != "" {
		dbUserInfo.Address.String = req.Address
	}
	if req.ContactNumber != "0" {
		dbUserInfo.ContactNumber.String = req.ContactNumber
	}
	if req.Linkedin != "" {
		dbUserInfo.LinkedIn.String = req.Linkedin
	}
	if req.Twitter != "" {
		dbUserInfo.Twitter.String = req.Twitter
	}
	if req.Instagram != "" {
		dbUserInfo.Instagram.String = req.Instagram
	}
	if req.Github != "" {
		dbUserInfo.Github.String = req.Github
	}

	return dbUserInfo
}
