package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/the-monkeys/the_monkeys/microservices/rabbitmq"

	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_authz/pb"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/constants"
	"github.com/the-monkeys/the_monkeys/microservices/service_types"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_authz/internal/cache"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_authz/internal/db"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_authz/internal/models"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_authz/internal/utils"
)

type AuthzSvc struct {
	dbConn db.AuthDBHandler
	jwt    utils.JwtWrapper
	config *config.Config
	logger *logrus.Logger
	qConn  rabbitmq.Conn
	pb.UnimplementedAuthServiceServer
}

func NewAuthzSvc(dbCli db.AuthDBHandler, jwt utils.JwtWrapper, config *config.Config, qConn rabbitmq.Conn) *AuthzSvc {
	return &AuthzSvc{
		dbConn: dbCli,
		jwt:    jwt,
		config: config,
		logger: logrus.New(),
		qConn:  qConn,
	}
}

func (as *AuthzSvc) RegisterUser(ctx context.Context, req *pb.RegisterUserRequest) (*pb.RegisterUserResponse, error) {
	as.logger.Infof("got the request data for : %+v", req.Email)
	user := &models.TheMonkeysUser{}

	if err := utils.ValidateRegisterUserRequest(req); err != nil {
		as.logger.Errorf("incomplete request body provided for email %s, error: %+v", req.Email, err)
		return nil, status.Errorf(codes.InvalidArgument, "Incomplete request body provided for email %s", req.Email)
	}

	// Check if the user exists with the same email id return conflict
	_, err := as.dbConn.CheckIfEmailExist(req.Email)
	if err == nil {
		if err == sql.ErrNoRows {
			as.logger.Infof("creating a new user with email: %v", req.Email)
		} else {
			return nil, status.Errorf(codes.AlreadyExists, "The user with email %s already exists", req.Email)
		}
	}

	hash := string(utils.GenHash())
	encHash := utils.HashPassword(hash)

	// Create a userId and username
	user.AccountId = utils.RandomString(16)
	user.Username = utils.RandomString(12)
	user.FirstName = req.FirstName
	user.LastName = req.GetLastName()
	user.Email = req.GetEmail()
	user.Password = utils.HashPassword(req.Password)
	user.UserStatus = "active"
	user.EmailVerificationToken = encHash
	user.EmailVerificationTimeout = sql.NullTime{
		Time:  time.Now().Add(time.Hour * 1),
		Valid: true,
	}
	if req.LoginMethod.String() == pb.RegisterUserRequest_LoginMethod_name[0] {
		user.LoginMethod = "the-monkeys"
	}

	user.IpAddress, user.Client = utils.IpClientConvert(req.IpAddress, req.Client)

	as.logger.Infof("registering the user with email %v", req.Email)
	userId, err := as.dbConn.RegisterUser(user)
	if err != nil {
		as.logger.Errorf("cannot register the user %s, error: %v", user.Email, err)
		return nil, status.Errorf(codes.Internal, "cannot register the user, something went wrong")
	}

	// Send email verification mail as a routine else the register api gets slower
	emailBody := utils.EmailVerificationHTML(user.FirstName, user.LastName, user.Username, hash)
	go func() {
		err := as.SendMail(user.Email, emailBody)
		if err != nil {
			log.Printf("Failed to send mail post registration: %v", err)
		}
		as.logger.Info("Email Sent!")
	}()

	go cache.AddUserLog(as.dbConn, user, constants.Register, constants.ServiceAuth, constants.EventRegister, as.logger)

	as.logger.Infof("user %s is successfully registered.", user.Email)

	// Generate and return token
	token, err := as.jwt.GenerateToken(user)
	if err != nil {
		as.logger.Errorf("failed to generate token for user %s: %v", user.Email, err)
		return nil, status.Errorf(codes.Aborted, "The user with email %s is successfully registered, try to log in", user.Email)
	}

	bx, err := json.Marshal(models.TheMonkeysMessage{
		Username:  user.Username,
		AccountId: user.AccountId,
		Action:    constants.USER_PROFILE_DIRECTORY_CREATE,
	})
	if err != nil {
		as.logger.Errorf("failed to marshal message, error: %v", err)
	}

	go as.qConn.PublishMessage(as.config.RabbitMQ.Exchange, as.config.RabbitMQ.RoutingKeys[0], bx)

	return &pb.RegisterUserResponse{
		StatusCode:    http.StatusCreated,
		Token:         token,
		EmailVerified: false,
		Username:      user.Username,
		Email:         user.Email,
		UserId:        userId,
		FirstName:     user.FirstName,
		LastName:      user.LastName,
		AccountId:     user.AccountId,
	}, nil
}

