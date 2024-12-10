package database

import (
	"database/sql"
	"fmt"

	"github.com/lib/pq"
	"github.com/sirupsen/logrus"

	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_notification/internal/models"
)

type NotificationDB interface {
	// Create Notification query
	CreateNotification(accountID string, notificationName string, message string, relatedBlogID, relatedUserAccountID string, channelName string) error
	GetUserNotifications(username string, limit int64, offset int64) ([]*models.Notification, error)
	MarkNotificationAsSeen(notificationIDs []int64, username string) error
	CheckIfUsernameExist(username string) (*models.TheMonkeysUser, error)
	GetUnseenNotifications(username string, limit int64, offset int64) ([]*models.Notification, error)
}

type notificationDB struct {
	db  *sql.DB
	log *logrus.Logger
}

func NewNotificationDb(cfg *config.Config, log *logrus.Logger) (NotificationDB, error) {
	url := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.Postgresql.PrimaryDB.DBUsername,
		cfg.Postgresql.PrimaryDB.DBPassword,
		cfg.Postgresql.PrimaryDB.DBHost,
		cfg.Postgresql.PrimaryDB.DBPort,
		cfg.Postgresql.PrimaryDB.DBName,
	)
	dbPsql, err := sql.Open("postgres", url)
	if err != nil {
		log.Fatalf("cannot connect psql using sql driver, error: %+v", err)
		return nil, err
	}

	if err = dbPsql.Ping(); err != nil {
		log.Errorf("ping test failed to psql using sql driver, error: %+v", err)
		return nil, err
	}
	log.Info("the monkeys notification service is connected to psql")
	return &notificationDB{db: dbPsql, log: log}, nil
}

// GetUserNotifications fetches notifications for a user with pagination
func (uh *notificationDB) GetUserNotifications(username string, limit int64, offset int64) ([]*models.Notification, error) {
	var userID int64
	err := uh.db.QueryRow(`SELECT id FROM user_account WHERE username = $1`, username).Scan(&userID)
	if err != nil {
		uh.log.Errorf("Error fetching user ID for username %s, error: %+v", username, err)
		return nil, err
	}

	// Step 1: Prepare the query with pagination
	query := `
		SELECT n.id, n.notification_type_id, nt.notification_name, n.message, n.related_blog_id, n.related_user_id, n.created_at, n.seen, n.delivery_status, nc.channel_name
		FROM notifications n
		JOIN notification_type nt ON n.notification_type_id = nt.id
		JOIN notification_channel nc ON n.channel_id = nc.id
		WHERE n.user_id = $1
		ORDER BY n.created_at DESC
		LIMIT $2 OFFSET $3;
	`

	// Step 2: Execute the query with pagination
	rows, err := uh.db.Query(query, userID, limit, offset)
	if err != nil {
		uh.log.Errorf("Error fetching notifications for user ID %d, error: %+v", userID, err)
		return nil, err
	}
	defer rows.Close()

	// Step 3: Collect the results into a slice of Notification structs
	var notifications []*models.Notification
	for rows.Next() {
		var notification models.Notification
		err := rows.Scan(&notification.ID, &notification.NotificationTypeID, &notification.NotificationName, &notification.Message,
			&notification.RelatedBlogID, &notification.RelatedUserID, &notification.CreatedAt, &notification.Seen,
			&notification.DeliveryStatus, &notification.ChannelName)
		if err != nil {
			uh.log.Errorf("Error scanning notification data for user ID %d, error: %+v", userID, err)
			return nil, err
		}
		notifications = append(notifications, &notification)
	}

	// Step 4: Check for errors after iteration
	if err := rows.Err(); err != nil {
		uh.log.Errorf("Row iteration error while fetching notifications for user ID %d, error: %+v", userID, err)
		return nil, err
	}

	uh.log.Infof("Successfully fetched notifications for user ID: %d", userID)
	return notifications, nil
}

