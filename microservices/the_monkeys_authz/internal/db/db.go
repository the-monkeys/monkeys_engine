package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/constants"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_authz/internal/models"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AuthDBHandler interface {
	// Get Operations
	CheckIfEmailExist(email string) (*models.TheMonkeysUser, error)
	CheckIfUsernameExist(username string) (*models.TheMonkeysUser, error)
	GetUserAccessForABlog(accountId, blogId string) ([]string, string, error)

	// Create Operations
	RegisterUser(user *models.TheMonkeysUser) (int64, error)

	// Update Operations
	UpdatePasswordRecoveryToken(hash string, req *models.TheMonkeysUser) error
	UpdatePassword(password string, user *models.TheMonkeysUser) error
	UpdateEmailVerificationToken(req *models.TheMonkeysUser) error
	UpdateEmailVerificationStatus(req *models.TheMonkeysUser) error
	UpdateUserName(currentUsername, newUsername string) error
	UpdateEmailId(emailId string, user *models.TheMonkeysUser) error

	// Create user logs to track activity
	CreateUserLog(user *models.TheMonkeysUser, description string) error
}
type authDBHandler struct {
	db  *sql.DB
	log *zap.SugaredLogger
}

// NewAuthDBHandler creates a new instance of AuthDBHandler
func NewAuthDBHandler(cfg *config.Config, logger *zap.SugaredLogger) (AuthDBHandler, error) {
	url := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.Postgresql.PrimaryDB.DBUsername,
		cfg.Postgresql.PrimaryDB.DBPassword,
		cfg.Postgresql.PrimaryDB.DBHost,
		cfg.Postgresql.PrimaryDB.DBPort,
		cfg.Postgresql.PrimaryDB.DBName,
	)

	dbPsql, err := sql.Open("postgres", url)
	if err != nil {
		logger.Fatalw("cannot connect psql using sql driver", "error", err)
		return nil, err
	}

	// Configure connection pooling
	dbPsql.SetMaxOpenConns(25)
	dbPsql.SetMaxIdleConns(10)
	dbPsql.SetConnMaxLifetime(5 * time.Minute)

	if err = dbPsql.Ping(); err != nil {
		logger.Errorw("ping test failed to psql", "error", err)
		return nil, err
	}

	return &authDBHandler{db: dbPsql, log: logger}, nil
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
		adh.log.Errorf("can't find a user existing with email %s", email, err)
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
		adh.log.Warnf("we are detecting some data inconsistency for user %s", user.Email)
	}

	return userId, nil
}

