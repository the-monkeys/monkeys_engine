package database

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"

	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_notification/internal/models"
)

// -- ================================
// -- Notification-related Tables
// -- ================================

// -- Table to store notification channels
// CREATE TABLE IF NOT EXISTS notification_channel (
//     id SERIAL PRIMARY KEY,
//     channel_name VARCHAR(50) NOT NULL UNIQUE -- 'Browser', 'Email', 'WhatsApp', 'SMS', 'OTP'
// );

// -- Table to store notification types
// CREATE TABLE IF NOT EXISTS notification_type (
//     id SERIAL PRIMARY KEY,
//     notification_name VARCHAR(100) NOT NULL UNIQUE,
//     description TEXT
// );

// -- Table to store notifications for users
// CREATE TABLE IF NOT EXISTS notifications (
//     id SERIAL PRIMARY KEY,
//     user_id BIGINT NOT NULL,
//     notification_type_id INTEGER NOT NULL,
//     message TEXT NOT NULL,
//     related_blog_id BIGINT,
//     related_user_id BIGINT,
//     created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
//     seen BOOLEAN DEFAULT FALSE,
//     delivery_status VARCHAR(20) DEFAULT 'pending',
//     channel_id INTEGER NOT NULL,
//     FOREIGN KEY (user_id) REFERENCES user_account(id) ON DELETE CASCADE,
//     FOREIGN KEY (notification_type_id) REFERENCES notification_type(id) ON DELETE CASCADE,
//     FOREIGN KEY (related_blog_id) REFERENCES blog(id) ON DELETE CASCADE,
//     FOREIGN KEY (related_user_id) REFERENCES user_account(id) ON DELETE CASCADE,
//     FOREIGN KEY (channel_id) REFERENCES notification_channel(id) ON DELETE CASCADE
// );

// -- Table for managing user notification preferences
// CREATE TABLE IF NOT EXISTS user_notification_preferences (
//     user_id BIGINT NOT NULL,
//     channel_id INTEGER NOT NULL,
//     is_enabled BOOLEAN DEFAULT TRUE,
//     PRIMARY KEY (user_id, channel_id),
//     FOREIGN KEY (user_id) REFERENCES user_account(id) ON DELETE CASCADE,
//     FOREIGN KEY (channel_id) REFERENCES notification_channel(id) ON DELETE CASCADE
// );

// -- ================================
// -- Browser Notification-related Tables
// -- ================================

// -- Table to store web push tokens
// CREATE TABLE IF NOT EXISTS web_push_tokens (
//     id SERIAL PRIMARY KEY,
//     user_id BIGINT NOT NULL,
//     endpoint TEXT NOT NULL,
//     p256dh_key TEXT NOT NULL,
//     auth_key TEXT NOT NULL,
//     FOREIGN KEY (user_id) REFERENCES user_account(id) ON DELETE CASCADE
// );

// -- ================================
// -- Email Notification-related Tables
// -- ================================

// -- Table to store email templates
// CREATE TABLE IF NOT EXISTS email_templates (
//     id SERIAL PRIMARY KEY,
//     template_name VARCHAR(100) NOT NULL,
//     subject VARCHAR(255) NOT NULL,
//     body TEXT NOT NULL,
//     created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
// );

// -- ================================
// -- WhatsApp Notification-related Tables
// -- ================================

// -- Table to store WhatsApp notifications
// CREATE TABLE IF NOT EXISTS whatsapp_notifications (
//     id SERIAL PRIMARY KEY,
//     user_id BIGINT NOT NULL,
//     message TEXT NOT NULL,
//     message_status VARCHAR(50) DEFAULT 'pending',
//     sent_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
//     FOREIGN KEY (user_id) REFERENCES user_account(id) ON DELETE CASCADE
// );

// -- ================================
// -- SMS and OTP-related Tables
// -- ================================

// -- Table to store SMS notifications
// CREATE TABLE IF NOT EXISTS sms_notifications (
//     id SERIAL PRIMARY KEY,
//     user_id BIGINT NOT NULL,
//     message TEXT NOT NULL,
//     phone_number VARCHAR(20) NOT NULL,
//     message_status VARCHAR(50) DEFAULT 'pending',
//     sent_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
//     FOREIGN KEY (user_id) REFERENCES user_account(id) ON DELETE CASCADE
// );

// -- Table to store OTP logs
// CREATE TABLE IF NOT EXISTS otp_logs (
//
//	id SERIAL PRIMARY KEY,
//	user_id BIGINT NOT NULL,
//	otp_code VARCHAR(6) NOT NULL,
//	sent_via VARCHAR(20) NOT NULL,
//	is_verified BOOLEAN DEFAULT FALSE,
//	expires_at TIMESTAMP NOT NULL,
//	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
//	verified_at TIMESTAMP,
//	FOREIGN KEY (user_id) REFERENCES user_account(id) ON DELETE CASCADE
//
// );
type NotificationDB interface {
	// Create Notification query

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
func (uh *notificationDB) GetUserNotifications(userID int64, limit int, offset int) ([]*models.Notification, error) {
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

// CreateNotification inserts a new notification into the notifications table
func (uh *notificationDB) CreateNotification(userID int64, notificationTypeID int, message string, relatedBlogID, relatedUserID *int64, channelID int) error {
	// Step 1: Prepare the insert query
	query := `
		INSERT INTO notifications 
		(user_id, notification_type_id, message, related_blog_id, related_user_id, channel_id) 
		VALUES ($1, $2, $3, $4, $5, $6)`

	// Step 2: Execute the query
	_, err := uh.db.Exec(query, userID, notificationTypeID, message, relatedBlogID, relatedUserID, channelID)
	if err != nil {
		uh.log.Errorf("Error creating notification for user ID %d, error: %+v", userID, err)
		return err
	}

	uh.log.Infof("Successfully created notification for user ID: %d", userID)
	return nil
}

// MarkNotificationAsSeen updates the 'seen' status of a notification for a user
func (uh *notificationDB) MarkNotificationAsSeen(notificationID int64, userID int64) error {
	query := `
		UPDATE notifications
		SET seen = TRUE
		WHERE id = $1 AND user_id = $2 AND seen = FALSE;
	`

	result, err := uh.db.Exec(query, notificationID, userID)
	if err != nil {
		uh.log.Errorf("Error marking notification as seen for notification ID %d, user ID %d, error: %+v", notificationID, userID, err)
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		uh.log.Errorf("Error checking rows affected for notification ID %d, user ID %d, error: %+v", notificationID, userID, err)
		return err
	}

	if rowsAffected == 0 {
		uh.log.Infof("No changes made. Either notification ID %d was already seen or does not exist for user ID %d", notificationID, userID)
		return nil
	}

	uh.log.Infof("Successfully marked notification ID %d as seen for user ID %d", notificationID, userID)
	return nil
}
