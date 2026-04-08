package freerangenotify

import (
	"context"
	"errors"

	frn "github.com/the-monkeys/freerangenotify/sdk/go/freerangenotify"
	"go.uber.org/zap"
)

// NotifyRequest is the high-level notification dispatch request.
// The consumer builds one of these per event and calls Notify().
type NotifyRequest struct {
	UserID   string                 // Monkeys username (FRN external_id)
	InAppTpl string                 // Template name for in_app channel (always sent)
	SSETpl   string                 // Template name for sse channel (empty = skip)
	EmailTpl string                 // Template name for email channel (empty = skip)
	Priority string                 // "low", "normal", "high", "critical"
	Category string                 // "social", "collaboration", "content", "security"
	Data     map[string]interface{} // Template variables
}

// isUserNotFoundErr returns true when FRN indicates the user does not exist.
func isUserNotFoundErr(err error) bool {
	var apiErr *frn.APIError
	if errors.As(err, &apiErr) && apiErr.IsNotFound() {
		return true
	}
	return false
}

// isConflictErr returns true when FRN returns 409 Conflict (e.g. duplicate email).
func isConflictErr(err error) bool {
	var apiErr *frn.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == 409 {
		return true
	}
	return false
}

// Notify sends in_app (always), then SSE and email if templates are provided.
// SSE and email failures are logged but do not propagate — only in_app is critical.
// If the first send fails because the user doesn't exist in FRN, it auto-registers
// the user and retries once.
func Notify(ctx context.Context, client *Client, req NotifyRequest, log *zap.SugaredLogger) error {
	// in_app — always sent, error propagated
	if err := client.Send(ctx, frn.NotificationSendParams{
		UserID:     req.UserID,
		Channel:    "in_app",
		Priority:   req.Priority,
		TemplateID: req.InAppTpl,
		Category:   req.Category,
		Data:       req.Data,
	}); err != nil {
		// Auto-register user in FRN if they don't exist (handles pre-integration users)
		if isUserNotFoundErr(err) {
			log.Infow("FRN user not found, auto-registering", "user", req.UserID)
			if regErr := client.RegisterUser(ctx, "", req.UserID, "", ""); regErr != nil {
				// 409 = user already exists with same email (DevEmail override) — safe to retry
				if !isConflictErr(regErr) {
					log.Errorw("FRN auto-registration failed", "user", req.UserID, "err", regErr)
					return err // return original error
				}
				log.Infow("FRN user already exists (409), retrying notification", "user", req.UserID)
			}
			// Retry the in_app send after registration
			if retryErr := client.Send(ctx, frn.NotificationSendParams{
				UserID:     req.UserID,
				Channel:    "in_app",
				Priority:   req.Priority,
				TemplateID: req.InAppTpl,
				Category:   req.Category,
				Data:       req.Data,
			}); retryErr != nil {
				log.Errorw("FRN in_app notification failed after auto-register", "user", req.UserID, "tpl", req.InAppTpl, "err", retryErr)
				return retryErr
			}
		} else {
			log.Errorw("FRN in_app notification failed", "user", req.UserID, "tpl", req.InAppTpl, "err", err)
			return err
		}
	}

	// SSE — fire-and-forget
	if req.SSETpl != "" {
		if err := client.Send(ctx, frn.NotificationSendParams{
			UserID:     req.UserID,
			Channel:    "sse",
			Priority:   req.Priority,
			TemplateID: req.SSETpl,
			Category:   req.Category,
			Data:       req.Data,
		}); err != nil {
			log.Warnw("FRN SSE notification failed (non-blocking)", "user", req.UserID, "err", err)
		}
	}

	// Email — fire-and-forget
	if req.EmailTpl != "" {
		if err := client.Send(ctx, frn.NotificationSendParams{
			UserID:     req.UserID,
			Channel:    "email",
			Priority:   req.Priority,
			TemplateID: req.EmailTpl,
			Category:   req.Category,
			Data:       req.Data,
		}); err != nil {
			log.Warnw("FRN email notification failed (non-blocking)", "user", req.UserID, "err", err)
		}
	}

	return nil
}
