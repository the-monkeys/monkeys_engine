# FreeRangeNotify Integration Plan — The Monkeys Engine

> **Scope:** Replace the existing PostgreSQL-backed notification service with FreeRangeNotify (FRN) for in-app, SSE, and email notifications.
> **Channels:** `in_app` (persistent inbox), `sse` (real-time push), `email` (transactional — not newsletter).
> **FRN runs on localhost** — backend at `http://localhost:8080`, frontend at `http://localhost:3000`.

---

## Table of Contents

1. [Architecture Overview](#1-architecture-overview)
2. [Environment Variables (.env)](#2-environment-variables)
3. [Event Audit — Current State vs. Target State](#3-event-audit)
4. [Phase 1: Foundation — FRN Client, Config, Constants](#4-phase-1-foundation)
5. [Phase 2: User Lifecycle — Register & Sync](#5-phase-2-user-lifecycle)
6. [Phase 3: Retarget the Notification Consumer](#6-phase-3-retarget-consumer)
7. [Phase 4: Add Missing Event Publishes](#7-phase-4-add-missing-publishes)
8. [Phase 5: Email Channel](#8-phase-5-email-channel)
9. [Phase 6: Gateway — SSE Token Endpoint](#9-phase-6-gateway-sse-token)
10. [Phase 7: Frontend Integration](#10-phase-7-frontend)
11. [Phase 8: Clean Up Legacy Code](#11-phase-8-clean-up)
12. [FRN Template Registry](#12-frn-template-registry)
13. [File-by-File Change Map](#13-file-change-map)

---

## 1. Architecture Overview

### Approach: RabbitMQ Consumer → FRN Adapter

Services publish events to `to_notification_svc_queue` (RoutingKeys[4]) — the same pattern already used for `USER_FOLLOWED`, `BLOG_LIKE`, and `USER_REGISTER`. The notification consumer reads from the queue and calls FRN's REST API instead of writing to PostgreSQL.

```
┌──────────────────────────────────────────────────────────────────────────┐
│                          The Monkeys Engine                              │
│                                                                          │
│  ┌─────────────┐     gRPC     ┌──────────────┐                          │
│  │   Gateway    │────────────▶│  Users Svc   │──┐                       │
│  │  (Gin:8081)  │             │  (gRPC:50053)│  │                       │
│  │              │     gRPC     ├──────────────┤  │  PublishMessage       │
│  │              │────────────▶│  Authz Svc   │──┤  RoutingKeys[4]       │
│  │              │             │  (gRPC:50051)│  │                       │
│  │              │     gRPC     ├──────────────┤  │                       │
│  │              │────────────▶│  Blog Svc    │──┘                       │
│  │              │             │  (gRPC:50052)│                           │
│  └──────┬───────┘             └──────────────┘                           │
│         │                            │                                   │
│         │ GET /sse-token             ▼                                   │
│         │                     ┌─────────────┐                            │
│         │                     │  RabbitMQ    │                            │
│         │                     │  Exchange:   │                            │
│         │                     │ smart_monkey │                            │
│         │                     └──────┬──────┘                            │
│         │                            │                                   │
│         │                            │ to_notification_svc_queue          │
│         │                            ▼                                   │
│         │                     ┌──────────────────────┐                   │
│         │                     │  Notification Svc    │                   │
│         │                     │  (Consumer + gRPC)   │                   │
│         │                     │  Port: 50055         │                   │
│         │                     │                      │                   │
│         │                     │  handleUserAction()  │                   │
│         │                     │   ├─ USER_REGISTER   │                   │
│         │                     │   ├─ USER_FOLLOWED   │  HTTP POST        │
│         │                     │   ├─ BLOG_LIKE       │───────────┐       │
│         │                     │   ├─ CO_AUTHOR_*     │           │       │
│         │                     │   ├─ PASSWORD_CHANGE │           │       │
│         │                     │   └─ EMAIL_VERIFIED  │           │       │
│         │                     └──────────────────────┘           │       │
│         │                                                        │       │
│         │                                                        ▼       │
│  ┌──────┴────────────────────────────────────────────────────────────┐   │
│  │                  FreeRangeNotify (localhost)                       │   │
│  │  ┌──────────────────┐    ┌──────────────┐   ┌────────────┐       │   │
│  │  │ Notification Svc │    │    Worker     │   │     UI     │       │   │
│  │  │  (REST :8080)    │    │  (async jobs) │   │  (:3000)   │       │   │
│  │  └────────┬─────────┘    └──────┬───────┘   └────────────┘       │   │
│  │           │                      │                                │   │
│  │  ┌────────▼──────────────────────▼───────┐                        │   │
│  │  │  Elasticsearch    │     Redis          │                        │   │
│  │  │  (FRN :9200)      │     (FRN :6379)    │                        │   │
│  │  └───────────────────┴────────────────────┘                        │   │
│  └───────────────────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────────────────┘
```

### Why this approach

| Concern | How it's handled |
|---|---|
| **Event ownership** | Each service (Users, Authz, Blog) publishes the event where it happens — they already have all the context (blog owner, co-authors, etc.). No proto changes needed. |
| **Decoupling** | Services don't know FRN exists. They publish a `TheMonkeysMessage` to RabbitMQ. The consumer is the only component with an FRN dependency. |
| **Resilience** | If FRN is down, messages sit in `to_notification_svc_queue` until it recovers. Fire-and-forget HTTP from the gateway would lose notifications silently. |
| **Existing pattern** | `FollowUser`, `LikeBlog`, and `RegisterUser` already publish to RoutingKeys[4]. We extend the pattern, not replace it. |
| **Single integration point** | Only the notification consumer needs the FRN HTTP client. One service to configure, test, and deploy. |

### What changes

1. **Services (Users, Authz, Blog):** Add `PublishMessage` calls for missing events (co-author invite, password change, etc.) — same 5-line pattern already used.
2. **Notification consumer:** Replace `db.CreateNotification()` (PostgreSQL) with FRN HTTP calls. Add new `case` branches for new events.
3. **Gateway:** Add one new endpoint (`GET /api/v1/notifications/sse-token`) so the frontend can get an SSE token. No notification sending from gateway.
4. **TheMonkeysMessage model:** Extend with fields for richer notification data (blog title, co-author lists).

---

## 2. Environment Variables

All FRN credentials and URLs go in `.env` — never hardcoded.

### New variables to add to `dev.env`:

```bash
# ============================================
# FreeRangeNotify Integration
# ============================================

# FRN Backend API (running via docker-compose in FreeRangeNotify repo)
FRN_BASE_URL=http://localhost:8080/v1
FRN_API_KEY=frn_live_xxxxxxxxxxxxx

# FRN Application ID (created once via FRN dashboard/API)
FRN_APP_ID=app-monkeys-xxx

# FRN Email channel (optional — enable when SMTP is configured in FRN)
FRN_EMAIL_ENABLED=false

# FRN SSE public URL (what the browser connects to — goes through gateway proxy)
FRN_SSE_PUBLIC_URL=http://localhost:8080/v1
```

### Config struct additions (`config/config.go`):

```go
type FreeRangeNotify struct {
    BaseURL      string `mapstructure:"FRN_BASE_URL"`
    APIKey       string `mapstructure:"FRN_API_KEY"`
    AppID        string `mapstructure:"FRN_APP_ID"`
    EmailEnabled bool   `mapstructure:"FRN_EMAIL_ENABLED"`
    SSEPublicURL string `mapstructure:"FRN_SSE_PUBLIC_URL"`
}
```

Add `FreeRangeNotify FreeRangeNotify` field to the top-level `Config` struct.

---

## 3. Event Audit — Current State vs. Target State

### Currently Implemented (old notification service — PostgreSQL)

| Event | Source Service | RabbitMQ Queue | Notification Created |
|---|---|---|---|
| User registered | Authz → RoutingKeys[4] | `to_notification_svc_queue` | ✅ `AccountCreated` (Browser) |
| User followed | Users → RoutingKeys[4] | `to_notification_svc_queue` | ✅ `NewFollower` (Browser) |
| Blog liked | Users → RoutingKeys[4] | `to_notification_svc_queue` | ✅ `BlogLiked` (Browser) |

### NOT Implemented (gaps in current code)

| Event | Where It Should Trigger | Current State |
|---|---|---|
| Comment on blog | No comment handler exists yet | ❌ No route, no handler |
| Co-author invitation sent | `users/services/service.go:InviteCoAuthor` L541 | ❌ Activity logged only, no notification publish |
| Co-author invite accepted | `users/services/service.go` | ❌ No `JoinedAsCoAuthor` handler found |
| Co-author invite declined | Not implemented | ❌ No handler |
| Removed from co-author | `users/services/service.go:RevokeCoAuthorAccess` L570 | ❌ Activity logged only, no notification publish |
| Co-authored blog published | Blog publish flow | ❌ No co-author notification |
| Password changed | `authz/services/services.go:UpdatePassword` L825 | ❌ Activity tracked only |
| Email verified | `authz/services/services.go:VerifyEmail` L914 | ❌ Activity tracked only |

### Target State with FRN

| # | Event | Channels | Category | Priority | Template (in_app) | Template (sse) | Template (email) |
|---|---|---|---|---|---|---|---|
| 1 | Someone followed you | `in_app` + `sse` | social | normal | `new_follower_inapp` | `new_follower_sse` | — |
| 2 | Comment on your blog | `in_app` + `sse` | social | normal | `new_comment_inapp` | `new_comment_sse` | — |
| 3 | Someone liked your blog | `in_app` | social | low | `blog_liked_inapp` | — | — |
| 4 | Invited as co-author | `in_app` + `sse` + `email` | collaboration | high | `coauthor_invite_inapp` | `coauthor_invite_sse` | `coauthor_invite_email` |
| 5 | Co-author accepted | `in_app` + `sse` | collaboration | normal | `coauthor_accept_inapp` | `coauthor_accept_sse` | — |
| 6 | Co-author declined | `in_app` | collaboration | normal | `coauthor_decline_inapp` | — | — |
| 7 | Removed as co-author | `in_app` + `sse` | collaboration | normal | `coauthor_removed_inapp` | `coauthor_removed_sse` | — |
| 8 | Co-authored blog published | `in_app` + `sse` | content | normal | `blog_published_coauthor_inapp` | `blog_published_coauthor_sse` | — |
| 9 | Email verified | `in_app` | security | high | `email_verified_inapp` | — | — |
| 10 | Password changed | `in_app` + `email` | security | high | `password_changed_inapp` | — | `password_changed_email` |

---

## 4. Phase 1: Foundation — FRN Client, Config, Constants

### 4.1 Create FRN HTTP Client

**New file:** `microservices/the_monkeys_notification/internal/freerangenotify/client.go`

This lives inside the notification service — the only component that talks to FRN. Lightweight HTTP client wrapping FRN's REST API.

```go
package freerangenotify

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    "go.uber.org/zap"
)

type Client struct {
    baseURL    string
    apiKey     string
    appID      string
    httpClient *http.Client
    log        *zap.SugaredLogger
}

func NewClient(baseURL, apiKey, appID string, log *zap.SugaredLogger) *Client {
    return &Client{
        baseURL: baseURL,
        apiKey:  apiKey,
        appID:   appID,
        httpClient: &http.Client{
            Timeout: 5 * time.Second,
        },
        log: log,
    }
}

type SendRequest struct {
    UserID     string                 `json:"user_id"`
    Channel    string                 `json:"channel"`
    Priority   string                 `json:"priority"`
    TemplateID string                 `json:"template_id"`
    Category   string                 `json:"category"`
    Data       map[string]interface{} `json:"data"`
}

// Send dispatches a single notification to FRN.
func (c *Client) Send(ctx context.Context, req SendRequest) error {
    body, err := json.Marshal(req)
    if err != nil {
        return fmt.Errorf("marshal FRN request: %w", err)
    }

    httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
        c.baseURL+"/notifications/", bytes.NewReader(body))
    if err != nil {
        return fmt.Errorf("create FRN HTTP request: %w", err)
    }
    httpReq.Header.Set("X-API-Key", c.apiKey)
    httpReq.Header.Set("Content-Type", "application/json")

    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return fmt.Errorf("FRN HTTP call failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 300 {
        return fmt.Errorf("FRN returned status %d", resp.StatusCode)
    }
    return nil
}

// RegisterUser registers a Monkeys user in FRN.
func (c *Client) RegisterUser(ctx context.Context, email, username string) error {
    payload := map[string]string{
        "email":   email,
        "user_id": username,
    }
    body, _ := json.Marshal(payload)

    httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
        c.baseURL+"/users/", bytes.NewReader(body))
    if err != nil {
        return fmt.Errorf("create FRN register request: %w", err)
    }
    httpReq.Header.Set("X-API-Key", c.apiKey)
    httpReq.Header.Set("Content-Type", "application/json")

    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return fmt.Errorf("FRN register user failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 300 {
        return fmt.Errorf("FRN register user returned status %d", resp.StatusCode)
    }
    return nil
}

// GetSSEToken gets a short-lived SSE token for a user.
func (c *Client) GetSSEToken(ctx context.Context, username string) (string, error) {
    payload := map[string]string{"user_id": username}
    body, _ := json.Marshal(payload)

    httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
        c.baseURL+"/sse/tokens", bytes.NewReader(body))
    if err != nil {
        return "", fmt.Errorf("create FRN SSE token request: %w", err)
    }
    httpReq.Header.Set("X-API-Key", c.apiKey)
    httpReq.Header.Set("Content-Type", "application/json")

    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return "", fmt.Errorf("FRN SSE token request failed: %w", err)
    }
    defer resp.Body.Close()

    var result struct {
        SSEToken string `json:"sse_token"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return "", fmt.Errorf("decode FRN SSE token response: %w", err)
    }
    return result.SSEToken, nil
}
```

### 4.2 Create Notification Helper

**New file:** `microservices/the_monkeys_notification/internal/freerangenotify/notify.go`

High-level helper that sends `in_app` + `sse` (and optionally `email`) in one call. Used by the consumer's `handleUserAction`.

```go
package freerangenotify

import (
    "context"

    "go.uber.org/zap"
)

type NotifyRequest struct {
    UserID   string                 // Monkeys username (FRN external_id)
    InAppTpl string                 // Template name for in_app channel
    SSETpl   string                 // Template name for sse channel (empty = skip)
    EmailTpl string                 // Template name for email channel (empty = skip)
    Priority string                 // "low", "normal", "high", "critical"
    Category string                 // "social", "collaboration", "content", "security"
    Data     map[string]interface{} // Template variables
}

// Notify sends in_app (always), then SSE and email if templates are provided.
// SSE and email are fire-and-forget — failure does not propagate.
func Notify(ctx context.Context, client *Client, req NotifyRequest, log *zap.SugaredLogger) error {
    // in_app — always sent, error propagated
    if err := client.Send(ctx, SendRequest{
        UserID:     req.UserID,
        Channel:    "in_app",
        Priority:   req.Priority,
        TemplateID: req.InAppTpl,
        Category:   req.Category,
        Data:       req.Data,
    }); err != nil {
        log.Errorw("FRN in_app notification failed", "user", req.UserID, "tpl", req.InAppTpl, "err", err)
        return err
    }

    // SSE — fire-and-forget
    if req.SSETpl != "" {
        if err := client.Send(ctx, SendRequest{
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
        if err := client.Send(ctx, SendRequest{
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
```

### 4.3 Constants — FRN Template Names

**New file:** `constants/frn_templates.go`

```go
package constants

// FRN Template names — registered once in FreeRangeNotify, referenced by name.
const (
    // Social
    FRNTplNewFollowerInApp = "new_follower_inapp"
    FRNTplNewFollowerSSE   = "new_follower_sse"
    FRNTplNewCommentInApp  = "new_comment_inapp"
    FRNTplNewCommentSSE    = "new_comment_sse"
    FRNTplBlogLikedInApp   = "blog_liked_inapp"

    // Collaboration
    FRNTplCoAuthorInviteInApp  = "coauthor_invite_inapp"
    FRNTplCoAuthorInviteSSE    = "coauthor_invite_sse"
    FRNTplCoAuthorInviteEmail  = "coauthor_invite_email"
    FRNTplCoAuthorAcceptInApp  = "coauthor_accept_inapp"
    FRNTplCoAuthorAcceptSSE    = "coauthor_accept_sse"
    FRNTplCoAuthorDeclineInApp = "coauthor_decline_inapp"
    FRNTplCoAuthorRemovedInApp = "coauthor_removed_inapp"
    FRNTplCoAuthorRemovedSSE   = "coauthor_removed_sse"

    // Content
    FRNTplBlogPublishedCoAuthorInApp = "blog_published_coauthor_inapp"
    FRNTplBlogPublishedCoAuthorSSE   = "blog_published_coauthor_sse"

    // Security
    FRNTplPasswordChangedInApp  = "password_changed_inapp"
    FRNTplPasswordChangedEmail  = "password_changed_email"
    FRNTplEmailVerifiedInApp    = "email_verified_inapp"
)

// FRN Categories
const (
    FRNCategorySocial        = "social"
    FRNCategoryCollaboration = "collaboration"
    FRNCategoryContent       = "content"
    FRNCategorySecurity      = "security"
)
```

---

## 5. Phase 2: User Lifecycle — Register & Sync

### Where: Authz Service — `RegisterUser` method

**File:** `microservices/the_monkeys_authz/internal/services/services.go` (~L343)

**Current behavior:** Publishes `USER_REGISTER` to RoutingKeys[0] (user svc) and RoutingKeys[4] (notification svc).

The `USER_REGISTER` message already reaches the notification consumer. The consumer currently creates a PostgreSQL row. After the rewrite (Phase 3), it will instead:

1. **Register the user in FRN** via `POST /v1/users/` (so FRN knows about the user for future notifications and SSE).
2. **Send a welcome notification** via `POST /v1/notifications/` using the `email_verified_inapp` template (or a dedicated `welcome_inapp` template).

**No changes to the Authz service.** The existing publish handles it. The consumer does the FRN calls.

---

## 6. Phase 3: Retarget the Notification Consumer → FRN

This is the core change. Rewrite `handleUserAction()` in `microservices/the_monkeys_notification/internal/consumer/consumer.go` to call FRN instead of PostgreSQL.

### 6.1 Current consumer code (what we're replacing)

```go
// consumer.go — handleUserAction (BEFORE)
func handleUserAction(user models.TheMonkeysMessage, log *zap.SugaredLogger, db database.NotificationDB) {
    switch user.Action {
    case constants.USER_REGISTER:
        err := db.CreateNotification(user.AccountId, constants.AccountCreated, "Welcome to The Monkeys!", user.BlogId, user.AccountId, "Browser")
    case constants.BLOG_LIKE:
        err := db.CreateNotification(user.AccountId, constants.BlogLiked, user.Notification, user.BlogId, user.AccountId, "Browser")
    case constants.USER_FOLLOWED:
        dbUser, _ := db.CheckIfUsernameExist(user.NewUsername)
        err = db.CreateNotification(dbUser.AccountId, constants.NewFollower, user.Notification, user.BlogId, user.AccountId, "Browser")
    }
}
```

### 6.2 New consumer code (calling FRN)

The function signature changes to accept `*freerangenotify.Client` instead of (or alongside) `database.NotificationDB`:

```go
// consumer.go — handleUserAction (AFTER)
func handleUserAction(user models.TheMonkeysMessage, log *zap.SugaredLogger, frn *freerangenotify.Client) {
    ctx := context.Background()

    switch user.Action {

    case constants.USER_REGISTER:
        log.Debugf("Received user registration: %s", user.Username)
        // Step 1: Register user in FRN so it knows about them
        if err := frn.RegisterUser(ctx, user.Email, user.Username); err != nil {
            log.Errorw("FRN user registration failed", "user", user.Username, "err", err)
            // Don't return — still try to send the welcome notification
        }
        // Step 2: Send welcome notification (in_app only)
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
        // user.Username = follower, user.NewUsername = person being followed
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
        // user.Username = blog owner (set by Users service), user.Notification has the message
        if err := freerangenotify.Notify(ctx, frn, freerangenotify.NotifyRequest{
            UserID:   user.Username,
            InAppTpl: constants.FRNTplBlogLikedInApp,
            SSETpl:   "",  // likes = low priority, no SSE
            Priority: "low",
            Category: constants.FRNCategorySocial,
            Data: map[string]interface{}{
                "liker_name": user.Username,
                "blog_title": user.BlogId,  // TODO: resolve to actual title
            },
        }, log); err != nil {
            log.Errorw("FRN like notification failed", "user", user.Username, "err", err)
        }

    // ──────────────────────────────────────────────────
    // NEW events (Phase 4 adds the PublishMessage calls)
    // ──────────────────────────────────────────────────

    case constants.CO_AUTHOR_INVITE:
        log.Debugf("Received co-author invite: %s invited %s for blog %s", user.Username, user.NewUsername, user.BlogId)
        if err := freerangenotify.Notify(ctx, frn, freerangenotify.NotifyRequest{
            UserID:   user.NewUsername,  // the invitee
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
            UserID:   user.NewUsername,  // the blog owner (notified that someone accepted)
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
            UserID:   user.NewUsername,  // the blog owner
            InAppTpl: constants.FRNTplCoAuthorDeclineInApp,
            SSETpl:   "",  // decline = in_app only
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
            UserID:   user.NewUsername,  // the person being removed
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
        // user.NewUsername = recipient co-author (publisher sends one message per co-author)
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

    default:
        log.Warnf("Unknown notification action: %s", user.Action)
    }
}
```

### 6.3 Update `ConsumeFromQueue` signature

Pass the FRN client instead of (or alongside) the database:

```go
// consumer.go
func ConsumeFromQueue(mgr *rabbitmq.ConnManager, conf config.RabbitMQ, log *zap.SugaredLogger, frn *freerangenotify.Client) {
    // ... same signal handling, same consumeQueue call
    go consumeQueue(mgr, conf.Queues[4], log, frn)
    select {}
}

func consumeQueue(mgr *rabbitmq.ConnManager, queueName string, log *zap.SugaredLogger, frn *freerangenotify.Client) {
    // ... same reconnection logic, same message loop
    for d := range msgs {
        user := models.TheMonkeysMessage{}
        if err := json.Unmarshal(d.Body, &user); err != nil {
            log.Errorf("Failed to unmarshal from RabbitMQ: %v", err)
            continue
        }
        handleUserAction(user, log, frn)
    }
}
```

### 6.4 Update `main.go` to wire FRN client

```go
// microservices/the_monkeys_notification/main.go

func main() {
    cfg, err := config.GetConfig()
    // ...

    // Initialize FRN client (replaces PostgreSQL connection for notification sending)
    frn := freerangenotify.NewClient(
        cfg.FreeRangeNotify.BaseURL,
        cfg.FreeRangeNotify.APIKey,
        cfg.FreeRangeNotify.AppID,
        log,
    )

    // Connect to rabbitmq server
    qConn := rabbitmq.NewConnManager(cfg.RabbitMQ)
    go consumer.ConsumeFromQueue(qConn, cfg.RabbitMQ, log, frn)

    // gRPC server remains for gateway to query/mark notifications
    // (gateway calls GetNotification, NotificationSeen — these now proxy to FRN)
    // ...
}
```

### 6.5 What happens to the gRPC service?

The gateway currently calls `GetNotification`, `NotificationSeen`, and `GetNotificationStream` via gRPC. These query PostgreSQL.

**Two options:**
1. **Remove the gRPC service entirely** — the frontend uses FRN's React SDK for inbox/mark-read, and the gateway's only new endpoint is `GET /sse-token`. The old WebSocket notification routes are removed.
2. **Keep the gRPC service as a proxy** — rewrite the handlers to call FRN's REST API (`GET /v1/notifications?user_id=...`, `POST /v1/notifications/{id}/read`).

**Recommendation:** Option 1. The FRN React SDK handles inbox fetching, mark-read, and SSE directly. No need for the gateway to proxy these operations. The gRPC service can be removed after the frontend switches to the SDK.

### 6.6 Extend `TheMonkeysMessage` model

The current model lacks fields needed for rich notifications. Add:

```go
// microservices/the_monkeys_notification/internal/models/models.go
type TheMonkeysMessage struct {
    Id           int64  `json:"id"`
    AccountId    string `json:"account_id"`
    Username     string `json:"username"`        // Actor (follower, liker, inviter, etc.)
    NewUsername  string `json:"new_username"`     // Recipient (followed user, invitee, etc.)
    Email        string `json:"email"`
    LoginMethod  string `json:"login_method"`
    ClientId     string `json:"client_id"`
    Client       string `json:"client"`
    IpAddress    string `json:"ip"`
    Action       string `json:"action"`
    Notification string `json:"notification"`
    BlogId       string `json:"blog_id"`
    BlogStatus   string `json:"blog_status"`
    BlogTitle    string `json:"blog_title"`       // NEW — for notification templates
}
```

**Important:** This model is also used by the Users service and Authz service when marshaling messages. The new `BlogTitle` field is optional (zero-value `""` if not set), so existing publishes continue to work without changes.

---

## 7. Phase 4: Add Missing Event Publishes in Services

These events currently have NO RabbitMQ publish to `RoutingKeys[4]`. We add them in the service where the event occurs — same 5-line `json.Marshal` + `PublishMessage` pattern already used for follow/like.

### 7.1 New Action Constants

**File:** `constants/actions.go` — add:

```go
const (
    // ... existing constants ...

    // Notification-specific actions (published to RoutingKeys[4])
    CO_AUTHOR_INVITE         = "co_author_invite"
    CO_AUTHOR_ACCEPT         = "co_author_accept"
    CO_AUTHOR_DECLINE        = "co_author_decline"
    CO_AUTHOR_REMOVED        = "co_author_removed"
    CO_AUTHOR_BLOG_PUBLISHED = "co_author_blog_published"
    PASSWORD_CHANGED         = "password_changed"
    EMAIL_VERIFIED           = "email_verified"
    BLOG_COMMENT             = "blog_comment"
)
```

### 7.2 Co-Author Invitation

**File:** `microservices/the_monkeys_users/internal/services/service.go` — `InviteCoAuthor` method (L541)

**Current code** (L563): Only logs activity, no RabbitMQ publish.

**Add after** `AddPermissionToAUser` succeeds (after L555):

```go
// Publish notification event for co-author invite
bx, err := json.Marshal(models.TheMonkeysMessage{
    AccountId:   resp.AccountId,           // invitee's account ID
    Username:    req.BlogOwnerUsername,     // the inviter
    NewUsername: req.Username,              // the invitee
    Action:      constants.CO_AUTHOR_INVITE,
    BlogId:      req.BlogId,
    BlogTitle:   "",                        // TODO: resolve blog title from blog service
    Notification: fmt.Sprintf("%s invited you as a co-author", req.BlogOwnerUsername),
})
if err != nil {
    us.log.Errorf("failed to marshal co-author invite message: %v", err)
}

go func() {
    if err := us.qConn.PublishMessage(us.config.RabbitMQ.Exchange, us.config.RabbitMQ.RoutingKeys[4], bx); err != nil {
        us.log.Errorf("failed to publish co-author invite notification: %v", err)
    }
}()
```

### 7.3 Co-Author Removed

**File:** `microservices/the_monkeys_users/internal/services/service.go` — `RevokeCoAuthorAccess` method (L570)

**Current code** (L594): Only logs activity, no publish.

**Add after** `RevokeBlogPermissionFromAUser` succeeds (after L585):

```go
// Publish notification event for co-author removal
bx, err := json.Marshal(models.TheMonkeysMessage{
    Username:    req.BlogOwnerUsername,     // the remover
    NewUsername: req.Username,              // the person being removed
    Action:      constants.CO_AUTHOR_REMOVED,
    BlogId:      req.BlogId,
    Notification: fmt.Sprintf("You have been removed as co-author from blog %s", req.BlogId),
})
if err != nil {
    us.log.Errorf("failed to marshal co-author removed message: %v", err)
}

go func() {
    if err := us.qConn.PublishMessage(us.config.RabbitMQ.Exchange, us.config.RabbitMQ.RoutingKeys[4], bx); err != nil {
        us.log.Errorf("failed to publish co-author removal notification: %v", err)
    }
}()
```

### 7.4 Password Changed

**File:** `microservices/the_monkeys_authz/internal/services/services.go` — `UpdatePassword` method (~L825)

**Current code** (~L855): Tracks activity, no notification publish.

**Add after** `UpdatePassword` DB call succeeds (after L849):

```go
// Publish notification event for password change
bx, err := json.Marshal(models.TheMonkeysMessage{
    Username:     user.Username,
    AccountId:    user.AccountId,
    Action:       constants.PASSWORD_CHANGED,
    Notification: "Your password was changed",
})
if err != nil {
    as.logger.Errorf("failed to marshal password changed message: %v", err)
}

go func() {
    if err := as.qConn.PublishMessage(as.config.RabbitMQ.Exchange, as.config.RabbitMQ.RoutingKeys[4], bx); err != nil {
        as.logger.Errorf("failed to publish password changed notification: %v", err)
    }
}()
```

### 7.5 Email Verified

**File:** `microservices/the_monkeys_authz/internal/services/services.go` — `VerifyEmail` method (~L914)

**Current code** (~L944): Tracks activity, no notification publish.

**Add after** `UpdateEmailVerificationStatus` succeeds (after L940):

```go
// Publish notification event for email verification
bx, err := json.Marshal(models.TheMonkeysMessage{
    Username:     user.Username,
    AccountId:    user.AccountId,
    Email:        user.Email,
    Action:       constants.EMAIL_VERIFIED,
    Notification: "Your email has been verified",
})
if err != nil {
    as.logger.Errorf("failed to marshal email verified message: %v", err)
}

go func() {
    if err := as.qConn.PublishMessage(as.config.RabbitMQ.Exchange, as.config.RabbitMQ.RoutingKeys[4], bx); err != nil {
        as.logger.Errorf("failed to publish email verified notification: %v", err)
    }
}()
```

### 7.6 Co-Author Accepted / Declined

**Status:** `JoinedAsCoAuthor` and `DeclinedCoAuthor` handlers do **not exist** in the codebase. The activity constants exist but no gRPC methods handle these.

**Prerequisite (future work):**

1. **Proto definition:** Add `AcceptCoAuthorInvite` and `DeclineCoAuthorInvite` RPCs in the Users service proto.
2. **Users service:** Implement handlers that update the permission status.
3. **Gateway routes:** Add `POST /api/v1/user/accept-invite/:blog_id` and `POST /api/v1/user/decline-invite/:blog_id`.
4. **Publish:** After accept/decline succeeds, publish to RoutingKeys[4]:

```go
// On accept
bx, _ := json.Marshal(models.TheMonkeysMessage{
    Username:    acceptingUsername,     // the co-author who accepted
    NewUsername: blogOwnerUsername,     // the blog owner to notify
    Action:      constants.CO_AUTHOR_ACCEPT,
    BlogId:      blogId,
    BlogTitle:   blogTitle,
})
us.qConn.PublishMessage(us.config.RabbitMQ.Exchange, us.config.RabbitMQ.RoutingKeys[4], bx)

// On decline
bx, _ := json.Marshal(models.TheMonkeysMessage{
    Username:    decliningUsername,
    NewUsername: blogOwnerUsername,
    Action:      constants.CO_AUTHOR_DECLINE,
    BlogId:      blogId,
    BlogTitle:   blogTitle,
})
us.qConn.PublishMessage(us.config.RabbitMQ.Exchange, us.config.RabbitMQ.RoutingKeys[4], bx)
```

The consumer already has the `case` branches (Phase 3) — they just need these publishes to start arriving.

### 7.7 Co-Authored Blog Published

**Where:** Blog service — after `PublishBlog` succeeds.

**Prerequisite:** A way to look up co-authors for a blog. Options:
- Add `GetBlogCoAuthors(blogId)` query to the Users service DB.
- Add the co-author list to the `PublishBlog` gRPC response.

**Implementation:** The Blog service (or Users service — whoever knows the co-authors) publishes **one message per co-author** to RoutingKeys[4]:

```go
// After blog publish succeeds, for each co-author:
for _, coAuthor := range coAuthors {
    if coAuthor == publisherUsername {
        continue
    }
    bx, _ := json.Marshal(models.TheMonkeysMessage{
        Username:    publisherUsername,
        NewUsername: coAuthor,
        Action:      constants.CO_AUTHOR_BLOG_PUBLISHED,
        BlogId:      blogId,
        BlogTitle:   blogTitle,
    })
    go func() {
        us.qConn.PublishMessage(us.config.RabbitMQ.Exchange, us.config.RabbitMQ.RoutingKeys[4], bx)
    }()
}
```

### 7.8 Comment on Blog

**Status:** No comment handler/route exists in the codebase.

**Prerequisite:** Implement the full blog commenting feature first:
1. Add `POST /api/v1/blog/:blog_id/comment` gateway route.
2. Add gRPC `CommentOnBlog` RPC to Blog or Users service.
3. After comment succeeds, publish to RoutingKeys[4]:

```go
if commenterUsername != blogAuthorUsername { // don't self-notify
    bx, _ := json.Marshal(models.TheMonkeysMessage{
        Username:     commenterUsername,
        NewUsername:  blogAuthorUsername,
        Action:       constants.BLOG_COMMENT,
        BlogId:       blogId,
        BlogTitle:    blogTitle,
        Notification: fmt.Sprintf("%s commented on your blog", commenterUsername),
    })
    us.qConn.PublishMessage(us.config.RabbitMQ.Exchange, us.config.RabbitMQ.RoutingKeys[4], bx)
}
```

Add the matching `case constants.BLOG_COMMENT:` in the consumer when this feature is built.

---

## 8. Phase 5: Email Channel

Email notifications are sent for high-priority events only: **co-author invite** and **password changed**.

### FRN Email Templates

Create these via the FRN API (one-time setup):

```bash
# Co-author invite email
curl -X POST http://localhost:8080/v1/templates/ \
  -H "X-API-Key: $FRN_API_KEY" \
  -d '{
    "app_id": "'$FRN_APP_ID'",
    "name": "coauthor_invite_email",
    "channel": "email",
    "subject": "{{.inviter_name}} invited you to co-author a blog on Monkeys",
    "body": "Hi! {{.inviter_name}} has invited you to collaborate on \"{{.blog_title}}\". Log in to accept or decline.",
    "variables": ["inviter_name", "blog_title"],
    "locale": "en",
    "metadata": { "category": "collaboration" }
  }'

# Password changed email
curl -X POST http://localhost:8080/v1/templates/ \
  -H "X-API-Key: $FRN_API_KEY" \
  -d '{
    "app_id": "'$FRN_APP_ID'",
    "name": "password_changed_email",
    "channel": "email",
    "subject": "Your Monkeys password was changed",
    "body": "Your account password was updated. If this was not you, contact support immediately.",
    "variables": [],
    "locale": "en",
    "metadata": { "category": "security" }
  }'
```

No code change needed — the consumer's `Notify()` call already sends email when `EmailTpl` is non-empty. The templates just need to exist in FRN.

---

## 9. Phase 6: Gateway — SSE Token Endpoint

The gateway needs **one new endpoint** that the frontend calls to get a short-lived SSE token. The FRN API key never leaves the backend. This is the only FRN-related code in the gateway.

### New Gateway Route

**New file:** `microservices/the_monkeys_gateway/internal/freerangenotify/routes.go`

```go
package freerangenotify

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/internal/auth"
    "go.uber.org/zap"
)

func RegisterRoutes(router *gin.Engine, client *Client, authClient *auth.ServiceClient, log *zap.SugaredLogger) {
    mware := auth.InitAuthMiddleware(authClient, log)

    routes := router.Group("/api/v1/notifications")
    routes.Use(mware.AuthRequired)

    routes.GET("/sse-token", func(ctx *gin.Context) {
        userName := ctx.GetString("userName")
        if userName == "" {
            ctx.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
            return
        }

        sseToken, err := client.GetSSEToken(ctx.Request.Context(), userName)
        if err != nil {
            log.Errorw("failed to get FRN SSE token", "user", userName, "err", err)
            ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get SSE token"})
            return
        }

        ctx.JSON(http.StatusOK, gin.H{
            "sse_token":    sseToken,
            "user_id":      userName,
            "sse_base_url": client.baseURL,
        })
    })
}
```

**Note:** The gateway also needs a lightweight FRN client — just for `GetSSEToken()`. It does NOT call `Send()` or `RegisterUser()` — only the notification consumer does that.

**New file:** `microservices/the_monkeys_gateway/internal/freerangenotify/client.go`

This is a minimal client with only the `GetSSEToken` method. Alternatively, import the same client package from the notification service if your module structure allows it.

### Gateway `main.go` Wiring

In `microservices/the_monkeys_gateway/main.go`, add:

```go
// Initialize FRN client (gateway only needs SSE token functionality)
frnClient := freerangenotify.NewClient(
    cfg.FreeRangeNotify.BaseURL,
    cfg.FreeRangeNotify.APIKey,
    cfg.FreeRangeNotify.AppID,
    log,
)

// Register FRN SSE token route
freerangenotify.RegisterRoutes(router, frnClient, authClient, log)
```

---

## 10. Phase 7: Frontend Integration

### Install the React SDK

```bash
npm install @freerangenotify/react
```

### Provider Setup

```tsx
// components/NotificationProvider.tsx
import { FreeRangeProvider } from '@freerangenotify/react';

export function NotificationProvider({ children }) {
  const { user } = useAuth();
  const [sseToken, setSseToken] = useState(null);
  const [frnUserId, setFrnUserId] = useState(null);

  useEffect(() => {
    if (!user) return;
    fetch('/api/v1/notifications/sse-token', {
      headers: { Authorization: `Bearer ${accessToken}` },
    })
      .then(r => r.json())
      .then(({ sse_token, user_id, sse_base_url }) => {
        setSseToken(sse_token);
        setFrnUserId(user_id);
      });
  }, [user]);

  if (!sseToken || !frnUserId) return <>{children}</>;

  return (
    <FreeRangeProvider
      apiKey={sseToken}
      userId={frnUserId}
      apiBaseURL={process.env.NEXT_PUBLIC_FRN_SSE_URL || "http://localhost:8080/v1"}
    >
      {children}
    </FreeRangeProvider>
  );
}
```

### Notification Bell

```tsx
import { NotificationBell } from '@freerangenotify/react';

<NotificationBell
  tabs={['All', 'Social', 'Collaboration', 'Security']}
  onNotification={(n) => toast.info(n.title)}
/>
```

---

## 11. Phase 8: Clean Up Legacy Code

Once FRN is fully wired and tested:

1. **Remove PostgreSQL dependency** from the notification service — it no longer writes to `notifications`, `notification_type`, or `notification_channel` tables.
2. **Remove old gateway WebSocket/gRPC notification routes** (`/api/v1/notification/*`) — the frontend uses FRN's React SDK instead.
3. **Remove gRPC service** from the notification microservice if the gateway no longer calls it (inbox/mark-read handled by FRN SDK).
4. **Keep the notification_type and notification_channel tables** in PostgreSQL for historical data. No need to drop them.
5. **Uncomment** `the_monkeys_notification` in `docker-compose.yml` with updated env vars for FRN.

---

## 12. FRN Template Registry

One-time setup script to create all templates. Run after FRN is running.

**New file:** `scripts/setup-frn-templates.sh`

```bash
#!/bin/bash
# Creates all FRN notification templates for The Monkeys.
# Run once after FRN is running: ./scripts/setup-frn-templates.sh

FRN_URL="${FRN_BASE_URL:-http://localhost:8080/v1}"
API_KEY="${FRN_API_KEY:?Set FRN_API_KEY}"
APP_ID="${FRN_APP_ID:?Set FRN_APP_ID}"

create_template() {
  local name=$1 channel=$2 subject=$3 body=$4 vars=$5 category=$6
  curl -s -X POST "$FRN_URL/templates/" \
    -H "X-API-Key: $API_KEY" \
    -H "Content-Type: application/json" \
    -d "{
      \"app_id\": \"$APP_ID\",
      \"name\": \"$name\",
      \"channel\": \"$channel\",
      \"subject\": \"$subject\",
      \"body\": \"$body\",
      \"variables\": $vars,
      \"locale\": \"en\",
      \"metadata\": { \"category\": \"$category\" }
    }"
  echo " -> $name ($channel)"
}

echo "Creating FRN templates for The Monkeys..."

# Social: New Follower
create_template "new_follower_inapp"   "in_app" '{{.follower_name}} started following you'  'You have a new follower.'                              '["follower_name"]'                                "social"
create_template "new_follower_sse"     "sse"    '{{.follower_name}} started following you'  'You have a new follower.'                              '["follower_name"]'                                "social"

# Social: New Comment
create_template "new_comment_inapp"    "in_app" '{{.commenter_name}} commented on \"{{.blog_title}}\"'   '{{.commenter_name}} wrote: {{.comment_preview}}'  '["commenter_name","blog_title","comment_preview"]' "social"
create_template "new_comment_sse"      "sse"    '{{.commenter_name}} commented on \"{{.blog_title}}\"'   '{{.commenter_name}} wrote: {{.comment_preview}}'  '["commenter_name","blog_title","comment_preview"]' "social"

# Social: Blog Liked
create_template "blog_liked_inapp"     "in_app" '{{.liker_name}} liked \"{{.blog_title}}\"'              'Someone appreciated your work!'                    '["liker_name","blog_title"]'                      "social"

# Collaboration: Co-Author Invite
create_template "coauthor_invite_inapp"  "in_app" '{{.inviter_name}} invited you to co-author a blog'      'You have been invited to collaborate on \"{{.blog_title}}\".' '["inviter_name","blog_title"]' "collaboration"
create_template "coauthor_invite_sse"    "sse"    '{{.inviter_name}} invited you to co-author a blog'      'You have been invited to collaborate on \"{{.blog_title}}\".' '["inviter_name","blog_title"]' "collaboration"
create_template "coauthor_invite_email"  "email"  '{{.inviter_name}} invited you to co-author a blog on Monkeys' 'Hi! {{.inviter_name}} has invited you to collaborate on \"{{.blog_title}}\". Log in to accept or decline.' '["inviter_name","blog_title"]' "collaboration"

# Collaboration: Co-Author Accept
create_template "coauthor_accept_inapp"  "in_app" '{{.coauthor_name}} accepted your co-author invite'     '{{.coauthor_name}} is now a co-author on \"{{.blog_title}}\".' '["coauthor_name","blog_title"]' "collaboration"
create_template "coauthor_accept_sse"    "sse"    '{{.coauthor_name}} accepted your co-author invite'     '{{.coauthor_name}} is now a co-author on \"{{.blog_title}}\".' '["coauthor_name","blog_title"]' "collaboration"

# Collaboration: Co-Author Decline
create_template "coauthor_decline_inapp" "in_app" '{{.coauthor_name}} declined your co-author invite'     '{{.coauthor_name}} declined the invite for \"{{.blog_title}}\".' '["coauthor_name","blog_title"]' "collaboration"

# Collaboration: Co-Author Removed
create_template "coauthor_removed_inapp" "in_app" 'You were removed as co-author'                          'You have been removed from \"{{.blog_title}}\" by {{.remover_name}}.' '["remover_name","blog_title"]' "collaboration"
create_template "coauthor_removed_sse"   "sse"    'You were removed as co-author'                          'You have been removed from \"{{.blog_title}}\" by {{.remover_name}}.' '["remover_name","blog_title"]' "collaboration"

# Content: Co-Authored Blog Published
create_template "blog_published_coauthor_inapp" "in_app" 'A co-authored blog was published'                   '\"{{.blog_title}}\" was published by {{.publisher_name}}.' '["publisher_name","blog_title"]' "content"
create_template "blog_published_coauthor_sse"   "sse"    'A co-authored blog was published'                   '\"{{.blog_title}}\" was published by {{.publisher_name}}.' '["publisher_name","blog_title"]' "content"

# Security: Password Changed
create_template "password_changed_inapp" "in_app" 'Your password was changed'                               'Your account password was updated. If this was not you, contact support immediately.' '[]' "security"
create_template "password_changed_email" "email"  'Your Monkeys password was changed'                        'Your account password was updated. If this was not you, contact support immediately.' '[]' "security"

# Security: Email Verified
create_template "email_verified_inapp"  "in_app"  'Your email was verified'                                  'Your email address has been verified successfully.'  '[]' "security"

echo "Done! All templates created."
```

---

## 13. File-by-File Change Map

### Notification Service (core changes)
| # | File | Action | Description |
|---|---|---|---|
| 1 | `microservices/the_monkeys_notification/internal/freerangenotify/client.go` | **Create** | FRN HTTP client (`Send`, `RegisterUser`, `GetSSEToken`) |
| 2 | `microservices/the_monkeys_notification/internal/freerangenotify/notify.go` | **Create** | High-level `Notify()` helper (in_app + sse + email) |
| 3 | `microservices/the_monkeys_notification/internal/consumer/consumer.go` | **Rewrite** | Replace `db.CreateNotification()` calls with FRN `Notify()` calls. Add new `case` branches for all 10 events. |
| 4 | `microservices/the_monkeys_notification/internal/models/models.go` | **Edit** | Add `BlogTitle` field to `TheMonkeysMessage` |
| 5 | `microservices/the_monkeys_notification/main.go` | **Edit** | Initialize FRN client, pass to consumer instead of DB |

### Config & Constants
| # | File | Action | Description |
|---|---|---|---|
| 6 | `dev.env` | **Edit** | Add `FRN_BASE_URL`, `FRN_API_KEY`, `FRN_APP_ID`, `FRN_EMAIL_ENABLED`, `FRN_SSE_PUBLIC_URL` |
| 7 | `config/config.go` | **Edit** | Add `FreeRangeNotify` struct to `Config` |
| 8 | `constants/frn_templates.go` | **Create** | FRN template name constants |
| 9 | `constants/actions.go` | **Edit** | Add `CO_AUTHOR_INVITE`, `CO_AUTHOR_ACCEPT`, `CO_AUTHOR_DECLINE`, `CO_AUTHOR_REMOVED`, `CO_AUTHOR_BLOG_PUBLISHED`, `PASSWORD_CHANGED`, `EMAIL_VERIFIED`, `BLOG_COMMENT` |

### Services (add missing publishes)
| # | File | Action | Description |
|---|---|---|---|
| 10 | `microservices/the_monkeys_users/internal/services/service.go` | **Edit** | Add `PublishMessage` in `InviteCoAuthor` and `RevokeCoAuthorAccess` |
| 11 | `microservices/the_monkeys_authz/internal/services/services.go` | **Edit** | Add `PublishMessage` in `UpdatePassword` and `VerifyEmail` |

### Gateway (SSE token only)
| # | File | Action | Description |
|---|---|---|---|
| 12 | `microservices/the_monkeys_gateway/internal/freerangenotify/client.go` | **Create** | Minimal FRN client (only `GetSSEToken`) |
| 13 | `microservices/the_monkeys_gateway/internal/freerangenotify/routes.go` | **Create** | `GET /api/v1/notifications/sse-token` endpoint |
| 14 | `microservices/the_monkeys_gateway/main.go` | **Edit** | Initialize FRN client, register SSE token route |

### Setup & Infra
| # | File | Action | Description |
|---|---|---|---|
| 15 | `scripts/setup-frn-templates.sh` | **Create** | One-time FRN template creation script |
| 16 | `docker-compose.yml` | **Edit** | Uncomment `the_monkeys_notification`, add FRN env vars |

### Future work (requires new features)
| # | File | Action | Description |
|---|---|---|---|
| 17 | Users service proto | **Edit** | Add `AcceptCoAuthorInvite`, `DeclineCoAuthorInvite` RPCs |
| 18 | `microservices/the_monkeys_users/internal/services/service.go` | **Edit** | Implement accept/decline handlers + publish to RoutingKeys[4] |
| 19 | Gateway user routes | **Edit** | Add accept/decline routes |
| 20 | Blog/Users service | **Edit** | Add `GetBlogCoAuthors` query for publish notifications |
| 21 | Comment feature | **Create** | Full comment system (routes, service, DB) + publish to RoutingKeys[4] |

---

## Implementation Priority

| Priority | Phase | Effort | Events Covered |
|---|---|---|---|
| **P0** | Phase 1 (Foundation) | Small | FRN client, config, constants in notification service |
| **P0** | Phase 3 (Retarget Consumer) | Medium | Rewrite consumer to call FRN. Covers follow, like, register. |
| **P1** | Phase 4.2-4.3 (Co-Author Invite/Remove) | Small | Add 2 `PublishMessage` calls in Users service |
| **P1** | Phase 4.4-4.5 (Security) | Small | Add 2 `PublishMessage` calls in Authz service |
| **P1** | Phase 6 (SSE Token) | Small | One gateway endpoint |
| **P2** | Phase 5 (Email) | Small | Create 2 email templates in FRN |
| **P2** | Phase 7 (Frontend) | Medium | React SDK integration |
| **P3** | Phase 4.6 (Accept/Decline) | Medium | Requires new gRPC RPCs + handlers + publishes |
| **P3** | Phase 4.7 (Blog Published) | Medium | Requires co-author lookup + publish loop |
| **P4** | Phase 4.8 (Comments) | Large | Requires full comment feature build |
| **P5** | Phase 8 (Clean Up) | Small | Remove old PostgreSQL/gRPC/WebSocket code |
