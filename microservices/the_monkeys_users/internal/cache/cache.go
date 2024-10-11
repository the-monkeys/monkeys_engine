package cache

import (
	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_users/internal/database"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_users/internal/models"
)

func AddUserLog(dbConn database.UserDb, user *models.UserLogs, activity, service, event string, logger *logrus.Logger) {
	logger.Infof("Adding user user log for: %v", user.AccountId)
	err := dbConn.CreateUserLog(user, activity)
	if err != nil {
		logger.Errorf("failed to store user registration log: %v. service: %s, method: %s", err, service, event)
	}
}
