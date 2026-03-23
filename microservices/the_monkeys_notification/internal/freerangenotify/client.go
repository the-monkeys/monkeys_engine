package freerangenotify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	frn "github.com/the-monkeys/freerangenotify/sdk/go/freerangenotify"
	"go.uber.org/zap"
)

// Client wraps the official FRN Go SDK, adding dev-email override logic.
// Only the notification service uses this — other services publish to RabbitMQ.
type Client struct {
	SDK      *frn.Client
	BaseURL  string // Stored for raw HTTP calls the SDK doesn't support
	APIKey   string
	DevEmail string // Override all email recipients in dev (empty = no override)
	Log      *zap.SugaredLogger
}

func NewClient(baseURL, apiKey, devEmail string, log *zap.SugaredLogger) *Client {
	sdk := frn.New(apiKey,
		frn.WithBaseURL(baseURL),
		frn.WithTimeout(5*time.Second),
	)
	return &Client{
		SDK:      sdk,
		BaseURL:  baseURL,
		APIKey:   apiKey,
		DevEmail: devEmail,
		Log:      log,
	}
}

// RegisterUser registers a Monkeys user in FRN so it can receive notifications.
func (c *Client) RegisterUser(ctx context.Context, email, username string) error {
	if c.DevEmail != "" {
		email = c.DevEmail
	}
	_, err := c.SDK.Users.Create(ctx, frn.CreateUserParams{
		Email:      email,
		ExternalID: username,
	})
	return err
}

// Send dispatches a single notification via the FRN SDK.
func (c *Client) Send(ctx context.Context, params frn.NotificationSendParams) error {
	_, err := c.SDK.Notifications.Send(ctx, params)
	return err
}

// UpdateUserExternalID updates a user's external_id in FRN via direct HTTP call.
// The SDK's Users.Update requires an internal UUID, but we only have the external_id
// (Monkeys username). FRN's API resolves external_id in /users/{external_id}.
func (c *Client) UpdateUserExternalID(ctx context.Context, oldExternalID, newExternalID string) error {
	payload, err := json.Marshal(map[string]string{
		"external_id": newExternalID,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal update payload: %w", err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/users/%s", c.BaseURL, oldExternalID)
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPut, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to build FRN user update request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to update FRN user external_id: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("FRN user update returned %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
