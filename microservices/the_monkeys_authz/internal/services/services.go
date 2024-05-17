package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_authz/pb"
	"github.com/the-monkeys/the_monkeys/common"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/microservices/service_types"
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
	pb.UnimplementedAuthServiceServer
}

func NewAuthzSvc(dbCli db.AuthDBHandler, jwt utils.JwtWrapper, config *config.Config) *AuthzSvc {
	return &AuthzSvc{
		dbConn: dbCli,
		jwt:    jwt,
		config: config,
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
				Message: "Email FirstName LastName Password are not  entered",
				Error:   "Incomplete,information required ",
			},
		}, errors.New("Incomplete, information required")
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

	hash := string(utils.GenHash())
	encHash := utils.HashPassword(hash)

	// Create a userId and username
	user.AccountId = utils.RandomString(16)
	user.Username = utils.RandomString(12)
	user.FirstName = req.FirstName
	user.LastName = req.GetLastName()
	user.Email = req.GetEmail()
	user.Password = utils.HashPassword(req.Password)
	// user.CreateTime = time.Now().Format(common.DATE_TIME_FORMAT)
	// user.UpdateTime = time.Now().Format(common.DATE_TIME_FORMAT)
	user.UserStatus = "active"
	user.EmailVerificationToken = encHash
	user.EmailVerificationTimeout = sql.NullTime{
		Time:  time.Now().Add(time.Hour * 24),
		Valid: true,
	}
	if req.LoginMethod.String() == pb.RegisterUserRequest_LoginMethod_name[0] {
		user.LoginMethod = "the-monkeys"
	}

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
			// Handle error
			log.Printf("Failed to send mail post registration: %v", err)
		}
		logrus.Info("Email Sent!")
	}()

	logrus.Infof("user %s is successfully registered.", user.Email)

	// Generate and return token
	token, err := as.jwt.GenerateToken(user)
	if err != nil {
		logrus.Errorf(service_types.CannotCreateToken(req.Email, err))
		return &pb.RegisterUserResponse{
			StatusCode: http.StatusBadRequest,
			Error: &pb.Error{
				Status:  http.StatusInternalServerError,
				Message: "You are successfully registered",
				Error:   service_types.LoginMsg,
			},
		}, nil
	}

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
	logrus.Infof("got the request data: %+v", req)

	claims, err := as.jwt.ValidateToken(req.Token)
	if err != nil {
		logrus.Errorf("cannot validate the json token, error: %v", err)
		return &pb.ValidateResponse{
			StatusCode: http.StatusBadRequest,
			Error: &pb.Error{
				Status:  http.StatusUnauthorized,
				Error:   common.ErrTokenIsNotValid.Error(),
				Message: common.ErrTokenIsNotValid.Error(),
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
				Error:   common.ErrTokenIsNotValid.Error(),
				Message: common.ErrTokenIsNotValid.Error(),
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

	return &pb.LoginUserResponse{
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

func (as *AuthzSvc) ForgotPassword(ctx context.Context, req *pb.ForgotPasswordReq) (*pb.ForgotPasswordRes, error) {
	logrus.Infof("user %s has forgotten their password", req.Email)

	// Check if the user is existing the db or not
	user, err := as.dbConn.CheckIfEmailExist(req.Email)
	if err != nil {
		return &pb.ForgotPasswordRes{
			Error: &pb.Error{
				Status:  http.StatusNotFound,
				Message: service_types.IfEmailExists,
				Error:   service_types.ErrIfEmailExists,
			},
		}, err
	}

	var alphaNumRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890")
	randomHash := make([]rune, 64)
	for i := 0; i < 64; i++ {
		// Intn() returns, as an int, a non-negative pseudo-random number in [0,n).
		randomHash[i] = alphaNumRunes[rand.Intn(len(alphaNumRunes)-1)]
	}

	emailVerifyHash := utils.HashPassword(string(randomHash))

	if err = as.dbConn.UpdatePasswordRecoveryToken(emailVerifyHash, user); err != nil {
		logrus.Errorf("error occurred while updating email verification token for %s, error: %v", req.Email, err)
		return nil, err
	}

	emailBody := utils.ResetPasswordTemplate(user.FirstName, user.LastName, string(randomHash), user.Username)
	go func() {
		err := as.SendMail(req.Email, emailBody)
		if err != nil {
			// Handle error
			log.Printf("Failed to send mail for password recovery: %v", err)
		}
	}()

	return &pb.ForgotPasswordRes{
		StatusCode: 200,
		Message:    "Verification link has been sent to the email!",
	}, nil
}

func (as *AuthzSvc) ResetPassword(ctx context.Context, req *pb.ResetPasswordReq) (*pb.ResetPasswordRes, error) {
	logrus.Infof("user %s has requested to reset their password", req.Username)

	user, err := as.dbConn.CheckIfUsernameExist(req.Username)
	if err != nil {
		return &pb.ResetPasswordRes{
			Error: &pb.Error{
				Status:  http.StatusNotFound,
				Message: service_types.EmailNotRegistered,
				Error:   "An account is not registered with this email",
			},
		}, err
	}

	timeTill, err := time.Parse("2006-01-02 15:04:05.999999 -0700 +0000", user.PasswordVerificationTimeout.Time.String())
	if err != nil {
		logrus.Error(err)
		return nil, nil
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
		return &pb.ResetPasswordRes{
			Error: &pb.Error{
				Status:  http.StatusInternalServerError,
				Message: "You are successfully registered",
				Error:   service_types.LoginMsg,
			},
		}, nil
	}

	return &pb.ResetPasswordRes{
		StatusCode: http.StatusOK,
		Token:      token,
		// EmailVerified: false,
		// UserName:      user.Username,
		// Email:         user.Email,
		// UserId:        user.Id,
		// FirstName:     user.FirstName,
		// LastName:      user.LastName,
	}, nil
}

func (as *AuthzSvc) UpdatePassword(ctx context.Context, req *pb.UpdatePasswordReq) (*pb.UpdatePasswordRes, error) {
	logrus.Infof("updating password for: %+v", req)

	user, err := as.dbConn.CheckIfUsernameExist(req.Username)
	if err != nil {
		return &pb.UpdatePasswordRes{
			Error: &pb.Error{
				Status:  http.StatusNotFound,
				Message: service_types.EmailNotRegistered,
				Error:   "An account is not registered with this email",
			},
		}, err
	}

	encHash := utils.HashPassword(req.Password)
	if err := as.dbConn.UpdatePassword(encHash, &models.TheMonkeysUser{
		Id:       user.Id,
		Email:    req.Email,
		Username: req.Username,
	}); err != nil {
		return nil, err
	}
	logrus.Infof("updated password for: %+v", req.Email)
	return &pb.UpdatePasswordRes{
		StatusCode: http.StatusOK,
	}, nil
}

func (as *AuthzSvc) RequestForEmailVerification(ctx context.Context, req *pb.EmailVerificationReq) (*pb.EmailVerificationRes, error) {
	if req.Email == "" {
		return nil, common.ErrBadRequest
	}
	logrus.Infof("user %v has requested for email verification", req.Email)

	user, err := as.dbConn.CheckIfEmailExist(req.Email)
	// fmt.Printf("user: %+v\n", user)
	if err != nil {
		logrus.Infof("user %v is gettig error", req.Email)

		return &pb.EmailVerificationRes{
			Error: &pb.Error{
				Status:  http.StatusNotFound,
				Message: service_types.EmailNotRegistered,
				Error:   service_types.ErrEmailNotRegistered,
			},
		}, err
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

	return &pb.EmailVerificationRes{
		StatusCode: http.StatusOK,
	}, nil
}

func (as *AuthzSvc) VerifyEmail(ctx context.Context, req *pb.VerifyEmailReq) (*pb.VerifyEmailRes, error) {
	// Check if the username exists in the database
	user, err := as.dbConn.CheckIfUsernameExist(req.Username)
	if err != nil {
		if err == sql.ErrNoRows {
			return &pb.VerifyEmailRes{
				Error: &pb.Error{
					Status:  http.StatusNotFound,
					Message: service_types.EmailNotRegistered,
					Error:   service_types.ErrEmailNotRegistered,
				},
			}, nil
		}
		return nil, fmt.Errorf("failed to check username: %w", err)
	}

	// Parse the email verification timeout from the user
	timeTill, err := time.Parse(time.RFC3339, user.EmailVerificationTimeout.Time.Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("failed to parse email verification timeout: %w", err)
	}

	// Check if the email verification timeout has expired
	if timeTill.Before(time.Now()) {
		return nil, status.Errorf(codes.Unauthenticated, "token expired already")
	}

	// Verify reset token
	if ok := utils.CheckPasswordHash(req.Token, user.EmailVerificationToken); !ok {
		logrus.Errorf("the token didn't match, error: %+v", err)
		return nil, status.Errorf(codes.Unauthenticated, "token didn't match")
	}

	err = as.dbConn.UpdateEmailVerificationStatus(user)
	if err != nil {
		logrus.Errorf("cannot update the verification details for %s, error: %v", req.Email, err)
		return nil, err
	}

	logrus.Infof("verified email: %s", user.Email)

	// Return a success response with the status code 200
	return &pb.VerifyEmailRes{
		StatusCode: 200,
	}, nil
}

// psql -U root -d the_monkeys_user_dev
