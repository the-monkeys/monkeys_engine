package models

import "time"

type Notification struct {
	ID                 int64     `json:"id"`
	NotificationTypeID int       `json:"notification_type_id"`
	NotificationName   string    `json:"notification_name"`
	Message            string    `json:"message"`
	RelatedBlogID      *int64    `json:"related_blog_id,omitempty"`
	RelatedUserID      *int64    `json:"related_user_id,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	Seen               bool      `json:"seen"`
	DeliveryStatus     string    `json:"delivery_status"`
	ChannelName        string    `json:"channel_name"`
}