// Validate user runs to check to validate the user. It checks
// If the token is correct
// If the token is expired
// Is the token belongs to the user
// Is the user existing in the db or an active user
func (as *AuthzSvc) Validate(ctx context.Context, req *pb.ValidateRequest) (*pb.ValidateResponse, error) {
	as.logger.Infof("validating user id %s or email %s", req.UserName, req.Email)

	claims, err := as.jwt.ValidateToken(req.Token)
	if err != nil {
		as.logger.Errorf("cannot validate the auth token, error: %v", err)
		return nil, status.Errorf(codes.Unauthenticated, "couldn't validate auth token")
	}

	// Check if the email exists
	user, err := as.dbConn.CheckIfEmailExist(claims.Email)
	if err != nil {
		as.logger.Errorf("cannot validate token as the email %s doesn't exist, error: %+v", claims.Email, err)
		return nil, status.Errorf(codes.NotFound, "email does not exist")
	}

	return &pb.ValidateResponse{
		StatusCode: http.StatusOK,
		UserId:     user.Id,
		Email:      claims.Email,
		UserName:   user.Username,
	}, nil
}

func (as *AuthzSvc) Login(ctx context.Context, req *pb.LoginUserRequest) (*pb.LoginUserResponse, error) {
	as.logger.Infof("user has requested to login with email: %s", req.Email)
	// Check if the user is existing the db or not
	user, err := as.dbConn.CheckIfEmailExist(req.Email)
	if err != nil {
		as.logger.Errorf("cannot find user with email %s, error: %v", req.Email, err)
		return nil, status.Errorf(codes.NotFound, "email does not exist")
	}

	// Check if the password match with the password hash
	if !utils.CheckPasswordHash(req.Password, user.Password) {
		as.logger.Errorf("password incorrect for email %s, error: %v", req.Email, err)
		return nil, status.Errorf(codes.Unauthenticated, "email/password incorrect")
	}

	token, err := as.jwt.GenerateToken(user)
	if err != nil {
		as.logger.Errorf(service_types.CannotCreateToken(req.Email, err))
		return nil, status.Errorf(codes.Internal, "cannot generate the token: %v", err)
	}

	user.IpAddress, user.Client = utils.IpClientConvert(req.IpAddress, req.Client)

	go cache.AddUserLog(as.dbConn, user, constants.Login, constants.ServiceAuth, constants.EventLogin, as.logger)

	resp := &pb.LoginUserResponse{
		StatusCode:    http.StatusOK,
		Token:         token,
		EmailVerified: false,
		UserName:      user.Username,
		Email:         user.Email,
		UserId:        user.Id,
		FirstName:     user.FirstName,
		LastName:      user.LastName,
		AccountId:     user.AccountId,
	}
	return resp, nil
}