func (adh *authDBHandler) insertIntoUserAccount(tx *sql.Tx, user *models.TheMonkeysUser) (int64, error) {
	stmt, err := tx.Prepare(`INSERT INTO user_account (
		account_id, username, first_name, last_name, email,
		role_id, user_status, view_permission) VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id;`)
	if err != nil {
		adh.log.Errorf("cannot prepare statement to add user into the USER_ACCOUNT: %v", err)
		return 0, err
	}
	defer func() {
		if err := stmt.Close(); err != nil {
			adh.log.Errorf("cannot close statement to add user into the USER_ACCOUNT: %v", err)
		}
	}()

	var userId int64
	err = stmt.QueryRow(user.AccountId, user.Username, user.FirstName, user.LastName, user.Email, 4, 1, constants.UserPublic).Scan(&userId)
	if err != nil {
		adh.log.Errorf("cannot execute query to add user to the USER_ACCOUNT: %v", err)
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
		adh.log.Errorf("cannot prepare statement to add user into the USER_AUTH_INFO: %v", err)
		return 0, err
	}
	defer func() {
		if err := stmt.Close(); err != nil {
			adh.log.Errorf("cannot close statement to add user into the USER_AUTH_INFO: %v", err)
		}
	}()

	var authId int64
	err = stmt.QueryRow(userId, user.Password, user.EmailVerificationToken,
		1, user.EmailVerificationTimeout, 1).Scan(&authId) // TODO: emailVerificationStatus and auth provider make it correct
	if err != nil {
		adh.log.Errorf("cannot execute query to add user to the USER_AUTH_INFO: %v", err)
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
		adh.log.Errorf("cannot prepare the reset link for %s, error: %v", req.Email, err)
		return status.Errorf(codes.Internal, "internal server error, error: %v", err)
	}

	defer func() {
		if err := stmt.Close(); err != nil {
			adh.log.Errorf("cannot close statement to update password recovery token for %s, error: %v", req.Email, err)
		}
	}()
	result := stmt.QueryRow(hash, time.Now().Add(time.Minute*5).Format(constants.DateTimeFormat), req.Id)
	if result.Err() != nil {
		adh.log.Errorf("cannot sent the reset link for %s, error: %v", req.Email, err)
		return status.Errorf(codes.Internal, "internal server error, error: %v", err)
	}

	err = tx.Commit()
	if err != nil {
		adh.log.Errorf("cannot commit the password recovery token for %s, error: %v", req.Email, err)
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
		adh.log.Errorf("can't find a user existing with username %s, error: %+v", username, err)
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
		adh.log.Errorf("cannot prepare statement to update password for %s error: %+v", user.Email, err)
		return err
	}
	defer func() {
		if err := stmt.Close(); err != nil {
			adh.log.Errorf("cannot close statement to update password for %s error: %+v", user.Email, err)
		}
	}()

	var userId int64
	err = stmt.QueryRow(password, user.Id).Scan(&userId)
	if err != nil {
		adh.log.Errorf("cannot update the password for %s, error: %v", user.Email, err)
		return err
	}

	// fmt.Printf("userId: %v\n", userId)
	// TODO: Add a record into the log table using the userId

	err = tx.Commit()
	if err != nil {
		adh.log.Errorf("cannot commit the password update for %s, error: %v", user.Email, err)
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
		adh.log.Errorf("can't get id by using account_id %s, error: %v", user.AccountId, err)
		return err
	}

	//From client name find client id
	if err := adh.db.QueryRow(`
			SELECT id FROM clients WHERE c_name = $1;`, user.Client).Scan(&clientId); err != nil {
		adh.log.Errorf("can't get id by using client name %s, error: %+v", user.Client, err)
		return err
	}

	stmt, err := adh.db.Prepare(`INSERT INTO user_account_log (user_id, ip_address, description, client_id) VALUES ($1, $2, $3, $4);`)
	if err != nil {
		adh.log.Errorf("cannot prepare statement to add user activity into the user_account_log: %v", err)
		return err
	}
	defer func() {
		if err := stmt.Close(); err != nil {
			adh.log.Errorf("cannot close statement to add user activity into the user_account_log: %v", err)
		}
	}()

	row, err := stmt.Exec(userId, user.IpAddress, description, clientId)
	if err != nil {
		adh.log.Errorf("cannot execute query to add user to the user_account_log: %v", err)
		return err
	}

	affectedRow, err := row.RowsAffected()
	if err != nil {
		adh.log.Errorf("error finding affected rows for user_account_log: %v", err)
		return err
	}

	if affectedRow == 0 {
		adh.log.Errorf("cannot create a record in the log table for user_account_log: %v", err)
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
		adh.log.Errorf("cannot prepare the reset link for %s, error: %v", req.Email, err)
		return status.Errorf(codes.Internal, "internal server error, error: %v", err)
	}

	defer func() {
		if err := stmt.Close(); err != nil {
			adh.log.Errorf("cannot close statement to update email verification token for %s, error: %v", req.Email, err)
		}
	}()
	result := stmt.QueryRow(req.EmailVerificationToken, req.EmailVerificationTimeout, req.Id)
	if result.Err() != nil {
		adh.log.Errorf("cannot sent the reset link for %s, error: %v", req.Email, err)
		return status.Errorf(codes.Internal, "internal server error, error: %v", err)
	}

	err = tx.Commit()
	if err != nil {
		adh.log.Errorf("cannot commit the password recovery token for %s, error: %v", req.Email, err)
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
		adh.log.Errorf("cannot prepare the reset link for %s, error: %v", req.Email, err)
		return status.Errorf(codes.Internal, "internal server error, error: %v", err)
	}

	defer func() {
		if err := stmt.Close(); err != nil {
			adh.log.Errorf("cannot close statement to update email verification status for %s, error: %v", req.Email, err)
		}
	}()
	_, err = stmt.Exec(constants.EmailVerificationStatusVerified, time.Now(), req.Id)
	if err != nil {
		adh.log.Errorf("cannot update the email verification status for %s, error: %v", req.Email, err)
		return status.Errorf(codes.Internal, "internal server error, error: %v", err)
	}

	err = tx.Commit()
	if err != nil {
		adh.log.Errorf("cannot commit the email verification status update for %s, error: %v", req.Email, err)
		return err
	}
	return nil
}

