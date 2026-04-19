package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_authz/pb"
	"github.com/the-monkeys/the_monkeys/constants"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_authz/internal/db"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_authz/internal/models"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_authz/internal/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	otpLength    = 6
	otpExpiryMin = 10
	cooldownSecs = 30
	maxAttempts  = 5
)

// InitiateRegistration validates input, generates OTP, stores in Redis, and sends email.
// No user account is created until OTP is verified.
func (as *AuthzSvc) InitiateRegistration(ctx context.Context, req *pb.InitiateRegistrationReq) (*pb.InitiateRegistrationRes, error) {
	as.logger.Debugf("initiate registration for: %s", req.Email)

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)

	if req.Email == "" || req.FirstName == "" || req.Password == "" {
		return nil, status.Errorf(codes.InvalidArgument, "email, first name, and password are required")
	}

	if err := utils.ValidateEmailFormat(req.Email); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid email address: %v", err)
	}

	if utils.IsDisposableEmail(req.Email) {
		return nil, status.Errorf(codes.InvalidArgument, "disposable email addresses are not allowed")
	}

	// Check if user already exists in the database
	if _, err := as.dbConn.CheckIfEmailExist(req.Email); err == nil {
		return nil, status.Errorf(codes.AlreadyExists, "user with this email already exists")
	}

	// Generate OTP
	otpCode, err := utils.GenerateOTP(otpLength)
	if err != nil {
		as.logger.Errorf("failed to generate OTP for %s: %v", req.Email, err)
		return nil, status.Errorf(codes.Internal, "failed to generate verification code")
	}

	// Extract client info
	clientInfo := as.extractClientInfo(req)
	ip, client := utils.IpClientConvert(clientInfo.IPAddress, clientInfo.Client)

	// Store in Redis with 10-min TTL — Redis auto-expires, no cleanup needed
	pending := &db.PendingRegistration{
		Email:        req.Email,
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		PasswordHash: utils.HashPassword(req.Password),
		OTPHash:      utils.HashPassword(otpCode),
		LoginMethod:  "the-monkeys",
		Attempts:     0,
		IpAddress:    ip,
		Client:       client,
		CreatedAt:    time.Now(),
	}

	if err := as.otpRepo.StorePendingRegistration(ctx, pending); err != nil {
		as.logger.Errorf("failed to store pending registration for %s: %v", req.Email, err)
		return nil, status.Errorf(codes.Internal, "failed to initiate registration")
	}

	// Send OTP email
	emailBody := utils.RegistrationOTPEmailHTML(req.FirstName, req.LastName, otpCode)
	go func() {
		if err := as.SendMail(req.Email, emailBody); err != nil {
			as.logger.Errorf("failed to send OTP email to %s: %v", req.Email, err)
		}
	}()

	return &pb.InitiateRegistrationRes{
		StatusCode: http.StatusOK,
		Message:    "Verification code sent to your email",
		ExpiresIn:  otpExpiryMin * 60,
	}, nil
}

