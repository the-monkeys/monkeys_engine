package consumer

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/constants"
	"github.com/the-monkeys/the_monkeys/microservices/rabbitmq"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_notification/internal/freerangenotify"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_notification/internal/models"
	"go.uber.org/zap"
)

func ConsumeFromQueue(mgr *rabbitmq.ConnManager, conf config.RabbitMQ, log *zap.SugaredLogger, frn *freerangenotify.Client) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Debug("Notification consumer: received termination signal, exiting")
		os.Exit(0)
	}()

	go consumeQueue(mgr, conf.Queues[4], log, frn)
	select {}
}

func consumeQueue(mgr *rabbitmq.ConnManager, queueName string, log *zap.SugaredLogger, frn *freerangenotify.Client) {
	backoff := time.Second

	for {
		msgs, err := mgr.Channel().Consume(
			queueName,
			"",
			true, // auto-ack
			false,
			false,
			false,
			nil,
		)
		if err != nil {
			log.Errorf("Notification consumer: failed to register on queue '%s', reconnecting in %v: %v", queueName, backoff, err)
			time.Sleep(backoff)
			if backoff *= 2; backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			mgr.Reconnect()
			continue
		}

		backoff = time.Second
		log.Info("Notification consumer: registered on queue: ", queueName)

		for d := range msgs {
			user := models.TheMonkeysMessage{}
			if err := json.Unmarshal(d.Body, &user); err != nil {
				log.Errorf("Failed to unmarshal user from RabbitMQ: %v", err)
				continue
			}
			handleUserAction(user, log, frn)
		}

		log.Warn("Notification consumer: channel closed, reconnecting...")
		mgr.Reconnect()
	}
}

