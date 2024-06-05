package service_types

import "fmt"

const (
	LoginMsg              = "Please login"
	EmailPasswordWrong    = "The email/password is wrong"
	IfEmailExists         = "Email will be sent to this address if it's registered"
	EmailNotRegistered    = "The email is not registered"
	Unauthorized          = "The user is not authorized"
	MessagingQueue        = "Messaging queue failed to execute"
	FailedToGenerateToken = "Failed to generate"
)

const (
	ErrEmailPasswordWrong    = "The email/password is wrong"
	ErrIfEmailExists         = "Email will be sent to this address if it's registered"
	ErrEmailNotRegistered    = "Cannot find an account registered with the provided email"
	ErrUnauthorized          = "The user is not authorized"
	ErrMessagingQueue        = "Messaging queue failed to execute"
	ErrFailedToGenerateToken = "Failed to generate"
)

func CannotCreateToken(val string, err error) string {
	return fmt.Sprintf("cannot create a token for %s, error: %+v", val, err)
}
