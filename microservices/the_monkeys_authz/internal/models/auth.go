package models

import (
	"database/sql"
)

// type TheMonkeysUser struct {
// 	Id                        int64     `json:"id" gorm:"primaryKey"`
// 	UUID                      string    `json:"unique_id"`
// 	FirstName                 string    `json:"first_name"`
// 	LastName                  string    `json:"last_name"`
// 	Email                     string    `json:"email"`
// 	Password                  string    `json:"password"`
// 	CreateTime                string    `json:"create_time,omitempty"`
// 	UpdateTime                string    `json:"update_time,omitempty"`
// 	IsActive                  bool      `json:"is_active,omitempty"`
// 	Role                      int32     `json:"role,omitempty"`
// 	EmailVerificationToken    string    `json:"email_verification_token"`
// 	EmailVerificationTimeout  time.Time `json:"email_verification_timeout"`
// 	MobileVerificationToken   string    `json:"mobile_verification_token"`
// 	MobileVerificationTimeout time.Time `json:"mobile_verification_timeout"`
// 	Deactivated               bool      `json:"deactivated"`
// 	LoginMethod               string    `json:"login_method"`
// 	EmailVerified             bool      `json:"email_verified"`
// }

// type PasswordReset struct {
// 	Id                int64     `json:"id" gorm:"primaryKey"`
// 	Email             string    `json:"email"`
// 	RecoveryHash      string    `json:"recovery_hash"`
// 	TimeOut           time.Time `json:"time_out"`
// 	LastPasswordReset time.Time `json:"last_pass_reset"`
// }

// ----------------------------------------------------NEW Monkeys----------------------------------------
type TheMonkeysUser struct {
	Id                          int64          `json:"id"`
	AccountId                   string         `json:"profile_id"`
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
	ClientId                    string         `json:"client_id"`
	Client                      string         `json:"client"`
	IpAddress                   string         `json:"ip"`
}

type TheMonkeysAccount struct {
	Id        int64  `json:"id"`
	AccountId string `json:"profile_id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
}

type TheMonkeysMessage struct {
	Id          int64  `json:"id"`
	AccountId   string `json:"profile_id"`
	Username    string `json:"username"`
	NewUsername string `json:"new_username"`
	Email       string `json:"email"`
	LoginMethod string `json:"login_method"`
	ClientId    string `json:"client_id"`
	Client      string `json:"client"`
	IpAddress   string `json:"ip"`
	Action      string `json:"action"`
}