func (adh *authDBHandler) UpdateUserName(currentUsername, newUsername string) error {
	stmt, err := adh.db.Prepare(`UPDATE user_account SET username = $1 WHERE username = $2`)
	if err != nil {
		adh.log.Errorf("cannot prepare statement to update username, error: %v", err)
		return err
	}
	defer func() {
		if err := stmt.Close(); err != nil {
			adh.log.Errorf("cannot close statement to update username, error: %v", err)
		}
	}()

	res, err := stmt.Exec(newUsername, currentUsername)
	if err != nil {
		adh.log.Errorf("cannot execute update username query, error: %v", err)
		return err
	}

	row, err := res.RowsAffected()
	if err != nil {
		adh.log.Errorf("error while checking rows affected for update username query, error: %v", err)
		return err
	}
	if row != 1 {
		adh.log.Errorf("more or less than 1 row is affected for update username query, error: %v", err)
		return errors.New("more or less than 1 row is affected")
	}

	return nil
}

func (adh *authDBHandler) UpdateEmailId(emailId string, user *models.TheMonkeysUser) error {
	if emailId == "" || user.Username == "" {
		return errors.New("both emailId and username are required")
	}

	adh.log.Infof("emailId: %v, username: %v", emailId, user.Username)

	// Begin a transaction
	tx, err := adh.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Update the email and set email_validation_status to 1
	updateEmailQuery := `
		UPDATE user_account
		SET email = $1, updated_at = CURRENT_TIMESTAMP
		WHERE username = $2
		RETURNING id
	`
	var userID int64
	err = tx.QueryRow(updateEmailQuery, emailId, user.Username).Scan(&userID)
	if err != nil {
		if err = tx.Rollback(); err != nil {
			adh.log.Errorf("Rollback failed: %v", err)
			return err
		}
		return fmt.Errorf("failed to update email: %w", err)
	}

	// Update the email_validation_status in user_auth_info
	updateStatusQuery := `
		UPDATE user_auth_info
		SET email_validation_status = 1, email_validation_token = $1, 
		email_verification_timeout = $2
		WHERE user_id = $3
	`
	result, err := tx.Exec(updateStatusQuery, user.EmailVerificationToken, user.EmailVerificationTimeout.Time, userID)
	if err != nil {
		if err = tx.Rollback(); err != nil {
			adh.log.Errorf("Rollback failed: %v", err)
			return err
		}
		return fmt.Errorf("failed to update email validation status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		if err = tx.Rollback(); err != nil {
			adh.log.Errorf("Rollback failed: %v", err)
			return err
		}
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		if err = tx.Rollback(); err != nil {
			adh.log.Errorf("Rollback failed: %v", err)
			return err
		}
		return fmt.Errorf("no user_auth_info record found for user_id %d", userID)
	}

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		if err = tx.Rollback(); err != nil {
			adh.log.Errorf("Rollback failed: %v", err)
			return err
		}
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetUserAccessForABlog retrieves the permissions and role for a given user on a specific blog
func (adh *authDBHandler) GetUserAccessForABlog(accountId, blogId string) ([]string, string, error) {
	// Default permission for all users
	var permissions = []string{constants.PermissionCreate}
	var userRole string

	// Get the blog ID and status from the blog table
	var blogID int64
	var blogStatus string
	err := adh.db.QueryRow("SELECT id, status FROM blog WHERE blog_id = $1", blogId).Scan(&blogID, &blogStatus)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, "", sql.ErrNoRows
		}
		return nil, "", fmt.Errorf("failed to get blog ID and status: %w", err)
	}

	// Get the user ID from the user_account table
	var userID int64
	err = adh.db.QueryRow("SELECT id FROM user_account WHERE account_id = $1", accountId).Scan(&userID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get user ID: %w", err)
	}

	// Check if there is a specific permission mapping in blog_permissions for the user and blog
	var permissionType string
	err = adh.db.QueryRow("SELECT permission_type FROM blog_permissions WHERE blog_id = $1 AND user_id = $2", blogID, userID).Scan(&permissionType)
	if err != nil && err != sql.ErrNoRows {
		return nil, "", fmt.Errorf("failed to query blog_permissions: %w", err)
	}

	// If a permission is found, grant the respective permissions based on the permission type
	if err == nil {
		switch permissionType {
		case "owner", "Owner":
			// Grant all permissions if the user is the owner
			permissions = append(permissions, "Read", "Edit", "Delete", "Archive", "Transfer-Ownership", "Publish", "Draft")
			userRole = "owner"
		case "editor", "Editor":
			// Grant editor permissions
			permissions = append(permissions, "Read", "Edit", "Publish", "Draft")
			userRole = "editor"
		case "viewer", "Viewer":
			// Grant viewer permission
			permissions = append(permissions, "Read")
			userRole = "viewer"
		}
	} else if blogStatus == constants.BlogStatusPublished {
		// If no specific mapping exists and the blog is published, give Read access
		permissions = append(permissions, "Read")
		userRole = "viewer" // Assuming "viewer" role if the blog is published and the user has no specific role
	} else {
		// If no permissions found and blog is not published, return no access
		return nil, "", fmt.Errorf("no access available for the blog")
	}

	return permissions, userRole, nil
}