func (as *AuthzSvc) ForgotPassword(ctx context.Context, req *pb.ForgotPasswordReq) (*pb.ForgotPasswordRes, error) {
	as.logger.Infof("User %s has forgotten their password", req.Email)

	// Check if the user exists in the database
	user, err := as.dbConn.CheckIfEmailExist(req.Email)
	if err != nil {
		as.logger.Errorf("Error checking if username exists in the database: %v", err)
		if err == sql.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, "If the account is registered with this email, you’ll receive an email verification link to reset your password.")
		}
		return nil, status.Errorf(codes.Internal, "Something went wrong while getting user")
	}

	var alphaNumRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890_")
	randomHash := make([]rune, 64)
	for i := 0; i < 64; i++ {
		// Intn() returns, as an int, a non-negative pseudo-random number in [0,n).
		randomHash[i] = alphaNumRunes[rand.Intn(len(alphaNumRunes)-1)]
	}

	emailVerifyHash := utils.HashPassword(string(randomHash))

	if err = as.dbConn.UpdatePasswordRecoveryToken(emailVerifyHash, user); err != nil {
		as.logger.Errorf("Error occurred while updating email verification token for %s, error: %v", req.Email, err)
		return nil, status.Errorf(codes.Internal, "Something went wrong while updating verification token")
	}

	emailBody := utils.ResetPasswordTemplate(user.FirstName, user.LastName, string(randomHash), user.Username)
	go func() {
		err := as.SendMail(req.Email, emailBody)
		if err != nil {
			as.logger.Errorf("Failed to send mail for password recovery: %v", err)
		}
	}()

	user.IpAddress, user.Client = utils.IpClientConvert(req.IpAddress, req.Client)

	go cache.AddUserLog(as.dbConn, user, constants.ForgotPassword, constants.ServiceAuth, constants.EventForgotPassword, as.logger)

	return &pb.ForgotPasswordRes{
		StatusCode: 200,
		Message:    "Verification link has been sent to the email!",
	}, nil
}

func (as *AuthzSvc) ResetPassword(ctx context.Context, req *pb.ResetPasswordReq) (*pb.ResetPasswordRes, error) {
	as.logger.Infof("user %s has requested to reset their password", req.Username)

	user, err := as.dbConn.CheckIfUsernameExist(req.Username)
	if err != nil {
		as.logger.Errorf("Error checking if username exists in the database: %v", err)
		if err == sql.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, "username not found")
		}
		return nil, status.Errorf(codes.Internal, "Something went wrong while getting user")
	}

	// timeTill, err := time.Parse(time.RFC3339, user.PasswordVerificationTimeout.Time.String())
	timeTill, err := time.Parse(time.RFC3339, user.PasswordVerificationTimeout.Time.Format(time.RFC3339))
	if err != nil {
		as.logger.Errorf("timeout couldn't be verified: %v", err)
		return nil, status.Errorf(codes.Internal, "timeout couldn't be verified")
	}

	if timeTill.Before(time.Now()) {
		as.logger.Errorf("the token has already expired, error: %+v", err)
		return nil, status.Errorf(codes.Unauthenticated, "token expired already")
	}

	// Verify reset token
	if ok := utils.CheckPasswordHash(req.Token, user.PasswordVerificationToken.String); !ok {
		as.logger.Errorf("the token didn't match, error: %+v", err)
		return nil, status.Errorf(codes.Unauthenticated, "token didn't match")
	}

	as.logger.Infof("Assigning a token to the user: %s having email: %s to reset their password", user.Username, user.Email)
	// Generate and return token
	token, err := as.jwt.GenerateToken(user)
	if err != nil {
		as.logger.Errorf(service_types.CannotCreateToken(req.Email, err))
		return nil, status.Errorf(codes.Internal, "could not create token")
	}

	user.IpAddress, user.Client = utils.IpClientConvert(req.IpAddress, req.Client)

	go cache.AddUserLog(as.dbConn, user, constants.VerifiedEmailForPassChange, constants.ServiceAuth, constants.EventVerifiedEmailForPassChange, as.logger)

	return &pb.ResetPasswordRes{
		StatusCode: http.StatusOK,
		Token:      token,
		// EmailVerified: false,
		UserName:  user.Username,
		Email:     user.Email,
		UserId:    user.Id,
		FirstName: user.FirstName,
		LastName:  user.LastName,
	}, nil
}

