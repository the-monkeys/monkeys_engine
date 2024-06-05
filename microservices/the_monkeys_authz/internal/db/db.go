package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/constants"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_authz/internal/models"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AuthDBHandler interface {
	// Get Operations
	CheckIfEmailExist(email string) (*models.TheMonkeysUser, error)
	CheckIfUsernameExist(username string) (*models.TheMonkeysUser, error)

	// Create Operations
	RegisterUser(user *models.TheMonkeysUser) (int64, error)

	// Update Operations
	UpdatePasswordRecoveryToken(hash string, req *models.TheMonkeysUser) error
	UpdatePassword(password string, user *models.TheMonkeysUser) error
	UpdateEmailVerificationToken(req *models.TheMonkeysUser) error
	UpdateEmailVerificationStatus(req *models.TheMonkeysUser) error

	// Create user logs to track activity
	CreateUserLog(user *models.TheMonkeysUser, description string) error
}
type authDBHandler struct {
	db *sql.DB
}

// NewAuthDBHandler creates a new instance of AuthDBHandler
func NewAuthDBHandler(cfg *config.Config) (AuthDBHandler, error) {
	url := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.Postgresql.PrimaryDB.DBUsername,
		cfg.Postgresql.PrimaryDB.DBPassword,
		cfg.Postgresql.PrimaryDB.DBHost,
		cfg.Postgresql.PrimaryDB.DBPort,
		cfg.Postgresql.PrimaryDB.DBName,
	)

	dbPsql, err := sql.Open("postgres", url)
	if err != nil {
		logrus.Fatalf("cannot connect psql using sql driver, error:, %+v", err)
		return nil, err
	}

	if err = dbPsql.Ping(); err != nil {
		logrus.Errorf("ping test failed to psql using sql driver, error: %+v", err)
		return nil, err

	}

	return &authDBHandler{db: dbPsql}, nil
}

// TODO: Find all the fields of models.TheMonkeysUser
func (adh *authDBHandler) CheckIfEmailExist(email string) (*models.TheMonkeysUser, error) {
	var tmu models.TheMonkeysUser
	if err := adh.db.QueryRow(`
            SELECT ua.id, ua.account_id, ua.username, ua.first_name, ua.last_name, 
            ua.email, uai.password_hash, evs.status, us.status, uai.email_validation_token,
            uai.email_verification_timeout
            FROM USER_ACCOUNT ua
            LEFT JOIN user_auth_info uai ON ua.id = uai.user_id
            LEFT JOIN email_validation_status evs ON uai.email_validation_status = evs.id
            LEFT JOIN user_status us ON ua.user_status = us.id
            WHERE ua.email = $1;
        `, email).
		Scan(&tmu.Id, &tmu.AccountId, &tmu.Username, &tmu.FirstName, &tmu.LastName, &tmu.Email, &tmu.Password,
			&tmu.EmailVerificationStatus, &tmu.UserStatus, &tmu.EmailVerificationToken, &tmu.EmailVerificationTimeout); err != nil {
		logrus.Errorf("can't find a user existing with email %s, error: %+v", email, err)
		return nil, err
	}

	return &tmu, nil
}

func (adh *authDBHandler) RegisterUser(user *models.TheMonkeysUser) (int64, error) {
	tx, err := adh.db.Begin()
	if err != nil {
		return 0, err
	}

	userId, err := adh.insertIntoUserAccount(tx, user)
	if err != nil {
		return 0, err
	}

	authId, err := adh.insertIntoUserAuthInfo(tx, user, userId)
	if err != nil {
		return 0, err
	}

	// USER_ACCOUNT_STATUS

	// EXTERNAL_AUTH_PROVIDERS

	err = tx.Commit()
	if err != nil {
		return 0, err
	}

	if userId != authId {
		logrus.Warnf("we are detecting some data inconsistency for user %s", user.Email)
	}

	return userId, nil
}