func handleUserAction(user models.TheMonkeysMessage, log *zap.SugaredLogger, frn *freerangenotify.Client) {
	ctx := context.Background()

	switch user.Action {
	case constants.USER_REGISTER:
		log.Debugf("Received user registration: %s", user.Username)
		// Register user in FRN so it can receive future notifications
		if err := frn.RegisterUser(ctx, user.Email, user.Username); err != nil {
			log.Errorw("FRN user registration failed", "user", user.Username, "err", err)
		}
		// Send welcome notification (in_app only)
		if err := freerangenotify.Notify(ctx, frn, freerangenotify.NotifyRequest{
			UserID:   user.Username,
			InAppTpl: constants.FRNTplEmailVerifiedInApp,
			Priority: "high",
			Category: constants.FRNCategorySecurity,
			Data:     map[string]interface{}{},
		}, log); err != nil {
			log.Errorw("FRN welcome notification failed", "user", user.Username, "err", err)
		}

	case constants.USER_FOLLOWED:
		log.Debugf("Received user follow: %s → %s", user.Username, user.NewUsername)
		if err := freerangenotify.Notify(ctx, frn, freerangenotify.NotifyRequest{
			UserID:   user.NewUsername,
			InAppTpl: constants.FRNTplNewFollowerInApp,
			SSETpl:   constants.FRNTplNewFollowerSSE,
			Priority: "normal",
			Category: constants.FRNCategorySocial,
			Data: map[string]interface{}{
				"follower_name": user.Username,
			},
		}, log); err != nil {
			log.Errorw("FRN follow notification failed", "follower", user.Username, "target", user.NewUsername, "err", err)
		}

	case constants.BLOG_LIKE:
		log.Debugf("Received blog like: %s liked %s", user.Username, user.BlogId)
		if err := freerangenotify.Notify(ctx, frn, freerangenotify.NotifyRequest{
			UserID:   user.Username,
			InAppTpl: constants.FRNTplBlogLikedInApp,
			Priority: "low",
			Category: constants.FRNCategorySocial,
			Data: map[string]interface{}{
				"liker_name": user.Username,
				"blog_title": user.BlogTitle,
			},
		}, log); err != nil {
			log.Errorw("FRN like notification failed", "user", user.Username, "err", err)
		}

	case constants.CO_AUTHOR_INVITE:
		log.Debugf("Received co-author invite: %s invited %s for blog %s", user.Username, user.NewUsername, user.BlogId)
		if err := freerangenotify.Notify(ctx, frn, freerangenotify.NotifyRequest{
			UserID:   user.NewUsername,
			InAppTpl: constants.FRNTplCoAuthorInviteInApp,
			SSETpl:   constants.FRNTplCoAuthorInviteSSE,
			EmailTpl: constants.FRNTplCoAuthorInviteEmail,
			Priority: "high",
			Category: constants.FRNCategoryCollaboration,
			Data: map[string]interface{}{
				"inviter_name": user.Username,
				"blog_title":   user.BlogTitle,
			},
		}, log); err != nil {
			log.Errorw("FRN co-author invite notification failed", "err", err)
		}

	case constants.CO_AUTHOR_ACCEPT:
		log.Debugf("Received co-author accept: %s accepted invite for blog %s", user.Username, user.BlogId)
		if err := freerangenotify.Notify(ctx, frn, freerangenotify.NotifyRequest{
			UserID:   user.NewUsername,
			InAppTpl: constants.FRNTplCoAuthorAcceptInApp,
			SSETpl:   constants.FRNTplCoAuthorAcceptSSE,
			Priority: "normal",
			Category: constants.FRNCategoryCollaboration,
			Data: map[string]interface{}{
				"coauthor_name": user.Username,
				"blog_title":    user.BlogTitle,
			},
		}, log); err != nil {
			log.Errorw("FRN co-author accept notification failed", "err", err)
		}

	case constants.CO_AUTHOR_DECLINE:
		log.Debugf("Received co-author decline: %s declined invite for blog %s", user.Username, user.BlogId)
		if err := freerangenotify.Notify(ctx, frn, freerangenotify.NotifyRequest{
			UserID:   user.NewUsername,
			InAppTpl: constants.FRNTplCoAuthorDeclineInApp,
			Priority: "normal",
			Category: constants.FRNCategoryCollaboration,
			Data: map[string]interface{}{
				"coauthor_name": user.Username,
				"blog_title":    user.BlogTitle,
			},
		}, log); err != nil {
			log.Errorw("FRN co-author decline notification failed", "err", err)
		}

	case constants.CO_AUTHOR_REMOVED:
		log.Debugf("Received co-author removed: %s removed %s from blog %s", user.Username, user.NewUsername, user.BlogId)
		if err := freerangenotify.Notify(ctx, frn, freerangenotify.NotifyRequest{
			UserID:   user.NewUsername,
			InAppTpl: constants.FRNTplCoAuthorRemovedInApp,
			SSETpl:   constants.FRNTplCoAuthorRemovedSSE,
			Priority: "normal",
			Category: constants.FRNCategoryCollaboration,
			Data: map[string]interface{}{
				"remover_name": user.Username,
				"blog_title":   user.BlogTitle,
			},
		}, log); err != nil {
			log.Errorw("FRN co-author removed notification failed", "err", err)
		}

	case constants.CO_AUTHOR_BLOG_PUBLISHED:
		log.Debugf("Received co-author blog published: %s published blog %s", user.Username, user.BlogId)
		if err := freerangenotify.Notify(ctx, frn, freerangenotify.NotifyRequest{
			UserID:   user.NewUsername,
			InAppTpl: constants.FRNTplBlogPublishedCoAuthorInApp,
			SSETpl:   constants.FRNTplBlogPublishedCoAuthorSSE,
			Priority: "normal",
			Category: constants.FRNCategoryContent,
			Data: map[string]interface{}{
				"publisher_name": user.Username,
				"blog_title":     user.BlogTitle,
			},
		}, log); err != nil {
			log.Errorw("FRN co-author blog published notification failed", "err", err)
		}

	case constants.PASSWORD_CHANGED:
		log.Debugf("Received password changed: %s", user.Username)
		if err := freerangenotify.Notify(ctx, frn, freerangenotify.NotifyRequest{
			UserID:   user.Username,
			InAppTpl: constants.FRNTplPasswordChangedInApp,
			EmailTpl: constants.FRNTplPasswordChangedEmail,
			Priority: "high",
			Category: constants.FRNCategorySecurity,
			Data:     map[string]interface{}{},
		}, log); err != nil {
			log.Errorw("FRN password changed notification failed", "err", err)
		}

	case constants.EMAIL_VERIFIED:
		log.Debugf("Received email verified: %s", user.Username)
		if err := freerangenotify.Notify(ctx, frn, freerangenotify.NotifyRequest{
			UserID:   user.Username,
			InAppTpl: constants.FRNTplEmailVerifiedInApp,
			Priority: "high",
			Category: constants.FRNCategorySecurity,
			Data:     map[string]interface{}{},
		}, log); err != nil {
			log.Errorw("FRN email verified notification failed", "err", err)
		}

	case constants.LOGIN_DETECTED:
		log.Debugf("Received login detected: %s", user.Username)
		if err := freerangenotify.Notify(ctx, frn, freerangenotify.NotifyRequest{
			UserID:   user.Username,
			InAppTpl: constants.FRNTplLoginDetectedInApp,
			SSETpl:   constants.FRNTplLoginDetectedSSE,
			EmailTpl: constants.FRNTplLoginDetectedEmail,
			Priority: "high",
			Category: constants.FRNCategorySecurity,
			Data: map[string]interface{}{
				"ip_address":   user.IpAddress,
				"client":       user.Client,
				"login_method": user.LoginMethod,
			},
		}, log); err != nil {
			log.Errorw("FRN login detected notification failed", "user", user.Username, "err", err)
		}

	case constants.PASSWORD_RESET_REQUESTED:
		log.Debugf("Received password reset requested: %s", user.Username)
		if err := freerangenotify.Notify(ctx, frn, freerangenotify.NotifyRequest{
			UserID:   user.Username,
			InAppTpl: constants.FRNTplPasswordResetReqInApp,
			EmailTpl: constants.FRNTplPasswordResetReqEmail,
			Priority: "high",
			Category: constants.FRNCategorySecurity,
			Data: map[string]interface{}{
				"ip_address": user.IpAddress,
			},
		}, log); err != nil {
			log.Errorw("FRN password reset requested notification failed", "user", user.Username, "err", err)
		}

	case constants.EMAIL_CHANGED:
		log.Debugf("Received email changed: %s", user.Username)
		if err := freerangenotify.Notify(ctx, frn, freerangenotify.NotifyRequest{
			UserID:   user.Username,
			InAppTpl: constants.FRNTplEmailChangedInApp,
			EmailTpl: constants.FRNTplEmailChangedEmail,
			Priority: "high",
			Category: constants.FRNCategorySecurity,
			Data: map[string]interface{}{
				"new_email": user.Email,
			},
		}, log); err != nil {
			log.Errorw("FRN email changed notification failed", "user", user.Username, "err", err)
		}

	case constants.USERNAME_CHANGED:
		log.Debugf("Received username changed: %s → %s", user.Username, user.NewUsername)
		// Send notification under the OLD username (FRN still knows the user by this external_id)
		if err := freerangenotify.Notify(ctx, frn, freerangenotify.NotifyRequest{
			UserID:   user.Username,
			InAppTpl: constants.FRNTplUsernameChangedInApp,
			Priority: "normal",
			Category: constants.FRNCategorySecurity,
			Data: map[string]interface{}{
				"new_username": user.NewUsername,
			},
		}, log); err != nil {
			log.Errorw("FRN username changed notification failed", "user", user.Username, "err", err)
		}
		// Update FRN external_id so future notifications route to the new username
		if err := frn.UpdateUserExternalID(ctx, user.Username, user.NewUsername); err != nil {
			log.Errorw("FRN external_id update failed — user may not receive future notifications",
				"old", user.Username, "new", user.NewUsername, "err", err)
		} else {
			log.Infow("FRN external_id updated", "old", user.Username, "new", user.NewUsername)
		}

	default:
		log.Warnf("Unknown notification action: %s", user.Action)
	}
}
