package database

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/lib/pq"
	// _ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_user/pb"
	"github.com/the-monkeys/the_monkeys/constants"

	"github.com/the-monkeys/the_monkeys/config"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_users/internal/models"
)

type UserDb interface {
	// Create queries
	CreateUserLog(user *models.UserLogs, description string) error
	AddBlogWithId(models.TheMonkeysMessage) error
	AddBlogPermission(models.TheMonkeysMessage) error
	AddUserInterest(interest []string, username string) error

	// Get queries
	CheckIfEmailExist(email string) (*models.TheMonkeysUser, error)
	CheckIfUsernameExist(username string) (*models.TheMonkeysUser, error)
	CheckIfAccIdExist(accountId string) (*models.TheMonkeysUser, error)
	GetMyProfile(username string) (*models.UserProfileRes, error)
	GetUserProfile(username string) (*models.UserAccount, error)
	GetAllTopicsFromDb() (*pb.GetTopicsResponse, error)
	GetAllCategories() (*pb.GetAllCategoriesRes, error)
	GetUserActivities(userId int64) (*pb.UserActivityResp, error)
	GetUserInterest(username string) ([]string, error)
	// Update queries
	UpdateUserProfile(username string, dbUserInfo *models.UserProfileRes) error
	UpdateBlogStatusToPublish(blogId string, status string) error

	// Delete queries
	DeleteUserProfile(username string) error
	RemoveUserInterest(interests []string, username string) error
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

	// Step 1: Fetch user profile information from the user_account table
	err := uh.db.QueryRow(`
		SELECT id, username, first_name, last_name, bio, avatar_url, created_at, address, linkedin, github, twitter, instagram 
		FROM user_account WHERE username = $1;`, username).
		Scan(&tmu.Id, &tmu.UserName, &tmu.FirstName, &tmu.LastName, &tmu.Bio, &tmu.AvatarUrl, &tmu.CreatedAt,
			&tmu.Address, &tmu.LinkedIn, &tmu.Github, &tmu.Twitter, &tmu.Instagram)

	if err != nil {
		uh.log.Errorf("Can't find a user with username %s, error: %+v", username, err)
		return nil, err
	}

	// Step 2: Fetch user's interests by joining user_interest and topics tables
	rows, err := uh.db.Query(`
		SELECT t.description 
		FROM topics t
		JOIN user_interest ui ON t.id = ui.topics_id
		WHERE ui.user_id = $1;`, tmu.Id)

	if err != nil {
		uh.log.Errorf("Error fetching interests for user ID %d, error: %+v", tmu.Id, err)
		return nil, err
	}
	defer rows.Close()

	// Step 3: Collect the interests into the UserAccount struct
	var interests []string
	for rows.Next() {
		var interest string
		if err := rows.Scan(&interest); err != nil {
			uh.log.Errorf("Error scanning interest for user ID %d, error: %+v", tmu.Id, err)
			return nil, err
		}
		interests = append(interests, interest)
	}

	// Assign interests to the user's profile
	tmu.Interests = interests

	uh.log.Infof("Successfully fetched profile and interests for user: %s", username)
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

	// Step 1: Fetch user profile information
	err := uh.db.QueryRow(`
		SELECT ua.account_id, ua.username, ua.first_name, ua.last_name, ua.email, ua.date_of_birth,
		ua.bio, ua.avatar_url, ua.created_at, ua.updated_at, ua.address, ua.contact_number, us.status,
		ua.view_permission, ua.linkedin, ua.github, ua.twitter, ua.instagram 
		FROM user_account ua
		INNER JOIN user_status us ON us.id = ua.user_status
		WHERE ua.username = $1;
	`, username).
		Scan(&profile.AccountId, &profile.Username, &profile.FirstName, &profile.LastName, &profile.Email,
			&profile.DateOfBirth, &profile.Bio, &profile.AvatarUrl, &profile.CreatedAt, &profile.UpdatedAt,
			&profile.Address, &profile.ContactNumber, &profile.UserStatus, &profile.ViewPermission,
			&profile.LinkedIn, &profile.Github, &profile.Twitter, &profile.Instagram)

	if err != nil {
		logrus.Errorf("can't find a user profile with username %s, error: %+v", username, err)
		return nil, err
	}

	// Step 2: Fetch user's interests by joining user_interest and topics tables
	rows, err := uh.db.Query(`
		SELECT t.description 
		FROM topics t
		JOIN user_interest ui ON t.id = ui.topics_id
		JOIN user_account ua ON ua.id = ui.user_id
		WHERE ua.username = $1;
	`, username)

	if err != nil {
		logrus.Errorf("Error fetching interests for username %s, error: %+v", username, err)
		return nil, err
	}
	defer rows.Close()

	// Step 3: Collect the interests into the UserProfileRes struct
	var interests []string
	for rows.Next() {
		var interest string
		if err := rows.Scan(&interest); err != nil {
			logrus.Errorf("Error scanning interest for user %s, error: %+v", username, err)
			return nil, err
		}
		interests = append(interests, interest)
	}

	// Assign the collected interests to the profile
	profile.Interests = interests

	logrus.Infof("Successfully fetched profile and interests for user: %s", username)
	return &profile, nil
}

