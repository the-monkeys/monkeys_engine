package utils

import (
	"time"

	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_user/pb"
	"github.com/the-monkeys/the_monkeys/logger"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_users/internal/models"
)

// MapUserUpdateData maps the user update request data to the database model.
func MapUserUpdateDataPatch(req *pb.UpdateUserProfileReq, dbUserInfo *models.UserProfileRes) *models.UserProfileRes {
	log := logger.ZapForService("tm_users")
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
		log.Errorf("couldn't parse date of birth to time.Time: %v", err)
	} else {
		log.Debugf("Parsed date of birth: %v", parsedTime)
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

func MapUserUpdateDataPut(req *pb.UpdateUserProfileReq, dbUserInfo *models.UserProfileRes) *models.UserProfileRes {
	log := logger.ZapForService("tm_users")
	dbUserInfo.Username = req.Username
	dbUserInfo.FirstName = req.FirstName
	dbUserInfo.LastName = req.LastName
	dbUserInfo.Bio.String = req.Bio
	parsedTime, err := time.Parse("2006-01-02", req.DateOfBirth)
	if err != nil {
		log.Errorf("couldn't parse date of birth to time.Time: %v", err)
	} else {
		log.Debugf("Parsed date of birth: %v", parsedTime)
		dbUserInfo.DateOfBirth.Time = parsedTime
	}
	dbUserInfo.Address.String = req.Address
	dbUserInfo.ContactNumber.String = req.ContactNumber
	dbUserInfo.LinkedIn.String = req.Linkedin
	dbUserInfo.Twitter.String = req.Twitter
	dbUserInfo.Instagram.String = req.Instagram
	dbUserInfo.Github.String = req.Github

	return dbUserInfo
}
