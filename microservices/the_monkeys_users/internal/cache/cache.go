package cache

import (
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_users/internal/database"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_users/internal/models"
	"go.uber.org/zap"
)

func AddUserLog(dbConn database.UserDb, userLog *models.UserLogs, activity, service, event string, logger *zap.SugaredLogger) {
	logger.Debugf("Adding user user log for: %v", userLog.AccountId)
	if err := dbConn.CreateUserLog(userLog, activity); err != nil {
		logger.Errorf("failed to store user registration log: %v. service: %s, method: %s", err, service, event)
	}
}