func (uh *uDBHandler) UpdateUserProfile(username string, dbUserInfo *models.UserProfileRes) error {
	uh.log.Infof("Starting profile update for user: %s", username)

	tx, err := uh.db.Begin()
	if err != nil {
		uh.log.Errorf("cannot begin transaction for user %s, error: %v", username, err)
		return status.Errorf(codes.Internal, "internal server error, error: %v", err)
	}

	stmt, err := tx.Prepare(`
		UPDATE user_account
		SET 
			first_name = $1,
			last_name = $2,
			date_of_birth = $3,
			bio = $4,
			updated_at = now(),
			address = $5,
			contact_number = $6,
			linkedin = $7,
			github = $8,
			twitter = $9,
			instagram = $10 
		WHERE username = $11;
	`)
	if err != nil {
		uh.log.Errorf("cannot prepare the update user query for user %s, error: %v", username, err)
		return status.Errorf(codes.Internal, "internal server error, error: %v", err)
	}
	defer stmt.Close()

	result := stmt.QueryRow(dbUserInfo.FirstName, dbUserInfo.LastName, dbUserInfo.DateOfBirth.Time,
		dbUserInfo.Bio.String, dbUserInfo.Address.String, dbUserInfo.ContactNumber.String,
		dbUserInfo.LinkedIn.String, dbUserInfo.Github.String, dbUserInfo.Twitter.String,
		dbUserInfo.Instagram.String, username)
	if result.Err() != nil {
		uh.log.Errorf("cannot update user %s, error: %v", username, result.Err())
		if pqErr, ok := result.Err().(*pq.Error); ok && pqErr.Code == "23505" {
			return status.Errorf(codes.AlreadyExists, "email already exists")
		}
		return status.Errorf(codes.Internal, "internal server error, error: %v", result.Err())
	}

	err = tx.Commit()
	if err != nil {
		uh.log.Errorf("cannot commit the update profile for user %s, error: %v", username, err)
		return status.Errorf(codes.Internal, "internal server error, error: %v", err)
	}

	uh.log.Infof("Successfully updated profile for user: %s", username)
	return nil
}

func (uh *uDBHandler) DeleteUserProfile(username string) error {
	tx, err := uh.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var id int64
	//using this username get id field from user account table
	if err := tx.QueryRow(`
			SELECT id FROM user_account where username = $1;`, username).Scan(&id); err != nil {
		logrus.Errorf("can't get id by using username %s, error: %+v", username, err)
		return nil
	}

	//using that id delete the row in user auth info table
	_, err = tx.Exec(`DELETE FROM user_auth_info WHERE user_id = $1`, id)
	if err != nil {
		return err
	}

	//using that id delete the row from user account table
	_, err = tx.Exec(`DELETE FROM user_account WHERE id = $1`, id)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}
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
			return nil, constants.ErrNotFound
		}
		return nil, constants.ErrInternal
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
			return nil, constants.ErrNotFound
		}
		return nil, constants.ErrInternal
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

func (uh *uDBHandler) AddBlogWithId(msg models.TheMonkeysMessage) error {
	tx, err := uh.db.Begin()
	if err != nil {
		return err
	}

	var userId int64
	//From account_id find user_id
	if err := tx.QueryRow(`
			SELECT id FROM user_account WHERE account_id = $1;`, msg.UserAccountId).Scan(&userId); err != nil {
		logrus.Errorf("can't get id by using user_account %s, error: %+v", msg.UserAccountId, err)
		return nil
	}

	stmt, err := tx.Prepare(`INSERT INTO blog (user_id, blog_id, status) VALUES ($1, $2, $3) RETURNING id;`)
	if err != nil {
		uh.log.Errorf("cannot prepare statement to add blog into the blog: %v", err)
		return err
	}
	defer stmt.Close()

	var blogId int64
	err = stmt.QueryRow(userId, msg.BlogId, msg.Status).Scan(&blogId)
	if err != nil {
		logrus.Errorf("cannot execute query to add blog into the blog: %v", err)
		return err
	}

	stmt2, err := tx.Prepare(`INSERT INTO blog_permissions (blog_id, user_id, permission_type) VALUES ($1, $2, $3);`)
	if err != nil {
		uh.log.Errorf("cannot prepare statement to add permission into the blog_permissions: %v", err)
		return err
	}

	row := stmt2.QueryRow(blogId, userId, constants.RoleOwner)
	if row.Err() != nil {
		logrus.Errorf("cannot execute query to add permission into the blog_permissions: %v", row.Err())
		return row.Err()
	}

	err = tx.Commit()
	if err != nil {
		logrus.Errorf("cannot commit the add blog for user %s, error: %v", msg.UserAccountId, err)
		return err
	}

	return nil
}

