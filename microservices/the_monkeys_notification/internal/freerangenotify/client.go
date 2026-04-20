package freerangenotify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	frn "github.com/the-monkeys/freerangenotify/sdk/go/freerangenotify"
	"go.uber.org/zap"
)

// Client wraps the official FRN Go SDK.
// Only the notification service uses this — other services publish to RabbitMQ.
type Client struct {
	SDK     *frn.Client
	Log     *zap.SugaredLogger
	baseURL string
	apiKey  string
	http    *http.Client
}

func NewClient(baseURL, apiKey string, log *zap.SugaredLogger) *Client {
	sdk := frn.New(apiKey,
		frn.WithBaseURL(baseURL),
		frn.WithTimeout(5*time.Second),
	)
	return &Client{
		SDK:     sdk,
		Log:     log,
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 10 * time.Second},
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

// UpdateUserExternalID renames a user's external_id in FRN.
// FRN's PUT /users/:id with external_id as identifier creates a new empty user
// instead of updating when the identifier-as-external_id differs from the body.
// Workaround: resolve internal UUID via GetByExternalID, then PUT by UUID.
//
// NOTE: The FRN SDK unmarshals response envelopes {"data":{...},"success":true}
// directly into the target struct, leaving all fields zero. We use raw HTTP here
// to correctly unwrap the envelope.
func (c *Client) UpdateUserExternalID(ctx context.Context, oldExternalID, newExternalID string) error {
	c.Log.Debugw("FRN UpdateUserExternalID called", "old_external_id", oldExternalID, "new_external_id", newExternalID)

	internalID, err := c.resolveInternalID(ctx, oldExternalID)
	if err != nil {
		return fmt.Errorf("resolve user by external_id %q: %w", oldExternalID, err)
	}
	if internalID == "" {
		return fmt.Errorf("user with external_id %q not found in FRN", oldExternalID)
	}

	if _, err := c.SDK.Users.Update(ctx, internalID, frn.UpdateUserParams{
		ExternalID: newExternalID,
	}); err != nil {
		return fmt.Errorf("update user %s external_id: %w", internalID, err)
	}
	return nil
}

// resolveInternalID fetches a user by external_id and returns their internal UUID.
// Parses the FRN envelope {"data":{"user_id":"..."},"success":true}.
func (c *Client) resolveInternalID(ctx context.Context, externalID string) (string, error) {
	url := c.baseURL + "/users/by-external-id/" + externalID
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("FRN GET user status=%d body=%s", resp.StatusCode, string(body))
	}

	var env struct {
		Data    frn.User `json:"data"`
		Success bool     `json:"success"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return "", fmt.Errorf("decode FRN user response: %w", err)
	}
	return env.Data.UserID, nil
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
