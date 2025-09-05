package cache

import (
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_authz/internal/db"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_authz/internal/models"
	"go.uber.org/zap"
)

func AddUserLog(dbConn db.AuthDBHandler, user *models.TheMonkeysUser, activity, service, event string, logger *zap.SugaredLogger) {
	if err := dbConn.CreateUserLog(user, activity); err != nil {
		logger.Errorf("failed to store user registration log: %v. service: %s, method: %s", err, service, event)
	}
}
