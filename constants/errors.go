package constants

import "errors"

// ErrNotFound represents a "not found" error
var ErrNotFound = errors.New("not found")

// ErrBadRequest represents a "bad request" error
var ErrBadRequest = errors.New("bad request")

// ErrInternal represents an "internal server error" error
var ErrInternal = errors.New("internal server error")

// ErrSQLDBClosed represents a "sql: database is closed" error
var ErrSQLDBClosed = errors.New("sql: database is closed")

// ErrTokenIsNotValid represents a "the token is invalid" error
var ErrTokenIsNotValid = errors.New("the token is invalid")
