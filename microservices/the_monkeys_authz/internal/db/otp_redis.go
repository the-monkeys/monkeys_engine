package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/the-monkeys/the_monkeys/config"
	"go.uber.org/zap"
)

const (
	otpRegisterPrefix = "tm:otp:register:" // Key: "tm:otp:register:<email>"
	otpResetPrefix    = "tm:otp:reset:"    // Key: "tm:otp:reset:<email>"
	otpTTL            = 10 * time.Minute
	maxOTPAttempts    = 5
)

// PendingRegistration is the data stored in Redis while awaiting OTP verification.
type PendingRegistration struct {
	Email        string    `json:"email"`
	FirstName    string    `json:"first_name"`
	LastName     string    `json:"last_name"`
	PasswordHash string    `json:"password_hash"`
	OTPHash      string    `json:"otp_hash"`
	LoginMethod  string    `json:"login_method"`
	Attempts     int       `json:"attempts"`
	IpAddress    string    `json:"ip_address"`
	Client       string    `json:"client"`
	CreatedAt    time.Time `json:"created_at"`
}

// ResetOTPData is the data stored in Redis for password reset OTP.
type ResetOTPData struct {
	Email    string    `json:"email"`
	OTPHash  string    `json:"otp_hash"`
	Attempts int       `json:"attempts"`
	CreateAt time.Time `json:"created_at"`
}

// OTPRepository handles OTP storage in Redis.
type OTPRepository struct {
	client *redis.Client
	log    *zap.SugaredLogger
}

// NewOTPRepository creates a Redis-backed OTP store.
func NewOTPRepository(cfg *config.Config, log *zap.SugaredLogger) (*OTPRepository, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.Redis.Host,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		PoolSize:     cfg.Redis.PoolSize,
		MinIdleConns: cfg.Redis.MaxIdle,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := rdb.Ping(ctx).Result(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	log.Infof("authz OTP repository connected to Redis at %s", cfg.Redis.Host)
	return &OTPRepository{client: rdb, log: log}, nil
}

// --- Registration OTP ---

func (r *OTPRepository) registerKey(email string) string {
	return otpRegisterPrefix + email
}

// StorePendingRegistration stores a pending registration with automatic TTL expiry.
func (r *OTPRepository) StorePendingRegistration(ctx context.Context, reg *PendingRegistration) error {
	data, err := json.Marshal(reg)
	if err != nil {
		return fmt.Errorf("failed to marshal pending registration: %w", err)
	}
	return r.client.Set(ctx, r.registerKey(reg.Email), data, otpTTL).Err()
}

// GetPendingRegistration retrieves a pending registration. Returns nil if not found/expired.
func (r *OTPRepository) GetPendingRegistration(ctx context.Context, email string) (*PendingRegistration, error) {
	data, err := r.client.Get(ctx, r.registerKey(email)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get pending registration: %w", err)
	}

	var reg PendingRegistration
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal pending registration: %w", err)
	}
	return &reg, nil
}

// IncrementRegisterAttempts increments the failed attempt counter.
// If max attempts reached, deletes the key. Returns the new count.
func (r *OTPRepository) IncrementRegisterAttempts(ctx context.Context, email string) (int, error) {
	reg, err := r.GetPendingRegistration(ctx, email)
	if err != nil {
		return 0, err
	}
	if reg == nil {
		return 0, fmt.Errorf("no pending registration found")
	}

	reg.Attempts++
	if reg.Attempts >= maxOTPAttempts {
		_ = r.DeletePendingRegistration(ctx, email)
		return reg.Attempts, nil
	}

	// Re-store with updated attempts, preserving remaining TTL
	ttl, err := r.client.TTL(ctx, r.registerKey(email)).Result()
	if err != nil || ttl <= 0 {
		ttl = otpTTL
	}

	data, _ := json.Marshal(reg)
	if err := r.client.Set(ctx, r.registerKey(email), data, ttl).Err(); err != nil {
		return reg.Attempts, fmt.Errorf("failed to update attempts: %w", err)
	}
	return reg.Attempts, nil
}

// DeletePendingRegistration removes a pending registration.
func (r *OTPRepository) DeletePendingRegistration(ctx context.Context, email string) error {
	return r.client.Del(ctx, r.registerKey(email)).Err()
}

// --- Password Reset OTP ---

func (r *OTPRepository) resetKey(email string) string {
	return otpResetPrefix + email
}

// StoreResetOTP stores a password reset OTP with automatic TTL expiry.
func (r *OTPRepository) StoreResetOTP(ctx context.Context, data *ResetOTPData) error {
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal reset OTP data: %w", err)
	}
	return r.client.Set(ctx, r.resetKey(data.Email), b, otpTTL).Err()
}

// GetResetOTP retrieves a password reset OTP. Returns nil if not found/expired.
func (r *OTPRepository) GetResetOTP(ctx context.Context, email string) (*ResetOTPData, error) {
	b, err := r.client.Get(ctx, r.resetKey(email)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get reset OTP: %w", err)
	}

	var data ResetOTPData
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal reset OTP: %w", err)
	}
	return &data, nil
}

// IncrementResetAttempts increments the reset OTP failed attempt counter.
func (r *OTPRepository) IncrementResetAttempts(ctx context.Context, email string) (int, error) {
	data, err := r.GetResetOTP(ctx, email)
	if err != nil {
		return 0, err
	}
	if data == nil {
		return 0, fmt.Errorf("no reset OTP found")
	}

	data.Attempts++
	if data.Attempts >= maxOTPAttempts {
		_ = r.DeleteResetOTP(ctx, email)
		return data.Attempts, nil
	}

	ttl, err := r.client.TTL(ctx, r.resetKey(email)).Result()
	if err != nil || ttl <= 0 {
		ttl = otpTTL
	}

	b, _ := json.Marshal(data)
	if err := r.client.Set(ctx, r.resetKey(email), b, ttl).Err(); err != nil {
		return data.Attempts, fmt.Errorf("failed to update reset attempts: %w", err)
	}
	return data.Attempts, nil
}

// DeleteResetOTP removes a password reset OTP.
func (r *OTPRepository) DeleteResetOTP(ctx context.Context, email string) error {
	return r.client.Del(ctx, r.resetKey(email)).Err()
}
