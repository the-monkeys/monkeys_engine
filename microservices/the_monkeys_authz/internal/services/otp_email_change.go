package services

import (
	"context"
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

// InitiateEmailChange validates the new email, generates OTP, stores in Redis, and sends OTP to the NEW email.
// The email is NOT changed until the OTP is verified.
func (as *AuthzSvc) InitiateEmailChange(ctx context.Context, req *pb.InitiateEmailChangeReq) (*pb.InitiateEmailChangeRes, error) {
	as.logger.Debugf("initiate email change for user: %s, new email: %s", req.Username, req.NewEmail)

	req.NewEmail = strings.ToLower(strings.TrimSpace(req.NewEmail))
	req.Username = strings.TrimSpace(req.Username)

	if req.Username == "" || req.NewEmail == "" {
		return nil, status.Errorf(codes.InvalidArgument, "username and new email are required")
	}

	// Validate the new email format
	if err := utils.ValidateEmailFormat(req.NewEmail); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid email address: %v", err)
	}

	// Block disposable emails
	if utils.IsDisposableEmail(req.NewEmail) {
		return nil, status.Errorf(codes.InvalidArgument, "disposable email addresses are not allowed")
	}

	// Verify the user exists
	user, err := as.dbConn.CheckIfUsernameExist(req.Username)
	if err != nil {
		as.logger.Errorf("user %s not found: %v", req.Username, err)
		return nil, status.Errorf(codes.NotFound, "user not found")
	}

	// Reject if the new email is the same as current
	if strings.EqualFold(user.Email, req.NewEmail) {
		return nil, status.Errorf(codes.InvalidArgument, "new email is the same as current email")
	}

	// Check if the new email is already in use
	if _, err := as.dbConn.CheckIfEmailExist(req.NewEmail); err == nil {
		return nil, status.Errorf(codes.AlreadyExists, "this email is already in use")
	}

	// Generate OTP
	otpCode, err := utils.GenerateOTP(otpLength)
	if err != nil {
		as.logger.Errorf("failed to generate OTP for email change %s: %v", req.NewEmail, err)
		return nil, status.Errorf(codes.Internal, "failed to generate verification code")
	}

	// Store in Redis with 10-min TTL
	data := &db.EmailChangeOTPData{
		Username: req.Username,
		OldEmail: user.Email,
		NewEmail: req.NewEmail,
		OTPHash:  utils.HashPassword(otpCode),
		Attempts: 0,
		CreateAt: time.Now(),
	}

	if err := as.otpRepo.StoreEmailChangeOTP(ctx, data); err != nil {
		as.logger.Errorf("failed to store email change OTP for %s: %v", req.NewEmail, err)
		return nil, status.Errorf(codes.Internal, "failed to initiate email change")
	}

	// Send OTP to the NEW email to verify ownership
	emailBody := utils.EmailChangeOTPEmailHTML(user.FirstName, user.LastName, otpCode)
	go func() {
		if err := as.SendMail(req.NewEmail, emailBody); err != nil {
			as.logger.Errorf("failed to send email change OTP to %s: %v", req.NewEmail, err)
		}
	}()

	// Track activity
	clientInfo := as.extractClientInfo(req)
	user.IpAddress, user.Client = utils.IpClientConvert(clientInfo.IPAddress, clientInfo.Client)
	as.trackAuthActivity(user, "initiate_email_change", req)

	return &pb.InitiateEmailChangeRes{
		StatusCode: http.StatusOK,
		Message:    "Verification code sent to your new email address",
		ExpiresIn:  otpExpiryMin * 60,
	}, nil
}

