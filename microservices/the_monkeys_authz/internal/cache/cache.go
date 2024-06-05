package cache

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_authz/internal/db"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_authz/internal/models"
)

func AddUserLog(dbConn db.AuthDBHandler, user *models.TheMonkeysUser, activity, service, event string, logger *logrus.Logger) {
	err := dbConn.CreateUserLog(user, fmt.Sprintf(activity, user.AccountId))
	if err != nil {
		logger.Errorf("failed to store user registration log: %v. service: %s, method: %s", err, service, event)
	}
}
