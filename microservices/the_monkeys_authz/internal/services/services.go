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
	logrus.Infof("got the request data for : %+v", req.Email)
	user := &models.TheMonkeysUser{}
	if req.Email == "" || req.FirstName == "" || req.LastName == "" || req.Password == "" {
		return &pb.RegisterUserResponse{
			StatusCode: http.StatusBadRequest,
			Error: &pb.Error{
				Status:  http.StatusBadRequest,
				Message: "Incomplete information: email, first name, last name and password are required.",
				Error:   "All fields (email, first_name, last_name and password) are mandatory and must be provided.",
			},
		}, nil
	}

	// Check if the user exists with the same email id return conflict
	_, err := as.dbConn.CheckIfEmailExist(req.Email)
	if err == nil {
		logrus.Errorf("cannot register the user, as the email %s is existing already", req.Email)
		return &pb.RegisterUserResponse{
			StatusCode: http.StatusConflict,
			Error: &pb.Error{
				Status:  http.StatusConflict,
				Message: "The email is already registered",
				Error:   "An account is already registered with this email",
			},
		}, nil
	}

	if req.IpAddress == "" {
		req.IpAddress = "127.0.0.1"
	}

	if req.Client == "" {
		req.Client = "Others"
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
		Time:  time.Now().Add(time.Hour * 24),
		Valid: true,
	}
	if req.LoginMethod.String() == pb.RegisterUserRequest_LoginMethod_name[0] {
		user.LoginMethod = "the-monkeys"
	}

	user.IpAddress = req.IpAddress
	user.Client = req.Client

	logrus.Infof("registering the user with email %v", req.Email)
	userId, err := as.dbConn.RegisterUser(user)
	if err != nil {
		return nil, err
	}

	// Send email verification mail as a routine else the register api gets slower
	emailBody := utils.EmailVerificationHTML(user.Username, hash)
	go func() {
		err := as.SendMail(user.Email, emailBody)
		if err != nil {
			log.Printf("Failed to send mail post registration: %v", err)
		}
		logrus.Info("Email Sent!")
	}()

	go cache.AddUserLog(as.dbConn, user, constants.Register, constants.ServiceAuth, constants.EventRegister, as.logger)

	logrus.Infof("user %s is successfully registered.", user.Email)

	// Generate and return token
	token, err := as.jwt.GenerateToken(user)
	if err != nil {
		logrus.Errorf(service_types.CannotCreateToken(req.Email, err))
		return &pb.RegisterUserResponse{
			StatusCode: http.StatusInternalServerError,
			Error: &pb.Error{
				Status:  http.StatusInternalServerError,
				Message: "You are successfully registered, try to login!",
				Error:   service_types.ErrFailedToGenerateToken,
			},
		}, nil
	}

	bx, err := json.Marshal(models.TheMonkeysMessage{
		Username:  user.Username,
		AccountId: user.AccountId,
		Action:    constants.USER_PROFILE_DIRECTORY_CREATE,
	})
	if err != nil {
		return &pb.RegisterUserResponse{
			StatusCode: http.StatusInternalServerError,
			Error: &pb.Error{
				Status:  http.StatusInternalServerError,
				Message: "You are successfully registered, try to login!",
				Error:   service_types.ErrMessagingQueue,
			},
		}, nil
	}

	go as.qConn.PublishDefaultProfilePhoto(as.config.RabbitMQ.Exchange, as.config.RabbitMQ.RoutingKeys[0], bx)

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
	logrus.Infof("validating user id %s or email %s", req.UserName, req.Email)

	claims, err := as.jwt.ValidateToken(req.Token)
	if err != nil {
		logrus.Errorf("cannot validate the json token, error: %v", err)
		return &pb.ValidateResponse{
			StatusCode: http.StatusBadRequest,
			Error: &pb.Error{
				Status:  http.StatusUnauthorized,
				Error:   constants.ErrTokenIsNotValid.Error(),
				Message: constants.ErrTokenIsNotValid.Error(),
			},
		}, nil
	}
	// fmt.Printf("claims: %+v\n", claims)
	// Check if the email exists
	user, err := as.dbConn.CheckIfEmailExist(claims.Email)
	if err != nil {
		logrus.Errorf("cannot validate token as the email %s doesn't exist, error: %+v", claims.Email, err)
		return &pb.ValidateResponse{
			StatusCode: http.StatusNotFound,
			Error: &pb.Error{
				Status:  http.StatusUnauthorized,
				Error:   constants.ErrTokenIsNotValid.Error(),
				Message: constants.ErrTokenIsNotValid.Error(),
			},
		}, nil
	}

	return &pb.ValidateResponse{
		StatusCode: http.StatusOK,
		UserId:     user.Id,
		Email:      claims.Email,
		UserName:   user.Username,
	}, nil
}

