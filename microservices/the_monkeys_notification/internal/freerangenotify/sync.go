package freerangenotify

import (
	"context"
	"strings"
	"time"

	frn "github.com/the-monkeys/freerangenotify/sdk/go/freerangenotify"
	"go.uber.org/zap"
)

// UserLister provides the minimal DB interface needed by SyncUsers.
type UserLister interface {
	ListActiveUsers(ctx context.Context) ([]BasicUser, error)
}

// BasicUser is a lightweight user record for FRN sync.
type BasicUser struct {
	AccountID string
	Username  string
	Email     string
	FirstName string
	LastName  string
}

// SyncUsers registers all active Monkeys users in FRN on startup.
// Uses BulkCreate with SkipExisting so already-registered users are silently skipped.
// Runs in the background — errors are logged, never fatal.
func SyncUsers(ctx context.Context, client *Client, lister UserLister, log *zap.SugaredLogger) {
	log.Debug("FRN user sync: starting background sync of existing users")
	start := time.Now()

	users, err := lister.ListActiveUsers(ctx)
	if err != nil {
		log.Errorw("FRN user sync: failed to list active users from DB", "err", err)
		return
	}
	log.Debugw("FRN user sync: fetched user list", "count", len(users))

	if len(users) == 0 {
		log.Debug("FRN user sync: no users to sync")
		return
	}

	// Build the bulk create params
	createParams := make([]frn.CreateUserParams, 0, len(users))
	for _, u := range users {
		externalID := u.Username
		if externalID == "" {
			externalID = u.AccountID
		}
		fullName := strings.TrimSpace(u.FirstName + " " + u.LastName)
		createParams = append(createParams, frn.CreateUserParams{
			FullName:   fullName,
			Email:      u.Email,
			ExternalID: externalID,
		})
	}

	resp, err := client.SDK.Users.BulkCreate(ctx, frn.BulkCreateUsersParams{
		Users:        createParams,
		SkipExisting: true,
	})
	if err != nil {
		log.Errorw("FRN user sync: bulk create failed", "err", err)
		return
	}

	log.Debugw("FRN user sync: complete",
		"response", resp,
		"total", len(users),
		"duration", time.Since(start).Round(time.Millisecond),
	)
}