// VerifyEmailChangeOTP verifies the OTP, updates the email, and marks it as verified.
func (as *AuthzSvc) VerifyEmailChangeOTP(ctx context.Context, req *pb.VerifyEmailChangeOTPReq) (*pb.VerifyEmailChangeOTPRes, error) {
	as.logger.Debugf("verify email change OTP for user: %s, new email: %s", req.Username, req.NewEmail)

	req.NewEmail = strings.ToLower(strings.TrimSpace(req.NewEmail))
	req.Username = strings.TrimSpace(req.Username)

	// Get email change OTP data from Redis
	data, err := as.otpRepo.GetEmailChangeOTP(ctx, req.NewEmail)
	if err != nil {
		as.logger.Errorf("error getting email change OTP for %s: %v", req.NewEmail, err)
		return nil, status.Errorf(codes.Internal, "failed to verify OTP")
	}
	if data == nil {
		return nil, status.Errorf(codes.DeadlineExceeded, "verification code has expired, please request a new one")
	}

	// Security: verify that the username matches who initiated the request
	if data.Username != req.Username {
		as.logger.Errorf("username mismatch for email change OTP: expected %s, got %s", data.Username, req.Username)
		return nil, status.Errorf(codes.PermissionDenied, "unauthorized email change attempt")
	}

	// Verify OTP (bcrypt timing-safe)
	if !utils.CheckPasswordHash(req.OtpCode, data.OTPHash) {
		attempts, incErr := as.otpRepo.IncrementEmailChangeAttempts(ctx, req.NewEmail)
		if incErr != nil {
			as.logger.Errorf("failed to increment email change OTP attempts for %s: %v", req.NewEmail, incErr)
		}
		if attempts >= maxAttempts {
			return nil, status.Errorf(codes.ResourceExhausted, "too many failed attempts, please request a new code")
		}
		return nil, status.Errorf(codes.InvalidArgument, "invalid verification code")
	}

	// OTP correct — re-check email uniqueness (race condition guard)
	if _, err := as.dbConn.CheckIfEmailExist(req.NewEmail); err == nil {
		_ = as.otpRepo.DeleteEmailChangeOTP(ctx, req.NewEmail)
		return nil, status.Errorf(codes.AlreadyExists, "this email is already in use")
	}

	// Get the current user for DB update
	user, err := as.dbConn.CheckIfUsernameExist(req.Username)
	if err != nil {
		as.logger.Errorf("user %s not found during email change verification: %v", req.Username, err)
		return nil, status.Errorf(codes.NotFound, "user not found")
	}

	// Update email AND mark as verified in a single transaction
	if err := as.dbConn.UpdateEmailIdAndMarkVerified(req.NewEmail, user); err != nil {
		as.logger.Errorf("failed to update email for user %s: %v", req.Username, err)
		return nil, status.Errorf(codes.Internal, "failed to update email")
	}

	// Clean up Redis
	_ = as.otpRepo.DeleteEmailChangeOTP(ctx, req.NewEmail)

	// Track activity
	clientInfo := as.extractClientInfo(req)
	user.IpAddress, user.Client = utils.IpClientConvert(clientInfo.IPAddress, clientInfo.Client)
	as.trackAuthActivity(user, "verify_email_change", req)

	// Send notification emails to BOTH old and new addresses.
	// Old email: alert that the address was changed (compromised-account visibility).
	// New email: confirm it is now linked to the account.
	go func() {
		oldBody := utils.EmailChangedNotifyOldEmailHTML(user.FirstName, user.LastName, req.NewEmail)
		if err := as.SendMail(data.OldEmail, oldBody); err != nil {
			as.logger.Errorf("failed to send email-changed alert to old address %s: %v", data.OldEmail, err)
		}
	}()
	go func() {
		newBody := utils.EmailChangedConfirmNewEmailHTML(user.FirstName, user.LastName)
		if err := as.SendMail(req.NewEmail, newBody); err != nil {
			as.logger.Errorf("failed to send email-changed confirmation to new address %s: %v", req.NewEmail, err)
		}
	}()

	// Publish email changed notification
	user.Email = req.NewEmail
	notifMsg, err := json.Marshal(models.TheMonkeysMessage{
		Username:  user.Username,
		AccountId: user.AccountId,
		Email:     req.NewEmail,
		Action:    constants.EMAIL_CHANGED,
	})
	if err != nil {
		as.logger.Errorf("failed to marshal email changed notification: %v", err)
	} else {
		go func() {
			if err := as.qConn.PublishMessage(as.config.RabbitMQ.Exchange, as.config.RabbitMQ.RoutingKeys[4], notifMsg); err != nil {
				as.logger.Errorf("failed to publish email changed notification: %v", err)
			}
		}()
	}

	// Generate new JWT with updated email
	token, _, err := as.jwt.GenerateToken(user)
	if err != nil {
		as.logger.Errorf("failed to generate token after email change for %s: %v", user.Username, err)
		return nil, status.Errorf(codes.Internal, "email updated but failed to generate new session")
	}

	as.logger.Infof("email changed for user %s: %s -> %s", user.Username, data.OldEmail, req.NewEmail)

	return &pb.VerifyEmailChangeOTPRes{
		StatusCode:    http.StatusOK,
		Token:         token,
		EmailVerified: true,
		UserId:        user.Id,
		UserName:      user.Username,
		FirstName:     user.FirstName,
		LastName:      user.LastName,
		Email:         req.NewEmail,
		AccountId:     user.AccountId,
	}, nil
}