func (adh *authDBHandler) insertIntoUserAccount(tx *sql.Tx, user *models.TheMonkeysUser) (int64, error) {
	stmt, err := tx.Prepare(`INSERT INTO user_account (
		account_id, username, first_name, last_name, email,
		role_id, user_status, view_permission) VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id;`)
	if err != nil {
		logrus.Errorf("cannot prepare statement to add user into the USER_ACCOUNT: %v", err)
		return 0, err
	}
	defer stmt.Close()

	var userId int64
	err = stmt.QueryRow(user.AccountId, user.Username, user.FirstName, user.LastName, user.Email, 4, 1, constants.UserPubilc).Scan(&userId)
	if err != nil {
		logrus.Errorf("cannot execute query to add user to the USER_ACCOUNT: %v", err)
		return 0, err
	}

	return userId, nil
}

func (adh *authDBHandler) insertIntoUserAuthInfo(tx *sql.Tx, user *models.TheMonkeysUser, userId int64) (int64, error) {
	stmt, err := tx.Prepare(`
	INSERT INTO user_auth_info (
	user_id, password_hash, 
	email_validation_token, email_validation_status, email_verification_timeout, auth_provider_id) 
	VALUES ($1, $2, $3, $4, $5, $6) RETURNING id;
	`)
	if err != nil {
		logrus.Errorf("cannot prepare statement to add user into the USER_AUTH_INFO: %v", err)
		return 0, err
	}
	defer stmt.Close()

	var authId int64
	err = stmt.QueryRow(userId, user.Password, user.EmailVerificationToken,
		1, user.EmailVerificationTimeout, 1).Scan(&authId) // TODO: emailVerificationStatus and auth provider make it correct
	if err != nil {
		logrus.Errorf("cannot execute query to add user to the USER_AUTH_INFO: %v", err)
		return 0, err
	}

	return authId, nil
}

func (adh *authDBHandler) UpdatePasswordRecoveryToken(hash string, req *models.TheMonkeysUser) error {
	// TODO: start a database transaction from here till all the process are complete
	tx, err := adh.db.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`UPDATE user_auth_info SET password_recovery_token=$1,
	password_recovery_timeout=$2 WHERE user_id=$3;`)
	if err != nil {
		logrus.Errorf("cannot prepare the reset link for %s, error: %v", req.Email, err)
		return status.Errorf(codes.Internal, "internal server error, error: %v", err)
	}

	defer stmt.Close()
	result := stmt.QueryRow(hash, time.Now().Add(time.Minute*5).Format(constants.DateTimeFormat), req.Id)
	if result.Err() != nil {
		logrus.Errorf("cannot sent the reset link for %s, error: %v", req.Email, err)
		return status.Errorf(codes.Internal, "internal server error, error: %v", err)
	}

	err = tx.Commit()
	if err != nil {
		logrus.Errorf("cannot commit the password recovery token for %s, error: %v", req.Email, err)
		return err
	}
	return nil
}

func (adh *authDBHandler) CheckIfUsernameExist(username string) (*models.TheMonkeysUser, error) {
	var tmu models.TheMonkeysUser
	if err := adh.db.QueryRow(`
			SELECT ua.id, ua.account_id, ua.username, ua.first_name, ua.last_name, 
			ua.email, uai.password_hash, uai.password_recovery_token, uai.password_recovery_timeout,
			evs.status, ua.user_status, uai.email_validation_token, uai.email_verification_timeout
			FROM user_account ua
			LEFT JOIN user_auth_info uai ON ua.id = uai.user_id
			LEFT JOIN email_validation_status evs ON uai.email_validation_status = evs.id
			WHERE ua.username = $1;	
		`, username).
		Scan(&tmu.Id, &tmu.AccountId, &tmu.Username, &tmu.FirstName, &tmu.LastName, &tmu.Email,
			&tmu.Password, &tmu.PasswordVerificationToken, &tmu.PasswordVerificationTimeout,
			&tmu.EmailVerificationStatus, &tmu.UserStatus, &tmu.EmailVerificationToken,
			&tmu.EmailVerificationTimeout); err != nil {
		logrus.Errorf("can't find a user existing with username %s, error: %+v", username, err)
		return nil, err
	}

	return &tmu, nil
}

