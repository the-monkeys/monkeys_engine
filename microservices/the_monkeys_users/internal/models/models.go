package models

import (
	"database/sql"
)

// TODO: Change as per schema user_account table
type UserAccount struct {
	Id            int64          `json:"id"`
	AccountId     string         `json:"account_id"`
	UserName      string         `json:"username"`
	FirstName     string         `json:"firstname"`
	LastName      string         `json:"lastname"`
	Email         string         `json:"email"`
	DateOfBirth   sql.NullTime   `json:"date_of_birth"`
	Bio           sql.NullString `json:"bio"`
	AvatarUrl     sql.NullString `json:"avatar_url"`
	Address       sql.NullString `json:"address"`
	ContactNumber sql.NullInt64  `json:"contact_number"`
	UserStatus    string         `json:"user_status"`
	CreatedAt     sql.NullTime   `json:"created_at,omitempty"`
	LinkedIn      sql.NullString `json:"linkedIn"`
	Github        sql.NullString `json:"github"`
	Twitter       sql.NullString `json:"twitter"`
	Instagram     sql.NullString `json:"instagram"`
}

type UserAuthInfo struct {
	Id                       int64          `json:"id"`
	UserId                   int64          `json:"user_id"`
	PasswordHash             sql.NullString `json:"password_hash"`
	CreatedAt                sql.NullTime   `json:"created_at"`
	PasswordRecoveryToken    sql.NullString `json:"password_recovery_token"`
	PasswordRecoveryTimeout  sql.NullTime   `json:"password_recovery_timeout"`
	PasswordUpdatedAt        sql.NullTime   `json:"password_updated_at"`
	EmailVerificationToken   string         `json:"email_verification_token"`
	EmailVerificationTimeout sql.NullTime   `json:"email_verification_timeout"`
	EmailValidationStatus    string         `json:"email_validation_satus"`
	EmailValidationTime      sql.NullTime   `json:"email_validation_time"`
	AuthProviderId           int64          `json:"auth_provider_id"`
}

type TheMonkeysUser struct {
	Id                          int64          `json:"id"`
	AccountId                   string         `json:"account_id,omitempty"`
	Username                    string         `json:"username"`
	FirstName                   string         `json:"first_name"`
	LastName                    string         `json:"last_name"`
	Email                       string         `json:"email"`
	Password                    string         `json:"password"`
	PasswordVerificationToken   sql.NullString `json:"password_verification_token"`
	PasswordVerificationTimeout sql.NullTime   `json:"password_verification_timeout"`
	EmailVerificationStatus     string         `json:"email_verified"`
	UserStatus                  string         `json:"is_active,omitempty"`
	EmailVerificationToken      string         `json:"email_verification_token"`
	EmailVerificationTimeout    sql.NullTime   `json:"email_verification_timeout"`
	MobileVerificationToken     string         `json:"mobile_verification_token"`
	MobileVerificationTimeout   sql.NullTime   `json:"mobile_verification_timeout"`
	LoginMethod                 string         `json:"login_method"`
}

type UserProfileRes struct {
	AccountId      string         `json:"account_id,omitempty"`
	Username       string         `json:"username,omitempty"`
	FirstName      string         `json:"first_name,omitempty"`
	LastName       string         `json:"last_name,omitempty"`
	Email          string         `json:"email,omitempty"`
	DateOfBirth    sql.NullTime   `json:"date_of_birth,omitempty"`
	RoleId         int64          `json:"role_id,omitempty"`
	Bio            sql.NullString `json:"bio,omitempty"`
	AvatarUrl      sql.NullString `json:"avatar_url,omitempty"`
	CreatedAt      sql.NullTime   `json:"created_at,omitempty"`
	UpdatedAt      sql.NullTime   `json:"updated_at,omitempty"`
	Address        sql.NullString `json:"address,omitempty"`
	ContactNumber  sql.NullString `json:"contact_number,omitempty"`
	UserStatus     string         `json:"user_status,omitempty"`
	ViewPermission string         `json:"view_permission,omitempty"`
	LinkedIn       sql.NullString `json:"linkedIn"`
	Github         sql.NullString `json:"github"`
	Twitter        sql.NullString `json:"twitter"`
	Instagram      sql.NullString `json:"instagram"`
}

type UserLogs struct {
	AccountId string `json:"account_id,omitempty"`
	Client    string `json:"client,omitempty"`
	IpAddress string `json:"ip_address,omitempty"`
}