// ResendEmailChangeOTP regenerates and resends the email change OTP. 30-second cooldown.
func (as *AuthzSvc) ResendEmailChangeOTP(ctx context.Context, req *pb.ResendEmailChangeOTPReq) (*pb.ResendEmailChangeOTPRes, error) {
	as.logger.Debugf("resend email change OTP for user: %s, new email: %s", req.Username, req.NewEmail)

	req.NewEmail = strings.ToLower(strings.TrimSpace(req.NewEmail))

	data, err := as.otpRepo.GetEmailChangeOTP(ctx, req.NewEmail)
	if err != nil {
		as.logger.Errorf("failed to get email change OTP for %s: %v", req.NewEmail, err)
		return nil, status.Errorf(codes.Internal, "failed to resend verification code")
	}
	if data == nil {
		return nil, status.Errorf(codes.NotFound, "no pending email change found, please initiate again")
	}

	// Security: verify username
	if data.Username != req.Username {
		return nil, status.Errorf(codes.PermissionDenied, "unauthorized")
	}

	// Rate limit: 30-second cooldown
	if time.Since(data.CreateAt) < cooldownSecs*time.Second {
		return nil, status.Errorf(codes.ResourceExhausted, "please wait before requesting a new code")
	}

	// Generate new OTP, reset attempts
	otpCode, err := utils.GenerateOTP(otpLength)
	if err != nil {
		as.logger.Errorf("failed to generate OTP for email change %s: %v", req.NewEmail, err)
		return nil, status.Errorf(codes.Internal, "failed to generate verification code")
	}

	data.OTPHash = utils.HashPassword(otpCode)
	data.Attempts = 0
	data.CreateAt = time.Now()

	if err := as.otpRepo.StoreEmailChangeOTP(ctx, data); err != nil {
		as.logger.Errorf("failed to update email change OTP for %s: %v", req.NewEmail, err)
		return nil, status.Errorf(codes.Internal, "failed to resend verification code")
	}

	// Get user for email template
	user, _ := as.dbConn.CheckIfUsernameExist(req.Username)
	firstName, lastName := "", ""
	if user != nil {
		firstName = user.FirstName
		lastName = user.LastName
	}

	emailBody := utils.EmailChangeOTPEmailHTML(firstName, lastName, otpCode)
	go func() {
		if err := as.SendMail(req.NewEmail, emailBody); err != nil {
			as.logger.Errorf("failed to resend email change OTP to %s: %v", req.NewEmail, err)
		}
	}()

	return &pb.ResendEmailChangeOTPRes{
		StatusCode: http.StatusOK,
		Message:    "Verification code resent to your new email address",
		ExpiresIn:  otpExpiryMin * 60,
	}, nil
}