func (uh *notificationDB) CreateNotification(accountID string, notificationName string, message string, relatedBlogID, relatedUserAccountID string, channelName string) error {
	// Step 1: Fetch the user_id based on accountID
	var userID int64
	err := uh.db.QueryRow(`SELECT id FROM user_account WHERE account_id = $1`, accountID).Scan(&userID)
	if err != nil {
		uh.log.Errorf("Error fetching user ID for accountID %s, error: %+v", accountID, err)
		return err
	}

	// Step 2: Fetch the notification_type_id based on notificationName
	var notificationTypeID int64
	err = uh.db.QueryRow(`SELECT id FROM notification_type WHERE notification_name = $1`, notificationName).Scan(&notificationTypeID)
	if err != nil {
		uh.log.Errorf("Error fetching notification type ID for notificationName %s, error: %+v", notificationName, err)
		return err
	}

	// Step 3: Fetch the channel_id based on channelName
	var channelID int64
	err = uh.db.QueryRow(`SELECT id FROM notification_channel WHERE channel_name = $1`, channelName).Scan(&channelID)
	if err != nil {
		uh.log.Errorf("Error fetching channel ID for channelName %s, error: %+v", channelName, err)
		return err
	}

	// Step 4: Handle nullable relatedBlogID and relatedUserAccountID
	var relatedBlogIDValue interface{} = nil
	if relatedBlogID != "" {
		relatedBlogIDValue = relatedBlogID // Use the provided blog ID if not empty
	}

	var relatedUserIDValue interface{} = nil
	if relatedUserAccountID != "" {
		// Fetch the related user ID if the related user account ID is provided
		err = uh.db.QueryRow(`SELECT id FROM user_account WHERE account_id = $1`, relatedUserAccountID).Scan(&relatedUserIDValue)
		if err != nil {
			uh.log.Errorf("Error fetching related user ID for relatedUserAccountID %s, error: %+v", relatedUserAccountID, err)
			return err
		}
	}

	// Step 5: Prepare the insert query
	query := `
        INSERT INTO notifications 
        (user_id, notification_type_id, message, related_blog_id, related_user_id, channel_id) 
        VALUES ($1, $2, $3, $4, $5, $6)`

	// Step 6: Execute the query
	_, err = uh.db.Exec(query, userID, notificationTypeID, message, relatedBlogIDValue, relatedUserIDValue, channelID)
	if err != nil {
		uh.log.Errorf("Error creating notification for user ID %d, error: %+v", userID, err)
		return err
	}

	uh.log.Infof("Successfully created notification for user ID: %d", userID)
	return nil
}

// MarkNotificationAsSeen updates the 'seen' status of multiple notifications for a user
func (uh *notificationDB) MarkNotificationAsSeen(notificationIDs []int64, username string) error {
	var userID int64
	err := uh.db.QueryRow(`SELECT id FROM user_account WHERE username = $1`, username).Scan(&userID)
	if err != nil {
		uh.log.Errorf("Error fetching user ID for username %s, error: %+v", username, err)
		return err
	}

	// Prepare the IN clause with ANY and use pq.Array to handle the slice
	query := `
		UPDATE notifications
		SET seen = TRUE
		WHERE id = ANY($1) AND user_id = $2 AND seen = FALSE;
	`

	// Execute the query with pq.Array to pass the slice correctly
	result, err := uh.db.Exec(query, pq.Array(notificationIDs), userID)
	if err != nil {
		uh.log.Errorf("Error marking notifications as seen for user ID %d, error: %+v", userID, err)
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		uh.log.Errorf("Error checking rows affected for user ID %d, error: %+v", userID, err)
		return err
	}

	if rowsAffected == 0 {
		uh.log.Infof("No changes made. Either notifications were already seen or do not exist for user ID %d", userID)
		return nil
	}

	uh.log.Infof("Successfully marked %d notifications as seen for user ID %d", rowsAffected, userID)
	return nil
}

