package logger

import (
	"os"

	"github.com/sirupsen/logrus"
)

// GetLogger initializes and returns a production-ready logger.
func GetLogger() *logrus.Logger {
	logger := logrus.New()

	logger.Out = os.Stdout

	os.Setenv("APP_ENV", "production")
	env := os.Getenv("APP_ENV") // Example: APP_ENV could be "production" or "development"

	if env == "production" {
		logger.SetLevel(logrus.InfoLevel)
	} else {
		logger.SetLevel(logrus.DebugLevel)
	}

	if env == "production" {
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02T15:04:05Z07:00", // ISO8601 format
		})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true, // More readable logs for development
		})
	}

	return logger
}