func (as *AuthzSvc) UpdatePassword(ctx context.Context, req *pb.UpdatePasswordReq) (*pb.UpdatePasswordRes, error) {
	as.logger.Infof("updating password for: %+v", req)

	// Check if the username exists in the database
	user, err := as.dbConn.CheckIfUsernameExist(req.Username)
	if err != nil {
		as.logger.Errorf("Error checking if username exists in the database: %v", err)
		if err == sql.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, "User doesn't exist")
		}
		return nil, status.Errorf(codes.Internal, "Something went wrong while verifying user")
	}

	encHash := utils.HashPassword(req.Password)

	if err := as.dbConn.UpdatePassword(encHash, &models.TheMonkeysUser{
		Id:       user.Id,
		Email:    req.Email,
		Username: req.Username,
	}); err != nil {
		as.logger.Errorf("could not update password for user %v, err: %v", req.Username, err)
		return nil, status.Errorf(codes.Internal, "could not update the password")
	}

	as.logger.Infof("updated password for: %+v", req.Email)

	user.IpAddress, user.Client = utils.IpClientConvert(req.IpAddress, req.Client)

	go cache.AddUserLog(as.dbConn, user, constants.UpdatedPassword, constants.ServiceAuth, constants.EventUpdatedPassword, as.logger)

	return &pb.UpdatePasswordRes{
		StatusCode: http.StatusOK,
	}, nil
}

func (as *AuthzSvc) RequestForEmailVerification(ctx context.Context, req *pb.EmailVerificationReq) (*pb.EmailVerificationRes, error) {
	if req.Email == "" {
		return nil, constants.ErrBadRequest
	}
	as.logger.Infof("user %v has requested for email verification", req.Email)

	user, err := as.dbConn.CheckIfEmailExist(req.Email)
	if err != nil {
		as.logger.Infof("user %v doesn't exist, error: %v", req.Email, err)
		return nil, status.Errorf(codes.NotFound, "User doesn't exist")
	}

	as.logger.Infof("generating verification email token for: %s", req.GetEmail())
	hash := string(utils.GenHash())
	encHash := utils.HashPassword(hash)

	user.EmailVerificationToken = encHash
	user.EmailVerificationTimeout = sql.NullTime{
		Time:  time.Now().Add(time.Minute * 5),
		Valid: true, // Valid is true if Time is not NULL
	}

	if err := as.dbConn.UpdateEmailVerificationToken(user); err != nil {
		as.logger.Infof("error occurred while updating email verification token: %v", err)
		return nil, status.Errorf(codes.Internal, "error occurred while updating email verification token")
	}

	emailBody := utils.EmailVerificationHTML(user.FirstName, user.LastName, user.Username, hash)
	as.logger.Infof("Sending verification email to: %s", req.GetEmail())

	// TODO: Handle error of the go routine
	go func() {
		err := as.SendMail(user.Email, emailBody)
		if err != nil {
			// Handle error
			log.Printf("Failed to send mail for password recovery: %v", err)
		}
		as.logger.Info("Email Sent!")
	}()

	user.IpAddress, user.Client = utils.IpClientConvert(req.IpAddress, req.Client)

	go cache.AddUserLog(as.dbConn, user, constants.RequestForEmailVerification, constants.ServiceAuth, constants.EventRequestForEmailVerification, as.logger)

	return &pb.EmailVerificationRes{
		StatusCode: http.StatusOK,
	}, nil
}