func (uh *notificationDB) CheckIfUsernameExist(username string) (*models.TheMonkeysUser, error) {
	return uh.fetchUserByIdentifier("username", username)
}

func (uh *notificationDB) fetchUserByIdentifier(identifierType, identifierValue string) (*models.TheMonkeysUser, error) {
	var tmu models.TheMonkeysUser
	query := `
		SELECT ua.id, ua.account_id, ua.username, ua.first_name, ua.last_name, 
		ua.email, uai.password_hash, uai.password_recovery_token, uai.password_recovery_timeout,
		evs.status, us.status, uai.email_validation_token, uai.email_verification_timeout
		FROM USER_ACCOUNT ua
		LEFT JOIN USER_AUTH_INFO uai ON ua.id = uai.user_id
		LEFT JOIN email_validation_status evs ON uai.email_validation_status = evs.id
		LEFT JOIN user_status us ON ua.user_status = us.id
		WHERE ua.` + identifierType + ` = $1;
	`

	if err := uh.db.QueryRow(query, identifierValue).
		Scan(&tmu.Id, &tmu.AccountId, &tmu.Username, &tmu.FirstName, &tmu.LastName, &tmu.Email,
			&tmu.Password, &tmu.PasswordVerificationToken, &tmu.PasswordVerificationTimeout,
			&tmu.EmailVerificationStatus, &tmu.UserStatus, &tmu.EmailVerificationToken,
			&tmu.EmailVerificationTimeout); err != nil {
		uh.log.Errorf("Can't find a user with %s: %s, error: %+v", identifierType, identifierValue, err)
		return nil, err
	}

	return &tmu, nil
}

// GetUnseenNotifications fetches unseen notifications for a user with pagination
func (uh *notificationDB) GetUnseenNotifications(username string, limit int64, offset int64) ([]*models.Notification, error) {
	var userID int64

	// Step 1: Fetch the user ID for the provided username
	err := uh.db.QueryRow(`SELECT id FROM user_account WHERE username = $1`, username).Scan(&userID)
	if err != nil {
		uh.log.Errorf("Error fetching user ID for username %s, error: %+v", username, err)
		return nil, err
	}

	// Step 2: Prepare the query to fetch only unseen notifications
	query := `
        SELECT n.id, n.notification_type_id, nt.notification_name, n.message, n.related_blog_id, n.related_user_id, 
               n.created_at, n.seen, n.delivery_status, nc.channel_name
        FROM notifications n
        JOIN notification_type nt ON n.notification_type_id = nt.id
        JOIN notification_channel nc ON n.channel_id = nc.id
        WHERE n.user_id = $1 AND n.seen = FALSE
        ORDER BY n.created_at DESC
        LIMIT $2 OFFSET $3;
    `

	// Step 3: Execute the query
	rows, err := uh.db.Query(query, userID, limit, offset)
	if err != nil {
		uh.log.Errorf("Error fetching unseen notifications for user ID %d, error: %+v", userID, err)
		return nil, err
	}
	defer rows.Close()

	// Step 4: Collect the results into a slice of Notification structs
	var notifications []*models.Notification
	for rows.Next() {
		var notification models.Notification
		err := rows.Scan(&notification.ID, &notification.NotificationTypeID, &notification.NotificationName,
			&notification.Message, &notification.RelatedBlogID, &notification.RelatedUserID,
			&notification.CreatedAt, &notification.Seen, &notification.DeliveryStatus,
			&notification.ChannelName)
		if err != nil {
			uh.log.Errorf("Error scanning notification data for user ID %d, error: %+v", userID, err)
			return nil, err
		}
		notifications = append(notifications, &notification)
	}

	// Step 5: Check for errors after iteration
	if err := rows.Err(); err != nil {
		uh.log.Errorf("Row iteration error while fetching unseen notifications for user ID %d, error: %+v", userID, err)
		return nil, err
	}

	// uh.log.Infof("Successfully fetched unseen notifications for user ID: %d", userID)
	return notifications, nil
}