// func (adh *authDBHandler) GetUserAccessForABlog(accountId, blogId string) ([]string, error) {
// 	var permissions []string

// 	// Get the blog ID and status from the blog table
// 	var blogID int64
// 	var blogStatus string
// 	err := adh.db.QueryRow("SELECT id, status FROM blog WHERE blog_id = $1", blogId).Scan(&blogID, &blogStatus)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to get blog ID and status: %w", err)
// 	}

// 	// Get the user ID from the user_account table
// 	var userID int64
// 	err = adh.db.QueryRow("SELECT id FROM user_account WHERE account_id = $1", accountId).Scan(&userID)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to get user ID: %w", err)
// 	}

// 	// First, check if the user has been invited as a co-author and whether they accepted the invitation
// 	var inviteStatus string
// 	err = adh.db.QueryRow("SELECT invite_status FROM co_author_invites WHERE blog_id = $1 AND invitee_id = $2", blogID, userID).Scan(&inviteStatus)
// 	if err != nil && err != sql.ErrNoRows {
// 		return nil, fmt.Errorf("failed to query co_author_invites: %w", err)
// 	}

// 	// If the user has accepted the invitation, get their co-author permissions
// 	var roleID int64
// 	if err == nil && inviteStatus == "accepted" {
// 		err = adh.db.QueryRow("SELECT role_id FROM co_author_permissions WHERE blog_id = $1 AND co_author_id = $2", blogID, userID).Scan(&roleID)
// 		if err != nil && err != sql.ErrNoRows {
// 			return nil, fmt.Errorf("failed to query co_author_permissions: %w", err)
// 		}
// 	}

// 	var permissionType string
// 	// If no co-author permissions, check for specific permissions in blog_permissions table
// 	if len(permissions) == 0 {
// 		err = adh.db.QueryRow("SELECT permission_type FROM blog_permissions WHERE blog_id = $1 AND user_id = $2", blogID, userID).Scan(&permissionType)
// 		if err != nil && err != sql.ErrNoRows {
// 			return nil, fmt.Errorf("failed to query blog_permissions: %w", err)
// 		}
// 	}

// 	// Retrieve the granted permissions dynamically from the permissions_granted table
// 	if roleID != 0 {
// 		rows, err := adh.db.Query(`
//             SELECT p.permission_desc
//             FROM permissions p
//             JOIN permissions_granted pg ON p.permission_id = pg.permission_id
//             WHERE pg.role_id = $1`, roleID)
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to retrieve permissions: %w", err)
// 		}
// 		defer rows.Close()

// 		for rows.Next() {
// 			var permission string
// 			if err := rows.Scan(&permission); err != nil {
// 				return nil, fmt.Errorf("failed to scan permission: %w", err)
// 			}
// 			permissions = append(permissions, permission)
// 		}
// 	}

// 	// If no permissions found and the blog is published, give public Read access
// 	if len(permissions) == 0 && blogStatus == constants.BlogStatusPublished {
// 		permissions = append(permissions, "Read")
// 	}

// 	// If no permissions are found and blog is not published, return no access
// 	if len(permissions) == 0 {
// 		return nil, fmt.Errorf("no access available for the blog")
// 	}

// 	return permissions, nil
// }