func (as *AuthzSvc) VerifyEmail(ctx context.Context, req *pb.VerifyEmailReq) (*pb.VerifyEmailRes, error) {
	// Check if the username exists in the database
	user, err := as.dbConn.CheckIfUsernameExist(req.Username)
	if err != nil {
		as.logger.Errorf("Error checking if username exists in the database: %v", err)
		if err == sql.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, "User doesn't exist")
		}
		return nil, status.Errorf(codes.Internal, "Something went wrong while verifying user")
	}

	// Parse the email verification timeout from the user
	timeTill, err := time.Parse(time.RFC3339, user.EmailVerificationTimeout.Time.Format(time.RFC3339))
	if err != nil {
		as.logger.Errorf("Failed to parse email verification timeout: %v", err)
		return nil, status.Errorf(codes.Unauthenticated, "Failed to parse email verification timeout: %v", err)
	}

	// Check if the email verification timeout has expired
	if timeTill.Before(time.Now()) {
		as.logger.Errorf("Email verification token expired already for %s, error: %v", req.Email, err)
		return nil, status.Errorf(codes.Unauthenticated, "Email verification token expired already or incorrect token")
	}

	// Verify reset token
	if ok := utils.CheckPasswordHash(req.Token, user.EmailVerificationToken); !ok {
		as.logger.Errorf("The token didn't match, error: %+v", err)
		return nil, status.Errorf(codes.Unauthenticated, "Email verification token expired already or incorrect token")
	}

	// Update email verification status
	err = as.dbConn.UpdateEmailVerificationStatus(user)
	if err != nil {
		as.logger.Errorf("Cannot update the verification details for %s, error: %v", req.Email, err)
		return nil, status.Errorf(codes.Internal, "Couldn't update email verification token")
	}

	as.logger.Infof("Verified email: %s", user.Email)

	// Set default IP address and client if not provided
	user.IpAddress, user.Client = utils.IpClientConvert(req.IpAddress, req.Client)

	// Add user log asynchronously
	go cache.AddUserLog(as.dbConn, user, constants.VerifyEmail, constants.ServiceAuth, constants.EventVerifiedEmail, as.logger)

	// Return a success response with status code 200
	return &pb.VerifyEmailRes{
		StatusCode: 200,
	}, nil
}

func (as *AuthzSvc) UpdateUsername(ctx context.Context, req *pb.UpdateUsernameReq) (*pb.UpdateUsernameRes, error) {
	// Check if the user exists
	user, err := as.dbConn.CheckIfUsernameExist(req.CurrentUsername)
	if err != nil {
		as.logger.Errorf("error while checking if the username exists for user %s, err: %v", req.CurrentUsername, err)
		if err == sql.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, fmt.Sprintf("user %s doesn't exist", req.CurrentUsername))
		}
		return nil, status.Errorf(codes.Internal, "cannot get the user profile")
	}

	// Update the username
	err = as.dbConn.UpdateUserName(req.CurrentUsername, req.NewUsername)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not update the username")
	}

	bx, err := json.Marshal(models.TheMonkeysMessage{
		Username:    user.Username,
		NewUsername: req.NewUsername,
		AccountId:   user.AccountId,
		Action:      constants.USER_PROFILE_DIRECTORY_UPDATE,
	})
	if err != nil {
		as.logger.Errorf("error while marshalling the message queue data, err: %v", err)
		return nil, status.Errorf(codes.Internal, "something went wrong")
	}

	go as.qConn.PublishMessage(as.config.RabbitMQ.Exchange, as.config.RabbitMQ.RoutingKeys[0], bx)

	user.IpAddress, user.Client = utils.IpClientConvert(req.Ip, req.Client)

	// Add a user log
	go cache.AddUserLog(as.dbConn, user, constants.UpdatedUserName, constants.ServiceAuth, constants.EventUpdateUsername, as.logger)

	token, err := as.jwt.GenerateToken(user)
	if err != nil {
		as.logger.Errorf(service_types.CannotCreateToken(req.NewUsername, err))
		as.logger.Errorf("error while marshalling the message queue data, err: %v", err)
		return nil, status.Errorf(codes.Internal, "something went wrong")
	}

	return &pb.UpdateUsernameRes{
		StatusCode:    http.StatusOK,
		Token:         token,
		EmailVerified: false,
		UserName:      req.NewUsername,
		Email:         user.Email,
		UserId:        user.Id,
		FirstName:     user.FirstName,
		LastName:      user.LastName,
		AccountId:     user.AccountId,
	}, nil
}