// VerifyRegistrationOTP verifies the OTP and creates the user account.
func (as *AuthzSvc) VerifyRegistrationOTP(ctx context.Context, req *pb.VerifyRegistrationOTPReq) (*pb.VerifyRegistrationOTPRes, error) {
	as.logger.Debugf("verify registration OTP for: %s", req.Email)

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	// Get pending registration from Redis
	pending, err := as.otpRepo.GetPendingRegistration(ctx, req.Email)
	if err != nil {
		as.logger.Errorf("failed to get pending registration for %s: %v", req.Email, err)
		return nil, status.Errorf(codes.Internal, "failed to verify OTP")
	}
	if pending == nil {
		// Key expired or never existed
		return nil, status.Errorf(codes.NotFound, "no pending registration found, please register again")
	}

	// Verify OTP (bcrypt timing-safe)
	if !utils.CheckPasswordHash(req.OtpCode, pending.OTPHash) {
		attempts, incErr := as.otpRepo.IncrementRegisterAttempts(ctx, req.Email)
		if incErr != nil {
			as.logger.Errorf("failed to increment OTP attempts for %s: %v", req.Email, incErr)
		}
		if attempts >= maxAttempts {
			return nil, status.Errorf(codes.ResourceExhausted, "too many failed attempts, please register again")
		}
		return nil, status.Errorf(codes.InvalidArgument, "invalid verification code")
	}

	// OTP correct — race condition guard: re-check email uniqueness
	if _, err := as.dbConn.CheckIfEmailExist(req.Email); err == nil {
		_ = as.otpRepo.DeletePendingRegistration(ctx, req.Email)
		return nil, status.Errorf(codes.AlreadyExists, "user with this email already exists")
	}

	// Create user account
	user := &models.TheMonkeysUser{
		AccountId:              utils.GenerateGUID(),
		Username:               utils.GenerateGUID(),
		FirstName:              pending.FirstName,
		LastName:               pending.LastName,
		Email:                  pending.Email,
		Password:               pending.PasswordHash,
		UserStatus:             "active",
		EmailVerificationToken: utils.HashPassword(string(utils.GenHash())),
		EmailVerificationTimeout: sql.NullTime{
			Time:  time.Now().Add(time.Hour),
			Valid: true,
		},
		LoginMethod: pending.LoginMethod,
		IpAddress:   pending.IpAddress,
		Client:      pending.Client,
	}

	userId, err := as.dbConn.RegisterUser(user)
	if err != nil {
		as.logger.Errorf("failed to register user %s after OTP verification: %v", pending.Email, err)
		return nil, status.Errorf(codes.Internal, "failed to create account")
	}

	// Clean up Redis
	_ = as.otpRepo.DeletePendingRegistration(ctx, req.Email)

	// Mark email verified (OTP proves ownership)
	if err := as.dbConn.UpdateEmailVerificationStatus(user); err != nil {
		as.logger.Errorf("failed to update email verification for %s: %v", user.Email, err)
	}

	// Track activity
	as.trackAuthActivity(user, "register", req)

	// Generate JWT
	token, refreshToken, err := as.jwt.GenerateToken(user)
	if err != nil {
		as.logger.Errorf("failed to generate token for %s: %v", user.Email, err)
		return nil, status.Errorf(codes.Aborted, "account created successfully, please log in")
	}

	// Publish RabbitMQ — identical payload and routing keys as existing RegisterUser
	msg, err := json.Marshal(models.TheMonkeysMessage{
		Username:     user.Username,
		AccountId:    user.AccountId,
		FirstName:    user.FirstName,
		LastName:     user.LastName,
		Email:        user.Email,
		Action:       constants.USER_REGISTER,
		Notification: constants.NotificationRegister,
	})
	if err != nil {
		as.logger.Errorf("failed to marshal register message: %v", err)
	}

	go func() {
		if pubErr := as.qConn.PublishMessage(as.config.RabbitMQ.Exchange, as.config.RabbitMQ.RoutingKeys[0], msg); pubErr != nil {
			as.logger.Errorf("failed to publish user register message: %v", pubErr)
		}
		if pubErr := as.qConn.PublishMessage(as.config.RabbitMQ.Exchange, as.config.RabbitMQ.RoutingKeys[4], msg); pubErr != nil {
			as.logger.Errorf("failed to publish notification: %v", pubErr)
		}
	}()

	as.logger.Infof("user %s registered via OTP", user.Email)

	return &pb.VerifyRegistrationOTPRes{
		StatusCode:              http.StatusCreated,
		Token:                   token,
		RefreshToken:            refreshToken,
		EmailVerified:           true,
		Username:                user.Username,
		Email:                   user.Email,
		UserId:                  userId,
		FirstName:               user.FirstName,
		LastName:                user.LastName,
		AccountId:               user.AccountId,
		EmailVerificationStatus: constants.EmailVerificationStatusVerified,
	}, nil
}