func (as *AuthzSvc) Login(ctx context.Context, req *pb.LoginUserRequest) (*pb.LoginUserResponse, error) {
	logrus.Infof("user has requested to login with email: %s", req.Email)
	// Check if the user is existing the db or not
	user, err := as.dbConn.CheckIfEmailExist(req.Email)
	if err != nil {
		// TODO: Check not found and internal error
		return &pb.LoginUserResponse{
			StatusCode: http.StatusNotFound,
			Error: &pb.Error{
				Status:  http.StatusNotFound,
				Message: service_types.EmailPasswordWrong,
				Error:   service_types.ErrEmailPasswordWrong,
			},
		}, nil
	}

	// Check if the password match with the password hash
	if !utils.CheckPasswordHash(req.Password, user.Password) {
		return &pb.LoginUserResponse{
			StatusCode: http.StatusNotFound,
			Error: &pb.Error{
				Status:  http.StatusNotFound,
				Message: "Incorrect email or password has been given, try again",
				Error:   "email/password incorrect",
			},
		}, err
	}

	token, err := as.jwt.GenerateToken(user)
	if err != nil {
		logrus.Errorf(service_types.CannotCreateToken(req.Email, err))
		return &pb.LoginUserResponse{
			StatusCode: http.StatusBadRequest,
			Error: &pb.Error{
				Status:  http.StatusInternalServerError,
				Message: "Something went wrong",
				Error:   "Error creating login token",
			},
		}, nil
	}

	if req.IpAddress == "" {
		req.IpAddress = "127.0.0.1"
	}

	if req.Client == "" {
		req.Client = "Others"
	}
	user.IpAddress = req.IpAddress
	user.Client = req.Client

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
	logrus.Infof("User %s has forgotten their password", req.Email)

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
		logrus.Errorf("Error occurred while updating email verification token for %s, error: %v", req.Email, err)
		return nil, status.Errorf(codes.Internal, "Something went wrong while updating verification token")
	}

	emailBody := utils.ResetPasswordTemplate(user.FirstName, user.LastName, string(randomHash), user.Username)
	go func() {
		err := as.SendMail(req.Email, emailBody)
		if err != nil {
			logrus.Errorf("Failed to send mail for password recovery: %v", err)
		}
	}()

	if req.IpAddress == "" {
		req.IpAddress = "127.0.0.1"
	}

	if req.Client == "" {
		req.Client = "Others"
	}
	user.IpAddress = req.IpAddress
	user.Client = req.Client

	go cache.AddUserLog(as.dbConn, user, constants.ForgotPassword, constants.ServiceAuth, constants.EventForgotPassword, as.logger)

	return &pb.ForgotPasswordRes{
		StatusCode: 200,
		Message:    "Verification link has been sent to the email!",
	}, nil
}

func (as *AuthzSvc) ResetPassword(ctx context.Context, req *pb.ResetPasswordReq) (*pb.ResetPasswordRes, error) {
	logrus.Infof("user %s has requested to reset their password", req.Username)

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
		logrus.Errorf("timeout couldn't be verified: %v", err)
		return nil, status.Errorf(codes.Internal, "timeout couldn't be verified")
	}

	if timeTill.Before(time.Now()) {
		logrus.Errorf("the token has already expired, error: %+v", err)
		return nil, status.Errorf(codes.Unauthenticated, "token expired already")
	}

	// Verify reset token
	if ok := utils.CheckPasswordHash(req.Token, user.PasswordVerificationToken.String); !ok {
		logrus.Errorf("the token didn't match, error: %+v", err)
		return nil, status.Errorf(codes.Unauthenticated, "token didn't match")
	}

	logrus.Infof("Assigning a token to the user: %s having email: %s to reset their password", user.Username, user.Email)
	// Generate and return token
	token, err := as.jwt.GenerateToken(user)
	if err != nil {
		logrus.Errorf(service_types.CannotCreateToken(req.Email, err))
		return nil, status.Errorf(codes.Internal, "could not create token")
	}

	if req.IpAddress == "" {
		req.IpAddress = "127.0.0.1"
	}

	if req.Client == "" {
		req.Client = "Others"
	}
	user.IpAddress = req.IpAddress
	user.Client = req.Client

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
	logrus.Infof("updating password for: %+v", req)

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

	if req.IpAddress == "" {
		req.IpAddress = "127.0.0.1"
	}

	if req.Client == "" {
		req.Client = "Others"
	}
	user.IpAddress = req.IpAddress
	user.Client = req.Client

	go cache.AddUserLog(as.dbConn, user, constants.UpdatedPassword, constants.ServiceAuth, constants.EventUpdatedPassword, as.logger)

	return &pb.UpdatePasswordRes{
		StatusCode: http.StatusOK,
	}, nil
}

