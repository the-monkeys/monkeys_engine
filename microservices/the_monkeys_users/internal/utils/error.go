package utils

import (
	"database/sql"

	"github.com/the-monkeys/the_monkeys/logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func Errors(err error) error {
	log := logger.ZapForService("tm_users")
	switch err {
	case sql.ErrNoRows:
		log.Debugf("cannot find the row")
		return status.Errorf(codes.NotFound, "failed to find the record, error: %v", err)
	case sql.ErrTxDone:
		log.Debugf("The transaction has already been committed or rolled back.")
		return status.Errorf(codes.Internal, "failed to find the record, error: %v", err)
	case sql.ErrConnDone:
		log.Debugf("The database connection has been closed.")
		return status.Errorf(codes.Unavailable, "failed to find the record, error: %v", err)
	default:
		log.Debugf("An internal server error occurred: %v", err)
		return status.Errorf(codes.Internal, "failed to find the record, error: %v", err)
	}
}