func (as *AuthzSvc) UpdatePasswordWithPassword(ctx context.Context, req *pb.UpdatePasswordWithPasswordReq) (*pb.UpdatePasswordWithPasswordRes, error) {
	as.logger.Infof("updating password of user: %s", req.Username)

	// Check if the user exists
	user, err := as.dbConn.CheckIfUsernameExist(req.Username)
	if err != nil {
		as.logger.Errorf("error while checking if the username exists for user %s, err: %v", req.Username, err)
		if err == sql.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, fmt.Sprintf("user %s doesn't exist", req.Username))
		}
		return nil, status.Errorf(codes.Internal, "cannot get the user profile")
	}

	// Check if the password match with the password hash
	if !utils.CheckPasswordHash(req.CurrentPassword, user.Password) {
		return nil, status.Errorf(codes.Unauthenticated, "password didn't match, cannot update password")
	}

	// Hash the new password
	hash := utils.HashPassword(req.NewPassword)

	// update the password
	err = as.dbConn.UpdatePassword(hash, user)
	if err != nil {
		as.logger.Errorf("error while updating the password for user %s, err: %v", req.Username, err)
		return nil, status.Errorf(codes.Internal, "cannot update the password")
	}

	user.IpAddress, user.Client = utils.IpClientConvert(req.IpAddress, req.Client)

	// Add a user log
	go cache.AddUserLog(as.dbConn, user, constants.UpdatedPassword, constants.ServiceAuth, constants.EventUpdatedPassword, as.logger)

	// Return
	return &pb.UpdatePasswordWithPasswordRes{
		StatusCode: http.StatusOK,
	}, nil
}

func (as *AuthzSvc) UpdateEmailId(ctx context.Context, req *pb.UpdateEmailIdReq) (*pb.UpdateEmailIdRes, error) {
	as.logger.Infof("updating email of user: %s", req.Username)

	user, err := as.dbConn.CheckIfUsernameExist(req.Username)
	if err != nil {
		as.logger.Errorf("error while checking if the username exists for user %s, err: %v", req.Username, err)
		if err == sql.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, fmt.Sprintf("user %s doesn't exist", req.Username))
		}
		return nil, status.Errorf(codes.Internal, "cannot get the user profile")
	}

	// check if the email is already in use
	_, err = as.dbConn.CheckIfEmailExist(req.NewEmail)
	if err == nil {
		if err == sql.ErrNoRows {
			as.logger.Infof("updating a new email: %v", req.NewEmail)
		} else {
			return nil, status.Errorf(codes.AlreadyExists, "The user with email %s already in use", req.NewEmail)
		}
	}

	hash := string(utils.GenHash())
	encHash := utils.HashPassword(hash)

	user.EmailVerificationToken = encHash
	user.EmailVerificationTimeout = sql.NullTime{
		Time:  time.Now().Add(time.Hour * 1),
		Valid: true,
	}

	// else update the email address
	err = as.dbConn.UpdateEmailId(req.NewEmail, user)
	if err != nil {
		as.logger.Errorf("error while updating the email for user %s, err: %v", req.Username, err)
		return nil, status.Errorf(codes.Internal, "cannot update the email")
	}

	// Send email verification mail as a routine else the register api gets slower
	emailBody := utils.EmailVerificationHTML(user.FirstName, user.LastName, user.Username, hash)
	go func() {
		err := as.SendMail(user.Email, emailBody)
		if err != nil {
			log.Printf("Failed to send mail post registration: %v", err)
		}
		as.logger.Info("Email Sent!")
	}()

	user.IpAddress, user.Client = utils.IpClientConvert(req.IpAddress, req.Client)
	// Add a user log
	go cache.AddUserLog(as.dbConn, user, constants.ChangedEmail, constants.ServiceAuth, constants.EventUpdateEmail, as.logger)

	token, err := as.jwt.GenerateToken(user)
	if err != nil {
		as.logger.Errorf(service_types.CannotCreateToken(user.Username, err))
		as.logger.Errorf("error while marshalling the message queue data, err: %v", err)
		return nil, status.Errorf(codes.Internal, "something went wrong")
	}

	return &pb.UpdateEmailIdRes{
		StatusCode:    http.StatusOK,
		Token:         token,
		EmailVerified: false,
		UserName:      user.Username,
		Email:         user.Email,
		UserId:        user.Id,
		FirstName:     user.FirstName,
		LastName:      user.LastName,
		AccountId:     user.AccountId,
	}, nil
}