func (as *AuthzSvc) RequestForEmailVerification(ctx context.Context, req *pb.EmailVerificationReq) (*pb.EmailVerificationRes, error) {
	if req.Email == "" {
		return nil, constants.ErrBadRequest
	}
	logrus.Infof("user %v has requested for email verification", req.Email)

	user, err := as.dbConn.CheckIfEmailExist(req.Email)
	if err != nil {
		logrus.Infof("user %v is getting error", req.Email)
		return &pb.EmailVerificationRes{
			Error: &pb.Error{
				Status:  http.StatusNotFound,
				Message: service_types.EmailNotRegistered,
				Error:   service_types.ErrEmailNotRegistered,
			},
		}, nil
	}

	logrus.Infof("generating verification email token for: %s", req.GetEmail())
	hash := string(utils.GenHash())
	encHash := utils.HashPassword(hash)

	user.EmailVerificationToken = encHash
	user.EmailVerificationTimeout = sql.NullTime{
		Time:  time.Now().Add(time.Minute * 5),
		Valid: true, // Valid is true if Time is not NULL
	}

	if err := as.dbConn.UpdateEmailVerificationToken(user); err != nil {
		return nil, err
	}

	emailBody := utils.EmailVerificationHTML(user.Username, hash)
	logrus.Infof("Sending verification email to: %s", req.GetEmail())

	// TODO: Handle error of the go routine
	go func() {
		err := as.SendMail(user.Email, emailBody)
		if err != nil {
			// Handle error
			log.Printf("Failed to send mail for password recovery: %v", err)
		}
		logrus.Info("Email Sent!")
	}()

	if req.IpAddress == "" {
		req.IpAddress = "127.0.0.1"
	}

	if req.Client == "" {
		req.Client = "Others"
	}
	user.IpAddress = req.IpAddress
	user.Client = req.Client

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
	if req.IpAddress == "" {
		req.IpAddress = "127.0.0.1"
	}
	if req.Client == "" {
		req.Client = "Others"
	}
	user.IpAddress = req.IpAddress
	user.Client = req.Client

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

	go as.qConn.PublishDefaultProfilePhoto(as.config.RabbitMQ.Exchange, as.config.RabbitMQ.RoutingKeys[0], bx)

	if req.Ip == "" {
		req.Ip = "127.0.0.1"
	}

	if req.Client == "" {
		req.Client = "Others"
	}

	user.IpAddress = req.Ip
	user.Client = req.Client

	// Add a user log
	go cache.AddUserLog(as.dbConn, user, constants.UpdatedUserName, constants.ServiceAuth, constants.EventUpdateUsername, as.logger)

	token, err := as.jwt.GenerateToken(user)
	if err != nil {
		logrus.Errorf(service_types.CannotCreateToken(req.NewUsername, err))
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

	if req.IpAddress == "" {
		req.IpAddress = "127.0.0.1"
	}

	if req.Client == "" {
		req.Client = "Others"
	}

	user.IpAddress = req.IpAddress
	user.Client = req.Client

	// Add a user log
	go cache.AddUserLog(as.dbConn, user, constants.UpdatedPassword, constants.ServiceAuth, constants.EventUpdatedPassword, as.logger)

	// Return
	return &pb.UpdatePasswordWithPasswordRes{
		StatusCode: http.StatusOK,
	}, nil
}
