package freerangenotify

import (
	"context"
	"strings"
	"time"

	frn "github.com/the-monkeys/freerangenotify/sdk/go/freerangenotify"
	"go.uber.org/zap"
)

// Client wraps the official FRN Go SDK.
// Only the notification service uses this — other services publish to RabbitMQ.
type Client struct {
	SDK *frn.Client
	Log *zap.SugaredLogger
}

func NewClient(baseURL, apiKey string, log *zap.SugaredLogger) *Client {
	sdk := frn.New(apiKey,
		frn.WithBaseURL(baseURL),
		frn.WithTimeout(5*time.Second),
	)
	return &Client{
		SDK: sdk,
		Log: log,
	}
}

// RegisterUser registers a Monkeys user in FRN so it can receive notifications.
// Maps username → external_id for future notification routing.
func (c *Client) RegisterUser(ctx context.Context, email, username, firstName, lastName string) error {
	fullName := strings.TrimSpace(firstName + " " + lastName)
	c.Log.Debugw("FRN RegisterUser called", "username", username, "email", email, "full_name", fullName)

	_, err := c.SDK.Users.Create(ctx, frn.CreateUserParams{
		FullName:   fullName,
		Email:      email,
		ExternalID: username,
	})
	if err != nil {
		c.Log.Debugw("FRN RegisterUser failed", "username", username, "email", email, "err", err)
	} else {
		c.Log.Debugw("FRN RegisterUser succeeded", "username", username, "external_id", username, "full_name", fullName)
	}
	return err
}

// Send dispatches a single notification via the FRN SDK.
func (c *Client) Send(ctx context.Context, params frn.NotificationSendParams) error {
	c.Log.Debugw("FRN Send called", "user_id", params.UserID, "channel", params.Channel, "template", params.TemplateID)
	_, err := c.SDK.Notifications.Send(ctx, params)
	if err != nil {
		c.Log.Debugw("FRN Send failed", "user_id", params.UserID, "channel", params.Channel, "err", err)
	}
	return err
}

// UpdateUserExternalID updates a user's external_id in FRN.
// SDK now accepts external_id as the identifier directly.
func (c *Client) UpdateUserExternalID(ctx context.Context, oldExternalID, newExternalID string) error {
	c.Log.Debugw("FRN UpdateUserExternalID called", "old_external_id", oldExternalID, "new_external_id", newExternalID)
	_, err := c.SDK.Users.Update(ctx, oldExternalID, frn.UpdateUserParams{
		ExternalID: newExternalID,
	})
	return err
}

// UpdateUserEmail updates a user's email in FRN, identified by their username (external_id).
func (c *Client) UpdateUserEmail(ctx context.Context, username, newEmail string) error {
	c.Log.Debugw("FRN UpdateUserEmail called", "external_id", username, "new_email", newEmail)
	_, err := c.SDK.Users.Update(ctx, username, frn.UpdateUserParams{
		Email: newEmail,
	})
	return err
}

// DeleteUser removes a user from FRN, identified by their username (external_id).
func (c *Client) DeleteUser(ctx context.Context, username string) error {
	c.Log.Debugw("FRN DeleteUser called", "external_id", username)
	return c.SDK.Users.Delete(ctx, username)
}

// DeactivateUser sets DND (Do Not Disturb) on the FRN user to suppress all notifications.
func (c *Client) DeactivateUser(ctx context.Context, username string) error {
	c.Log.Debugw("FRN DeactivateUser called (DND=true)", "external_id", username)
	_, err := c.SDK.Users.UpdatePreferences(ctx, username, frn.Preferences{DND: true})
	return err
}

// ReactivateUser clears DND on the FRN user to resume notifications.
func (c *Client) ReactivateUser(ctx context.Context, username string) error {
	c.Log.Debugw("FRN ReactivateUser called (DND=false)", "external_id", username)
	_, err := c.SDK.Users.UpdatePreferences(ctx, username, frn.Preferences{DND: false})
	return err
}

// UpdateUserPreferences syncs notification preferences from Monkeys to FRN.
func (c *Client) UpdateUserPreferences(ctx context.Context, username string, emailEnabled, pushEnabled bool) error {
	c.Log.Debugw("FRN UpdateUserPreferences called", "external_id", username, "email_enabled", emailEnabled, "push_enabled", pushEnabled)
	_, err := c.SDK.Users.UpdatePreferences(ctx, username, frn.Preferences{
		EmailEnabled: &emailEnabled,
		PushEnabled:  &pushEnabled,
	})
	return err
}