// ResendRegistrationOTP regenerates and resends the OTP. 30-second cooldown.
func (as *AuthzSvc) ResendRegistrationOTP(ctx context.Context, req *pb.ResendRegistrationOTPReq) (*pb.ResendRegistrationOTPRes, error) {
	as.logger.Debugf("resend registration OTP for: %s", req.Email)

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	pending, err := as.otpRepo.GetPendingRegistration(ctx, req.Email)
	if err != nil {
		as.logger.Errorf("failed to get pending registration for %s: %v", req.Email, err)
		return nil, status.Errorf(codes.Internal, "failed to resend verification code")
	}
	if pending == nil {
		return nil, status.Errorf(codes.NotFound, "no pending registration found, please register again")
	}

	// Rate limit: 30-second cooldown
	if time.Since(pending.CreatedAt) < cooldownSecs*time.Second {
		return nil, status.Errorf(codes.ResourceExhausted, "please wait before requesting a new code")
	}

	// Generate new OTP, reset attempts
	otpCode, err := utils.GenerateOTP(otpLength)
	if err != nil {
		as.logger.Errorf("failed to generate OTP for %s: %v", req.Email, err)
		return nil, status.Errorf(codes.Internal, "failed to generate verification code")
	}

	pending.OTPHash = utils.HashPassword(otpCode)
	pending.Attempts = 0
	pending.CreatedAt = time.Now()

	if err := as.otpRepo.StorePendingRegistration(ctx, pending); err != nil {
		as.logger.Errorf("failed to update pending registration for %s: %v", req.Email, err)
		return nil, status.Errorf(codes.Internal, "failed to resend verification code")
	}

	emailBody := utils.RegistrationOTPEmailHTML(pending.FirstName, pending.LastName, otpCode)
	go func() {
		if err := as.SendMail(req.Email, emailBody); err != nil {
			as.logger.Errorf("failed to resend OTP email to %s: %v", req.Email, err)
		}
	}()

	return &pb.ResendRegistrationOTPRes{
		StatusCode: http.StatusOK,
		Message:    "Verification code resent to your email",
		ExpiresIn:  otpExpiryMin * 60,
	}, nil
}

// VerifyResetOTP verifies the password reset OTP and returns a short-lived JWT.
func (as *AuthzSvc) VerifyResetOTP(ctx context.Context, req *pb.VerifyResetOTPReq) (*pb.VerifyResetOTPRes, error) {
	as.logger.Debugf("verify reset OTP for: %s", req.Email)

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	// Get reset OTP data from Redis
	resetData, err := as.otpRepo.GetResetOTP(ctx, req.Email)
	if err != nil {
		as.logger.Errorf("error getting reset OTP for %s: %v", req.Email, err)
		return nil, status.Errorf(codes.Internal, "failed to verify OTP")
	}
	if resetData == nil {
		return nil, status.Errorf(codes.DeadlineExceeded, "verification code has expired")
	}

	// Verify OTP hash
	if !utils.CheckPasswordHash(req.OtpCode, resetData.OTPHash) {
		attempts, incErr := as.otpRepo.IncrementResetAttempts(ctx, req.Email)
		if incErr != nil {
			as.logger.Errorf("failed to increment reset OTP attempts for %s: %v", req.Email, incErr)
		}
		if attempts >= maxAttempts {
			return nil, status.Errorf(codes.ResourceExhausted, "too many failed attempts, please request a new code")
		}
		return nil, status.Errorf(codes.InvalidArgument, "invalid verification code")
	}

	// OTP correct — look up user for JWT generation
	user, err := as.dbConn.CheckIfEmailExist(req.Email)
	if err != nil {
		as.logger.Errorf("user not found after OTP verification for %s: %v", req.Email, err)
		return nil, status.Errorf(codes.NotFound, "user not found")
	}

	// Generate short-lived password reset token (5 min)
	resetToken, err := as.jwt.GeneratePasswordResetToken(user)
	if err != nil {
		as.logger.Errorf("failed to generate password reset token for %s: %v", req.Email, err)
		return nil, status.Errorf(codes.Internal, "failed to verify OTP")
	}

	// Delete the reset OTP (single-use)
	_ = as.otpRepo.DeleteResetOTP(ctx, req.Email)

	// Track activity
	clientInfo := as.extractClientInfo(req)
	user.IpAddress, user.Client = utils.IpClientConvert(clientInfo.IPAddress, clientInfo.Client)
	as.trackAuthActivity(user, "verify_reset_otp", req)

	return &pb.VerifyResetOTPRes{
		StatusCode: http.StatusOK,
		Token:      resetToken,
	}, nil
}
