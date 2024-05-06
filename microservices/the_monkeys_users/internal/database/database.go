package database

import (
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_user/pb"
	"github.com/the-monkeys/the_monkeys/common"
	"github.com/the-monkeys/the_monkeys/config"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_users/internal/models"
)

type UserDb interface {
	CheckIfEmailExist(email string) (*models.TheMonkeysUser, error)
	CheckIfUsernameExist(username string) (*models.TheMonkeysUser, error)
	GetMyProfile(username string) (*models.UserProfileRes, error)
	GetUserProfile(username string) (*models.UserAccount, error)
	UpdateUserProfile(username string, dbUserInfo *models.UserProfileRes) error
	DeleteUserProfile(username string) error
	GetAllTopicsFromDb() (*pb.GetTopicsResponse, error)
	GetAllCategories() (*pb.GetAllCategoriesRes, error)
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

func (uh *uDBHandler) UpdateUserProfile(username string, dbUserInfo *models.UserProfileRes) error {
	// TODO: start a database transaction from here till all the process are complete
	tx, err := uh.db.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`
	UPDATE user_account
		SET 
		username = $1,
		first_name = $2,
		last_name = $3,
		email = $4,
		date_of_birth = $5,
		bio = $6,
        updated_at = now(),
		address = $7,
		contact_number = $8
		WHERE username = $9;
	`)
	if err != nil {
		logrus.Errorf("cannot prepare the update user query for user %s, error: %v", dbUserInfo.Username, err)
		return status.Errorf(codes.Internal, "internal server error, error: %v", err)
	}

	defer stmt.Close()
	result := stmt.QueryRow(dbUserInfo.Username, dbUserInfo.FirstName, dbUserInfo.LastName, dbUserInfo.Email,
		dbUserInfo.DateOfBirth.Time, dbUserInfo.Bio.String, dbUserInfo.Address.String, dbUserInfo.ContactNumber.String, username)
	if result.Err() != nil {
		logrus.Errorf("cannot update user %s, error: %v", dbUserInfo.Username, err)
		return status.Errorf(codes.Internal, "internal server error, error: %v", err)
	}

	err = tx.Commit()
	if err != nil {
		logrus.Errorf("cannot commit the update profile for user %s, error: %v", dbUserInfo.Username, err)
		return err
	}
	return nil
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

func (uh *uDBHandler) DeleteUserProfile(username string) error {
	tx, err := uh.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var id int64
	//using this username get id field from useraccount table
	if err := tx.QueryRow(`
			SELECT id FROM user_account where username = $1;`, username).Scan(&id); err != nil {
		logrus.Errorf("can't get id by using username %s, error: %+v", username, err)
		return nil
	}

	//using that id delete the row in userauthinfo table
	_, err = tx.Exec(`DELETE FROM user_auth_info WHERE user_id = $1`, id)
	if err != nil {
		return err
	}

	//using that id delete the row from useraccount table
	_, err = tx.Exec(`DELETE FROM user_account WHERE id = $1`, id)
	if err != nil {
		return err
	}

	tx.Commit()
	return nil
}

// Write a function to create a user log user_account_log
func (uh *uDBHandler) AddUserLog(username string, ip string, description string, clientName string) error {
	var userId int64
	var clientId int8
	//From username find user id
	if err := uh.db.QueryRow(`
			SELECT id FROM user_account WHERE username = $1;`, username).Scan(&userId); err != nil {
		logrus.Errorf("can't get id by using username %s, error: %+v", username, err)
		return nil
	}

	//From clientname find client id
	if err := uh.db.QueryRow(`
			SELECT id FROM clients WHERE c_name = $1;`, clientName).Scan(&clientId); err != nil {
		logrus.Errorf("can't get id by using client name %s, error: %+v", clientName, err)
		return nil
	}

	//Add a user log to user_account_log table
	stmt, err := uh.db.Prepare(`INSERT INTO user_account_log (user_id, ip_address, description, client_id) VALUES ($1, $2, $3, $4)`)
	if err != nil {
		logrus.Errorf("cannot prepare statement to add user log into the user_account_log: %v", err)
		return err
	}

	defer stmt.Close()

	row := stmt.QueryRow(userId, ip, description, clientId)
	if row.Err() != nil {
		logrus.Errorf("cannot execute query to log user into user_account_log: %v", row.Err())
		return row.Err()
	}

	return nil
}

func (uh *uDBHandler) GetAllTopicsFromDb() (*pb.GetTopicsResponse, error) {
	resp := &pb.GetTopicsResponse{}
	topics := []*pb.Topics{}
	rows, err := uh.db.Query("SELECT description, category FROM topics")
	if err != nil {
		// Check if the error is "not found" or "internal server error" and return accordingly
		if errors.Is(err, sql.ErrNoRows) {
			return nil, common.ErrNotFound
		}
		return nil, common.ErrInternal
	}
	defer rows.Close()

	var topic, category string
	for rows.Next() {
		err := rows.Scan(&topic, &category)
		if err != nil {
			return nil, err
		}
		topics = append(topics, &pb.Topics{
			Topic:    topic,
			Category: category,
		})
	}

	resp.Topics = topics
	return resp, nil
}
func (uh *uDBHandler) GetAllCategories() (*pb.GetAllCategoriesRes, error) {
	resp := &pb.GetAllCategoriesRes{}
	categories := make(map[string]*pb.Category)
	rows, err := uh.db.Query("SELECT  DISTINCT description, category FROM topics")
	if err != nil {
		// Check if the error is "not found" or "internal server error" and return accordingly
		if errors.Is(err, sql.ErrNoRows) {
			return nil, common.ErrNotFound
		}
		return nil, common.ErrInternal
	}
	defer rows.Close()

	var Description, category string
	for rows.Next() {
		err := rows.Scan(&Description, &category)
		if err != nil {
			return nil, err
		}
		if _, ok := categories[category]; !ok {
			categories[category] = &pb.Category{
				Topics: make([]string, 0), // Initialize Topics slice for the category
			}
		}

		// Append Description to the Topics slice of the corresponding category
		categories[category].Topics = append(categories[category].Topics, Description)
	}

	// Assign the map to resp.Categories
	resp.Category = categories
	return resp, nil
}
