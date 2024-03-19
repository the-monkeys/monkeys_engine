package database

import (
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/config"

	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_users/internal/models"
)

type UserDb interface {
	CheckIfEmailExist(email string) (*models.TheMonkeysUser, error)
	CheckIfUsernameExist(username string) (*models.TheMonkeysUser, error)
	GetMyProfile(username string) (*models.UserProfileRes, error)
	GetUserProfile(username string) (*models.UserAccount, error)
}

type uDBHandler struct {
	db  *sql.DB
	log *logrus.Logger
}

func NewUserDbHandler(cfg *config.Config, log *logrus.Logger) (UserDb, error) {
	url := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.Postgresql.PrimaryDB.DBUsername,
		cfg.Postgresql.PrimaryDB.DBPassword,
		cfg.Postgresql.PrimaryDB.DBHost,
		cfg.Postgresql.PrimaryDB.DBPort,
		cfg.Postgresql.PrimaryDB.DBName,
	)
	dbPsql, err := sql.Open("postgres", url)
	if err != nil {
		logrus.Fatalf("cannot connect psql using sql driver, error: %+v", err)
		return nil, err
	}

	if err = dbPsql.Ping(); err != nil {
		logrus.Errorf("ping test failed to psql using sql driver, error: %+v", err)
		return nil, err
	}

	return &uDBHandler{db: dbPsql, log: log}, nil
}

// To get User Profile
func (uh *uDBHandler) GetUserProfile(username string) (*models.UserAccount, error) {
	var tmu models.UserAccount
	if err := uh.db.QueryRow(`
        SELECT username, first_name, last_name, bio, avatar_url 
        FROM user_account WHERE username = $1;`, username).
		Scan(&tmu.UserName, &tmu.FirstName, &tmu.LastName, &tmu.Bio, &tmu.AvatarUrl); err != nil {
		logrus.Errorf("can't find a user existing with this profile id  %s, error: %+v", username, err)
		return nil, err
	}

	return &tmu, nil
}

// TODO: Find all the fields of models.TheMonkeysUser
func (uh *uDBHandler) CheckIfEmailExist(email string) (*models.TheMonkeysUser, error) {
	var tmu models.TheMonkeysUser
	if err := uh.db.QueryRow(`
            SELECT ua.id, ua.account_id, ua.username, ua.first_name, ua.last_name, 
            ua.email, uai.password_hash, evs.status, us.status, uai.email_validation_token,
            uai.email_verification_timeout
            FROM USER_ACCOUNT ua
            LEFT JOIN USER_AUTH_INFO uai ON ua.id = uai.user_id
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

func (uh *uDBHandler) CheckIfUsernameExist(username string) (*models.TheMonkeysUser, error) {
	var tmu models.TheMonkeysUser
	if err := uh.db.QueryRow(`
			SELECT ua.id, ua.account_id, ua.username, ua.first_name, ua.last_name, 
			ua.email, uai.password_hash, uai.password_recovery_token, uai.password_recovery_timeout,
			evs.status, us.status, uai.email_validation_token, uai.email_verification_timeout
			FROM USER_ACCOUNT ua
			LEFT JOIN USER_AUTH_INFO uai ON ua.id = uai.user_id
			LEFT JOIN email_validation_status evs ON uai.email_validation_status = evs.id
			LEFT JOIN user_status us ON ua.user_status = us.id
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

func (uh *uDBHandler) GetMyProfile(username string) (*models.UserProfileRes, error) {
	var profile models.UserProfileRes
	if err := uh.db.QueryRow(`
			SELECT ua.account_id, ua.username, ua.first_name, ua.last_name, ua.email, ua.date_of_birth,
			ua.bio, ua.avatar_url, ua.created_at, ua.updated_at, ua.address, ua.contact_number, us.status,
			ua.view_permission
			FROM user_account ua
			INNER JOIN user_status us ON us.id = ua.user_status
			WHERE ua.username = $1;
		`, username).
		Scan(&profile.AccountId, &profile.Username, &profile.FirstName, &profile.LastName, &profile.Email,
			&profile.DateOfBirth, &profile.Bio, &profile.AvatarUrl, &profile.CreatedAt, &profile.UpdatedAt,
			&profile.Address, &profile.ContactNumber, &profile.UserStatus, &profile.ViewPermission); err != nil {
		logrus.Errorf("can't find a user profile existing with username %s, error: %+v", username, err)
		return nil, err
	}

	return &profile, nil
}

// TODO: If the record doesn't exist throw 404 error
// func (uh *uDBHandler) UpdateMyProfile(info *pb.SetMyProfileReq) error {
// 	stmt, err := uh.db.Prepare(`UPDATE the_monkeys_user SET first_name=$1, last_name=$2,
// 	country_code=$3, mobile_no=$4, about=$5, instagram=$6, twitter=$7, update_time=$8 WHERE id=$9`)
// 	if err != nil {
// 		uh.log.Errorf("cannot prepare update profile statement, error: %v", err)
// 		return err
// 	}
// 	defer stmt.Close()
// 	time := time.Now().Format(common.DATE_TIME_FORMAT)
// 	res, err := stmt.Exec(info.FirstName, info.LastName, info.CountryCode, info.MobileNo,
// 		info.About, info.Instagram, info.Twitter, time, info.Id)
// 	if err != nil {
// 		uh.log.Errorf("cannot execute update profile statement, error: %v", err)
// 		return err
// 	}

// 	row, err := res.RowsAffected()
// 	if err != nil {
// 		logrus.Errorf("error while checking rows affected for %s, error: %v", info.Email, err)
// 		return err
// 	}
// 	if row > 1 {
// 		logrus.Errorf("more or less than 1 row is affected for %s, error: %v", info.Email, err)
// 		return errors.New("more or less than 1 row is affected")
// 	}

// 	return nil
// }

// TODO: If the record doesn't exist throw 404 error
func (uh *uDBHandler) UploadProfilePic(pic []byte, id int64) error {
	stmt, err := uh.db.Prepare(`UPDATE the_monkeys_user SET profile_pic=$1 WHERE id=$2`)
	if err != nil {
		uh.log.Errorf("cannot prepare upload profile pic statement, error: %v", err)
		return err
	}
	defer stmt.Close()

	res, err := stmt.Exec(pic, id)
	if err != nil {
		uh.log.Errorf("cannot execute update profile pic statement, error: %v", err)
		return err
	}

	row, err := res.RowsAffected()
	if err != nil {
		logrus.Errorf("error while checking rows affected for %d, error: %v", id, err)
		return err
	}
	if row != 1 {
		logrus.Errorf("more or less than 1 row is affected for %d, error: %v", id, err)
		return errors.New("more or less than 1 row is affected")
	}

	return nil
}

func (uh *uDBHandler) DeactivateMyAccount(id int64) error {
	stmt, err := uh.db.Prepare(`UPDATE the_monkeys_user SET deactivated=true WHERE id=$1`)
	if err != nil {
		uh.log.Errorf("cannot prepare deactivate profile statement, error: %v", err)
		return err
	}
	defer stmt.Close()

	res, err := stmt.Exec(id)
	if err != nil {
		uh.log.Errorf("cannot execute deactivate profile statement, error: %v", err)
		return err
	}

	row, err := res.RowsAffected()
	if err != nil {
		logrus.Errorf("error while checking rows affected for %d, error: %v", id, err)
		return err
	}
	if row != 1 {
		logrus.Errorf("more or less than 1 row is affected for %d, error: %v", id, err)
		return errors.New("more or less than 1 row is affected")
	}

	return nil
}