func (uh *uDBHandler) GetUserActivities(userId int64) (*pb.UserActivityResp, error) {
	uh.log.Infof("Retrieving user activity for: %v", userId)
	activities := []*pb.UserActivity{}
	rows, err := uh.db.Query("SELECT description, timestamp FROM user_account_log WHERE user_id = $1 ORDER BY timestamp DESC;", userId)
	if err != nil {
		uh.log.Errorf("error retrieving user activities for user id %v, err: %v", userId, err)
		return nil, status.Errorf(codes.Internal, "cannot get the user activity")
	}
	defer rows.Close()

	for rows.Next() {
		var desc, timestamp string
		err := rows.Scan(&desc, &timestamp)
		if err != nil {
			uh.log.Errorf("cannot scan the user activity, err: %v", err)
			return nil, status.Errorf(codes.Internal, "cannot scan the user activity")
		}
		activities = append(activities, &pb.UserActivity{
			Description: desc,
			Timestamp:   timestamp,
		})
	}

	if err = rows.Err(); err != nil {
		uh.log.Errorf("error iterating over rows, err: %v", err)
		return nil, status.Errorf(codes.Internal, "error iterating over rows")
	}

	if len(activities) == 0 {
		return nil, status.Errorf(codes.NotFound, fmt.Sprintf("activity for user id %v doesn't exist", userId))
	}

	return &pb.UserActivityResp{
		Response: activities,
	}, nil
}

func (uh *uDBHandler) AddBlogPermission(models.TheMonkeysMessage) error {
	return nil
}

func (uh *uDBHandler) UpdateBlogStatusToPublish(blogId string, status string) error {
	uh.log.Infof("the blog %v is being published", blogId)

	row := uh.db.QueryRow("UPDATE blog SET status = $1 WHERE blog_id = $2", status, blogId)
	if row.Err() != nil {
		return row.Err()
	}

	uh.log.Infof("the blog %v is successfully published", blogId)
	return nil
}

func (uh *uDBHandler) CheckIfAccIdExist(accountId string) (*models.TheMonkeysUser, error) {
	var tmu models.TheMonkeysUser
	if err := uh.db.QueryRow(`
			SELECT ua.id, ua.account_id, ua.username, ua.first_name, ua.last_name, 
			ua.email, uai.password_hash, uai.password_recovery_token, uai.password_recovery_timeout,
			evs.status, us.status, uai.email_validation_token, uai.email_verification_timeout
			FROM USER_ACCOUNT ua
			LEFT JOIN USER_AUTH_INFO uai ON ua.id = uai.user_id
			LEFT JOIN email_validation_status evs ON uai.email_validation_status = evs.id
			LEFT JOIN user_status us ON ua.user_status = us.id
			WHERE ua.account_id = $1;
		`, accountId).
		Scan(&tmu.Id, &tmu.AccountId, &tmu.Username, &tmu.FirstName, &tmu.LastName, &tmu.Email,
			&tmu.Password, &tmu.PasswordVerificationToken, &tmu.PasswordVerificationTimeout,
			&tmu.EmailVerificationStatus, &tmu.UserStatus, &tmu.EmailVerificationToken,
			&tmu.EmailVerificationTimeout); err != nil {
		logrus.Errorf("can't find a user existing with accountId %s, error: %+v", accountId, err)
		return nil, err
	}

	return &tmu, nil
}

func (uh *uDBHandler) AddUserInterest(interests []string, username string) error {
	// Start a transaction
	tx, err := uh.db.Begin()
	if err != nil {
		uh.log.Errorf("Failed to start transaction: %+v", err)
		return err
	}

	// Step 1: Fetch the user ID based on username
	var userId int64
	err = tx.QueryRow(`SELECT id FROM user_account WHERE username = $1`, username).Scan(&userId)
	if err != nil {
		uh.log.Errorf("Failed to fetch user ID for username: %s, error: %+v", username, err)
		tx.Rollback() // rollback transaction on error
		return err
	}

	// Step 2: Iterate over the interests and insert them into the user_interest table
	for _, interest := range interests {
		// Fetch the topic ID based on the interest description
		var topicId int
		err = tx.QueryRow(`SELECT id FROM topics WHERE description = $1`, interest).Scan(&topicId)
		if err != nil {
			uh.log.Errorf("Failed to fetch topic ID for interest: %s, error: %+v", interest, err)
			tx.Rollback() // rollback transaction on error
			return err
		}

		// Step 3: Check if the user is already following this interest
		var exists int
		err = tx.QueryRow(`SELECT COUNT(1) FROM user_interest WHERE user_id = $1 AND topics_id = $2`, userId, topicId).Scan(&exists)
		if err != nil {
			uh.log.Errorf("Failed to check if user is already following interest: %s, error: %+v", interest, err)
			tx.Rollback() // rollback transaction on error
			return err
		}

		// If the user is already following the interest, skip the insert and log it
		if exists > 0 {
			uh.log.Infof("User %s already follows interest: %s, skipping", username, interest)
			continue
		}

		// Insert into user_interest table for interests not already followed
		_, err = tx.Exec(`INSERT INTO user_interest (user_id, topics_id) VALUES ($1, $2)`, userId, topicId)
		if err != nil {
			uh.log.Errorf("Failed to insert user interest for username: %s and interest: %s, error: %+v", username, interest, err)
			tx.Rollback() // rollback transaction on error
			return err
		}
	}

	// Step 4: Commit the transaction
	if err := tx.Commit(); err != nil {
		uh.log.Errorf("Failed to commit transaction: %+v", err)
		return err
	}

	uh.log.Infof("Successfully added new interests for user: %s", username)
	return nil
}