func (adh *authDBHandler) UpdatePassword(password string, user *models.TheMonkeysUser) error {
	tx, err := adh.db.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`UPDATE user_auth_info SET password_hash=$1 WHERE user_id=$2 RETURNING user_id;`)
	if err != nil {
		logrus.Errorf("cannot prepare statement to update password for %s error: %+v", user.Email, err)
		return err
	}
	defer stmt.Close()

	var userId int64
	err = stmt.QueryRow(password, user.Id).Scan(&userId)
	if err != nil {
		logrus.Errorf("cannot update the password for %s, error: %v", user.Email, err)
		return err
	}

	fmt.Printf("userId: %v\n", userId)
	// TODO: Add a record into the log table using the userId

	err = tx.Commit()
	if err != nil {
		logrus.Errorf("cannot commit the password update for %s, error: %v", user.Email, err)
		return err
	}
	return nil
}

func (adh *authDBHandler) CreateUserLog(user *models.TheMonkeysUser, description string) error {
	var userId int64
	var clientId int8
	var err error

	//From username find user id
	if err = adh.db.QueryRow(`
			SELECT id FROM user_account WHERE account_id = $1;`, user.AccountId).Scan(&userId); err != nil {
		logrus.Errorf("can't get id by using account_id %s, error: %v", user.AccountId, err)
		return err
	}

	//From client name find client id
	if err := adh.db.QueryRow(`
			SELECT id FROM clients WHERE c_name = $1;`, user.Client).Scan(&clientId); err != nil {
		logrus.Errorf("can't get id by using client name %s, error: %+v", user.Client, err)
		return err
	}

	stmt, err := adh.db.Prepare(`INSERT INTO user_account_log (user_id, ip_address, description, client_id) VALUES ($1, $2, $3, $4);`)
	if err != nil {
		logrus.Errorf("cannot prepare statement to add user activity into the user_account_log: %v", err)
		return err
	}
	defer stmt.Close()

	row, err := stmt.Exec(userId, user.IpAddress, description, clientId)
	if err != nil {
		logrus.Errorf("cannot execute query to add user to the user_account_log: %v", err)
		return err
	}

	affectedRow, err := row.RowsAffected()
	if err != nil {
		logrus.Errorf("error finding affected rows for user_account_log: %v", err)
		return err
	}

	if affectedRow == 0 {
		logrus.Errorf("cannot create a record in the log table for user_account_log: %v", err)
		return errors.New("cannot create a record in the log table")
	}

	return nil
}

func (adh *authDBHandler) UpdateEmailVerificationToken(req *models.TheMonkeysUser) error {
	tx, err := adh.db.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`UPDATE user_auth_info SET email_validation_token=$1,
	email_verification_timeout=$2 WHERE user_id=$3;`)
	if err != nil {
		logrus.Errorf("cannot prepare the reset link for %s, error: %v", req.Email, err)
		return status.Errorf(codes.Internal, "internal server error, error: %v", err)
	}

	defer stmt.Close()
	result := stmt.QueryRow(req.EmailVerificationToken, req.EmailVerificationTimeout, req.Id)
	if result.Err() != nil {
		logrus.Errorf("cannot sent the reset link for %s, error: %v", req.Email, err)
		return status.Errorf(codes.Internal, "internal server error, error: %v", err)
	}

	err = tx.Commit()
	if err != nil {
		logrus.Errorf("cannot commit the password recovery token for %s, error: %v", req.Email, err)
		return err
	}
	return nil
}

func (adh *authDBHandler) UpdateEmailVerificationStatus(req *models.TheMonkeysUser) error {
	tx, err := adh.db.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`UPDATE user_auth_info SET email_validation_status=(SELECT id FROM email_validation_status WHERE status=$1),
	email_validation_time=$2 WHERE user_id=$3;`)
	if err != nil {
		logrus.Errorf("cannot prepare the reset link for %s, error: %v", req.Email, err)
		return status.Errorf(codes.Internal, "internal server error, error: %v", err)
	}

	defer stmt.Close()
	result := stmt.QueryRow("verified", time.Now(), req.Id)
	if result.Err() != nil {
		logrus.Errorf("cannot sent the reset link for %s, error: %v", req.Email, err)
		return status.Errorf(codes.Internal, "internal server error, error: %v", err)
	}

	err = tx.Commit()
	if err != nil {
		logrus.Errorf("cannot commit the password recovery token for %s, error: %v", req.Email, err)
		return err
	}
	return nil
}