func (uh *uDBHandler) GetUserInterest(username string) ([]string, error) {
	// Step 1: Fetch the user ID based on username
	var userId int64
	err := uh.db.QueryRow(`SELECT id FROM user_account WHERE username = $1`, username).Scan(&userId)
	if err != nil {
		uh.log.Errorf("Failed to fetch user ID for username: %s, error: %+v", username, err)
		return nil, err
	}

	// Step 2: Fetch the user's interests (topics) based on user_id
	rows, err := uh.db.Query(`
        SELECT t.description 
        FROM topics t
        JOIN user_interest ui ON t.id = ui.topics_id
        WHERE ui.user_id = $1`, userId)
	if err != nil {
		uh.log.Errorf("Failed to fetch user interests for user ID: %d, error: %+v", userId, err)
		return nil, err
	}
	defer rows.Close()

	// Step 3: Collect the descriptions of the topics
	var interests []string
	for rows.Next() {
		var description string
		if err := rows.Scan(&description); err != nil {
			uh.log.Errorf("Failed to scan topic description, error: %+v", err)
			return nil, err
		}
		interests = append(interests, description)
	}

	if err := rows.Err(); err != nil {
		uh.log.Errorf("Error iterating over user interests, error: %+v", err)
		return nil, err
	}

	uh.log.Infof("Successfully fetched interests for user: %s", username)
	return interests, nil
}

func (uh *uDBHandler) RemoveUserInterest(interests []string, username string) error {
	// Start a transaction
	tx, err := uh.db.Begin()
	if err != nil {
		uh.log.Errorf("Failed to start transaction: %+v", err)
		return err
	}

	// Step 1: Fetch the user ID based on username
	var userId int64
	err = tx.QueryRow(`SELECT id FROM user_account WHERE username = $1`, username).Scan(&userId)
	if err != nil {
		uh.log.Errorf("Failed to fetch user ID for username: %s, error: %+v", username, err)
		tx.Rollback() // rollback transaction on error
		return err
	}

	// Step 2: Iterate over the interests and remove them from the user_interest table
	for _, interest := range interests {
		// Fetch the topic ID based on the interest description
		var topicId int
		err = tx.QueryRow(`SELECT id FROM topics WHERE description = $1`, interest).Scan(&topicId)
		if err != nil {
			uh.log.Errorf("Failed to fetch topic ID for interest: %s, error: %+v", interest, err)
			tx.Rollback() // rollback transaction on error
			return err
		}

		// Step 3: Check if the user is actually following this interest
		var exists int
		err = tx.QueryRow(`SELECT COUNT(1) FROM user_interest WHERE user_id = $1 AND topics_id = $2`, userId, topicId).Scan(&exists)
		if err != nil {
			uh.log.Errorf("Failed to check if user follows interest: %s, error: %+v", interest, err)
			tx.Rollback() // rollback transaction on error
			return err
		}

		// If the user is not following the interest, skip and log it
		if exists == 0 {
			uh.log.Infof("User %s does not follow interest: %s, skipping removal", username, interest)
			continue
		}

		// Remove the user's interest from the user_interest table
		_, err = tx.Exec(`DELETE FROM user_interest WHERE user_id = $1 AND topics_id = $2`, userId, topicId)
		if err != nil {
			uh.log.Errorf("Failed to remove user interest for username: %s and interest: %s, error: %+v", username, interest, err)
			tx.Rollback() // rollback transaction on error
			return err
		}
	}

	// Step 4: Commit the transaction
	if err := tx.Commit(); err != nil {
		uh.log.Errorf("Failed to commit transaction: %+v", err)
		return err
	}

	uh.log.Infof("Successfully removed selected interests for user: %s", username)
	return nil
}
