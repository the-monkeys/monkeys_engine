# FreeRangeNotify Integration — End-to-End Guide

This document covers the complete integration of **FreeRangeNotify (FRN)** into **The Monkeys Engine**, including infrastructure setup, backend plumbing, template seeding, user migration, SSE real-time delivery, in-app persistence, and frontend rendering.

---

## Table of Contents

1. [Architecture Overview](#1-architecture-overview)
2. [Infrastructure & Environment Setup](#2-infrastructure--environment-setup)
3. [Backend: Notification Microservice](#3-backend-notification-microservice)
4. [Backend: Gateway Proxy Routes](#4-backend-gateway-proxy-routes)
5. [Template Seeding (26 Templates)](#5-template-seeding-26-templates)
6. [User Migration (132 Users)](#6-user-migration-132-users)
7. [Frontend: SSE + In-App Notification Dropdown](#7-frontend-sse--in-app-notification-dropdown)
8. [FRN-Side Changes Required](#8-frn-side-changes-required)
9. [Event-to-Notification Mapping](#9-event-to-notification-mapping)
10. [Testing & Verification](#10-testing--verification)

---

## 1. Architecture Overview

```
┌──────────────┐      HTTP/REST       ┌──────────────────┐
│   Frontend   │ ◄──────────────────► │  Monkeys Gateway │
│  (Next.js)   │                      │   (Gin, :8081)   │
│              │                      │                  │
│  EventSource │ ◄─── SSE ──────────► │  FRN API (:8080) │
└──────────────┘                      └────────┬─────────┘
                                               │
                                    gRPC / HTTP proxy
                                               │
                                     ┌─────────▼──────────┐
                                     │ Notification Svc    │
                                     │ (Go, :50055)        │
                                     │                     │
                                     │ RabbitMQ consumer   │
                                     │ → FRN Go SDK        │
                                     └─────────┬───────────┘
                                               │
                                          FRN HTTP API
                                               │
                                     ┌─────────▼───────────┐
                                     │ FreeRangeNotify      │
                                     │  API    (:8080)      │
                                     │  Worker (background) │
                                     │  ES     (:9200)      │
                                     │  Redis  (:6379)      │
                                     └────────────────────┘
```

**Data flow:**

1. A Monkeys service (blog, user, authz) publishes an event to **RabbitMQ** (exchange: `smart_monkey`, routing key: `to_notification_svc_key`).
2. The **Notification Microservice** consumes from `to_notification_svc_queue` (queue index 4).
3. The consumer maps the event action to FRN template constants and calls the **FRN Go SDK** (`freerangenotify v0.2.2`) to dispatch:
   - **in_app** — always sent (persisted in FRN's Elasticsearch)
   - **sse** — real-time push via Server-Sent Events (fire-and-forget)
   - **email** — sent for security-critical events (fire-and-forget)
4. The **Gateway** proxies three FRN endpoints behind auth middleware so the frontend never talks to FRN directly.
5. The **Frontend** fetches persisted in_app notifications on mount and maintains an SSE connection for real-time updates.

**Two separate Docker Compose stacks:**
- **Monkeys Engine** — gateway, blog, user, authz, activity, notification, ES (port 9201 externally), Postgres, RabbitMQ, Redis
- **FreeRangeNotify** — api, worker, ES (port 9200), Redis, UI

Cross-stack communication uses `host.docker.internal`.

---

## 2. Infrastructure & Environment Setup

### 2.1 Elasticsearch Port Separation

FRN runs its own Elasticsearch on port 9200. To avoid conflicts, Monkeys ES was remapped:

| Service       | Internal Port | External Port |
|---------------|---------------|---------------|
| Monkeys ES    | 9200          | **9201**      |
| FRN ES        | 9200          | 9200          |
| Monkeys ES Transport | 9300   | **9301**      |

### 2.2 FRN Environment Variables (`.env`)

Added to the Monkeys Engine `.env` file:

```env
# FreeRangeNotify Integration
# FRN runs in a separate docker-compose; use host.docker.internal to reach it from containers.
FRN_BASE_URL=http://host.docker.internal:8080/v1
FRN_API_KEY=frn_key=
FRN_APP_ID=da32eb9e-app-id-bebe-freerangenotify
FRN_EMAIL_ENABLED=true
FRN_SSE_PUBLIC_URL=http://localhost:8080/v1
FRN_DEV_EMAIL=email@example.com
```

Key decisions:
- `FRN_BASE_URL` uses `host.docker.internal` because the notification service runs inside a Docker container but FRN is in a separate compose stack.
- `FRN_SSE_PUBLIC_URL` uses `localhost:8080` because the **browser** connects to SSE directly (not from inside a container).
- `FRN_DEV_EMAIL` overrides all email recipients in development so emails always go to the test address.

### 2.3 Docker Compose Changes

- **Notification service** was uncommented and enabled in `docker-compose.yml`.
- **Activity service** added `elasticsearch-node1` to `depends_on` (was crash-looping on startup without ES).

### 2.4 Frontend Environment (`.env.local`)

```env
NEXT_PUBLIC_API_URL=http://127.0.0.1:8081/api/v1
NEXT_PUBLIC_API_URL_V2=http://127.0.0.1:8081/api/v2
NEXT_PUBLIC_WSS_URL=ws://127.0.0.1:8081/api/v1
NEXT_PUBLIC_WSS_URL_V2=ws://127.0.0.1:8081/api/v2
NEXT_PUBLIC_LIVE_URL=https://themonkeys.live
NEXT_PUBLIC_FRN_URL=http://localhost:8080/v1
```

`NEXT_PUBLIC_FRN_URL` is used by the frontend to connect to FRN's SSE endpoint directly from the browser.

---

## 3. Backend: Notification Microservice

### 3.1 FRN SDK Client Wrapper

**File:** `microservices/the_monkeys_notification/internal/freerangenotify/client.go`

Wraps the official FRN Go SDK with dev-email override logic:

```go
type Client struct {
    SDK      *frn.Client
    DevEmail string
    Log      *zap.SugaredLogger
}

func NewClient(baseURL, apiKey, devEmail string, log *zap.SugaredLogger) *Client {
    sdk := frn.New(apiKey,
        frn.WithBaseURL(baseURL),
        frn.WithTimeout(5*time.Second),
    )
    return &Client{SDK: sdk, DevEmail: devEmail, Log: log}
}

func (c *Client) RegisterUser(ctx context.Context, email, username string) error {
    if c.DevEmail != "" {
        email = c.DevEmail // Override in dev
    }
    _, err := c.SDK.Users.Create(ctx, frn.CreateUserParams{
        Email:      email,
        ExternalID: username, // Monkeys username as FRN external_id
    })
    return err
}

func (c *Client) Send(ctx context.Context, params frn.NotificationSendParams) error {
    _, err := c.SDK.Notifications.Send(ctx, params)
    return err
}
```

### 3.2 Notify Dispatcher

**File:** `microservices/the_monkeys_notification/internal/freerangenotify/notify.go`

Sends in_app (always, error propagated), SSE and email (fire-and-forget):

```go
type NotifyRequest struct {
    UserID   string
    InAppTpl string                 // Always sent
    SSETpl   string                 // Empty = skip
    EmailTpl string                 // Empty = skip
    Priority string                 // "low", "normal", "high", "critical"
    Category string                 // "social", "collaboration", "content", "security"
    Data     map[string]interface{} // Template variables
}

func Notify(ctx context.Context, client *Client, req NotifyRequest, log *zap.SugaredLogger) error {
    // in_app — critical, error propagated
    if err := client.Send(ctx, frn.NotificationSendParams{
        UserID: req.UserID, Channel: "in_app", Priority: req.Priority,
        TemplateID: req.InAppTpl, Category: req.Category, Data: req.Data,
    }); err != nil {
        return err
    }

    // SSE — fire-and-forget
    if req.SSETpl != "" {
        _ = client.Send(ctx, frn.NotificationSendParams{...})
    }

    // Email — fire-and-forget
    if req.EmailTpl != "" {
        _ = client.Send(ctx, frn.NotificationSendParams{...})
    }
    return nil
}
```

### 3.3 RabbitMQ Consumer

**File:** `microservices/the_monkeys_notification/internal/consumer/consumer.go`

Consumes from queue index 4 (`to_notification_svc_queue`) and routes to 14 event handlers:

```go
func ConsumeFromQueue(mgr *rabbitmq.ConnManager, conf config.RabbitMQ, log *zap.SugaredLogger, frn *freerangenotify.Client) {
    go consumeQueue(mgr, conf.Queues[4], log, frn)
    select {}
}
```

Each event is unmarshalled from the `TheMonkeysMessage` struct and dispatched via `handleUserAction()`.

---

## 4. Backend: Gateway Proxy Routes

**File:** `microservices/the_monkeys_gateway/internal/notification/routes.go`

Three FRN proxy routes are registered under `/api/v1/notification`, all behind `AuthRequired` middleware:

| Method | Path | Handler | FRN Endpoint | Auth Header |
|--------|------|---------|--------------|-------------|
| `GET` | `/sse-token` | `GetSSEToken` | `POST /v1/sse/tokens` | `Bearer <API_KEY>` |
| `GET` | `/frn` | `GetFRNNotifications` | `GET /v1/notifications?user_id=<username>&channel=in_app` | `X-API-Key` |
| `POST` | `/frn/read-all` | `MarkAllFRNRead` | `POST /v1/notifications/read-all` | `X-API-Key` |

### 4.1 SSE Token Endpoint

The gateway requests a short-lived SSE token from FRN on behalf of the authenticated user:

```go
func (nsc *NotificationServiceClient) GetSSEToken(ctx *gin.Context) {
    userName := ctx.GetString("userName")
    // POST to FRN: /v1/sse/tokens with {"user_id": userName}
    // Returns: {sse_token, user_id, expires_in}
    // Forward response directly to client
}
```

The frontend uses this token to connect to `FRN_URL/sse?sse_token=<token>` via `EventSource`.

### 4.2 Notification List Endpoint

Proxies FRN's notification list with pagination:

```go
func (nsc *NotificationServiceClient) GetFRNNotifications(ctx *gin.Context) {
    // GET FRN: /v1/notifications?user_id=<username>&channel=in_app&page_size=20&page=1
    // Returns: {notifications: [...], total, page, page_size}
}
```

### 4.3 Mark All Read Endpoint

```go
func (nsc *NotificationServiceClient) MarkAllFRNRead(ctx *gin.Context) {
    // POST FRN: /v1/notifications/read-all with {"user_id": userName}
}
```

### 4.4 Route Registration

In `microservices/the_monkeys_gateway/main.go`, the notification routes were enabled:

```go
notification.RegisterNotificationRoute(server.router, cfg, authClient, log)
```

This line was previously commented out and had to be uncommented along with the import.

---

## 5. Template Seeding (26 Templates)

### 5.1 Template Constants

**File:** `constants/frn_templates.go`

26 template name constants across 4 categories:

| Category | Templates | Channels |
|----------|-----------|----------|
| **Social** (5) | `new_follower_inapp`, `new_follower_sse`, `new_comment_inapp`, `new_comment_sse`, `blog_liked_inapp` | in_app, sse |
| **Collaboration** (8) | `coauthor_invite_inapp/sse/email`, `coauthor_accept_inapp/sse`, `coauthor_decline_inapp`, `coauthor_removed_inapp/sse` | in_app, sse, email |
| **Content** (2) | `blog_published_coauthor_inapp`, `blog_published_coauthor_sse` | in_app, sse |
| **Security** (11) | `password_changed_inapp/email`, `email_verified_inapp`, `login_detected_inapp/sse/email`, `password_reset_requested_inapp/email`, `email_changed_inapp/email`, `username_changed_inapp` | in_app, sse, email |

### 5.2 Seeding via API

All 26 templates were seeded using FRN's bulk seed endpoint. The endpoint is **idempotent** — re-running returns `"skipped": 26` if templates already exist.

```powershell
$headers = @{ "X-API-Key" = "<FRN_API_KEY>" }
$body = @'
{
  "app_id": "<FRN_APP_ID>",
  "templates": [
    {"name":"new_follower_inapp","channel":"in_app","category":"social","subject":"New Follower","body":"{{.follower_name}} started following you","priority":"normal"},
    {"name":"new_follower_sse","channel":"sse","category":"social","subject":"New Follower","body":"{{.follower_name}} started following you","priority":"normal"},
    {"name":"new_comment_inapp","channel":"in_app","category":"social","subject":"New Comment","body":"{{.commenter_name}} commented on your post","priority":"normal"},
    {"name":"new_comment_sse","channel":"sse","category":"social","subject":"New Comment","body":"{{.commenter_name}} commented on your post","priority":"normal"},
    {"name":"blog_liked_inapp","channel":"in_app","category":"social","subject":"Blog Liked","body":"{{.liker_name}} liked your blog \"{{.blog_title}}\"","priority":"low"},
    {"name":"coauthor_invite_inapp","channel":"in_app","category":"collaboration","subject":"Co-Author Invitation","body":"{{.inviter_name}} invited you to co-author \"{{.blog_title}}\"","priority":"high"},
    {"name":"coauthor_invite_sse","channel":"sse","category":"collaboration","subject":"Co-Author Invitation","body":"{{.inviter_name}} invited you to co-author \"{{.blog_title}}\"","priority":"high"},
    {"name":"coauthor_invite_email","channel":"email","category":"collaboration","subject":"You've been invited to co-author a blog","body":"{{.inviter_name}} invited you to co-author \"{{.blog_title}}\". Log in to accept or decline.","priority":"high"},
    {"name":"coauthor_accept_inapp","channel":"in_app","category":"collaboration","subject":"Invitation Accepted","body":"{{.coauthor_name}} accepted your co-author invitation for \"{{.blog_title}}\"","priority":"normal"},
    {"name":"coauthor_accept_sse","channel":"sse","category":"collaboration","subject":"Invitation Accepted","body":"{{.coauthor_name}} accepted your co-author invitation for \"{{.blog_title}}\"","priority":"normal"},
    {"name":"coauthor_decline_inapp","channel":"in_app","category":"collaboration","subject":"Invitation Declined","body":"{{.coauthor_name}} declined your co-author invitation for \"{{.blog_title}}\"","priority":"normal"},
    {"name":"coauthor_removed_inapp","channel":"in_app","category":"collaboration","subject":"Removed as Co-Author","body":"{{.remover_name}} removed you as co-author from \"{{.blog_title}}\"","priority":"normal"},
    {"name":"coauthor_removed_sse","channel":"sse","category":"collaboration","subject":"Removed as Co-Author","body":"{{.remover_name}} removed you as co-author from \"{{.blog_title}}\"","priority":"normal"},
    {"name":"blog_published_coauthor_inapp","channel":"in_app","category":"content","subject":"Blog Published","body":"{{.publisher_name}} published \"{{.blog_title}}\" that you co-authored","priority":"normal"},
    {"name":"blog_published_coauthor_sse","channel":"sse","category":"content","subject":"Blog Published","body":"{{.publisher_name}} published \"{{.blog_title}}\" that you co-authored","priority":"normal"},
    {"name":"password_changed_inapp","channel":"in_app","category":"security","subject":"Password Changed","body":"Your password was changed successfully. If this wasn't you, secure your account immediately.","priority":"high"},
    {"name":"password_changed_email","channel":"email","category":"security","subject":"Your password was changed","body":"Your password was changed successfully. If this wasn't you, reset your password immediately.","priority":"high"},
    {"name":"email_verified_inapp","channel":"in_app","category":"security","subject":"Welcome!","body":"Your email has been verified. Welcome to The Monkeys!","priority":"high"},
    {"name":"login_detected_inapp","channel":"in_app","category":"security","subject":"New Login Detected","body":"New login from {{.client}} ({{.ip_address}}) via {{.login_method}}","priority":"high"},
    {"name":"login_detected_sse","channel":"sse","category":"security","subject":"New Login Detected","body":"New login from {{.client}} ({{.ip_address}}) via {{.login_method}}","priority":"high"},
    {"name":"login_detected_email","channel":"email","category":"security","subject":"New login to your account","body":"We detected a new login from {{.client}} ({{.ip_address}}) via {{.login_method}}. If this wasn't you, secure your account.","priority":"high"},
    {"name":"password_reset_requested_inapp","channel":"in_app","category":"security","subject":"Password Reset Requested","body":"A password reset was requested from {{.ip_address}}. If this wasn't you, ignore this.","priority":"high"},
    {"name":"password_reset_requested_email","channel":"email","category":"security","subject":"Password reset requested","body":"A password reset was requested for your account from {{.ip_address}}. If this wasn't you, ignore this email.","priority":"high"},
    {"name":"email_changed_inapp","channel":"in_app","category":"security","subject":"Email Changed","body":"Your email was changed to {{.new_email}}. If this wasn't you, contact support.","priority":"high"},
    {"name":"email_changed_email","channel":"email","category":"security","subject":"Your email address was changed","body":"Your email was changed to {{.new_email}}. If this wasn't you, contact support immediately.","priority":"high"},
    {"name":"username_changed_inapp","channel":"in_app","category":"security","subject":"Username Changed","body":"Your username was changed to {{.new_username}}","priority":"normal"}
  ]
}
'@

Invoke-RestMethod -Uri "http://localhost:8080/v1/templates/seed" `
  -Method POST -Headers $headers -ContentType "application/json" -Body $body
```

**Response:** `{"created": 26, "skipped": 0}` on first run, `{"created": 0, "skipped": 26}` on re-run.

Each template definition includes:
- `name` — matches the constant in Go (e.g., `new_follower_inapp`)
- `channel` — `in_app`, `sse`, or `email`
- `category` — `social`, `collaboration`, `content`, or `security`
- `subject` / `body` — with `{{.variable}}` placeholders
- `priority` — `low`, `normal`, `high`, or `critical`

---

## 6. User Migration (132 Users)

All existing Monkeys users were migrated to FRN using the bulk upsert endpoint.

### 6.1 Export Users from Postgres

```powershell
# Query all users from Monkeys Postgres
$connStr = "Host=localhost;Port=5432;Database=the_monkeys_db;Username=myuser;Password=mypassword"
$query = "SELECT username, email FROM users WHERE user_status = 'active'"
# Returns 132 rows
```

### 6.2 Bulk Upsert to FRN

```powershell
$headers = @{ "X-API-Key" = "<FRN_API_KEY>" }

# Transform to FRN format and POST
# Each user needs: email + external_id (= Monkeys username)
$body = @'
{
  "app_id": "<FRN_APP_ID>",
  "upsert": true,
  "users": [
    {"email": "user1@example.com", "external_id": "user1_username"},
    {"email": "user2@example.com", "external_id": "user2_username"},
    ... (132 users total)
  ]
}
'@

Invoke-RestMethod -Uri "http://localhost:8080/v1/users/bulk" `
  -Method POST -Headers $headers -ContentType "application/json" -Body $body
```

**Response:** `{"created": 131, "updated": 1}` — the test user `dave_augustus` already existed and was updated.

### 6.3 Key Points

- The `upsert: true` flag ensures existing users are updated rather than erroring.
- The `external_id` field maps to the Monkeys username, which is used throughout the system to identify users.
- FRN generates its own internal UUID for each user; the `external_id` is how Monkeys references them.

### 6.4 Ongoing User Sync

**New user registration:** Automatically handled when the notification service receives a `USER_REGISTER` event via RabbitMQ. The consumer calls `frn.RegisterUser(ctx, email, username)`.

**Username changes:** When a user changes their username, the `USERNAME_CHANGED` event handler sends the notification under the old username, then calls `frn.UpdateUserExternalID(oldUsername, newUsername)` to update the FRN record. This is critical — without this update, the user would stop receiving notifications under their new username.

---

## 7. Frontend: SSE + In-App Notification Dropdown

### 7.1 Notification Types

**File:** `local/the_monkeys/apps/the_monkeys/src/services/notification/notificationTypes.ts`

```typescript
export interface FRNNotificationContent {
  title: string;
  body: string;
  data: Record<string, unknown>;
}

export interface FRNNotification {
  notification_id: string;
  app_id?: string;
  user_id?: string;
  channel: string;
  priority: string;
  status: string;            // "sent", "read", "seen"
  content: FRNNotificationContent;  // Nested content object
  category?: string;
  template_id?: string;
  created_at: string;
  updated_at?: string;
}
```

Key detail: FRN returns notification content as a **nested object** (`content.body`, `content.title`), not flat fields.

### 7.2 WSNotificationDropdown Component

**File:** `local/the_monkeys/apps/the_monkeys/src/components/layout/navbar/WSNotificationDropdown.tsx`

The component handles three concerns:

#### A. Fetch persisted notifications on mount

```typescript
const fetchNotifications = useCallback(async () => {
  const { data } = await axiosInstance.get<{
    notifications: FRNNotification[] | null;
    total: number;
  }>('/notification/frn?page_size=20&channel=in_app');

  const list = data.notifications ?? [];
  setNotifications(list);
  setUnreadCount(list.filter((n) => n.status !== 'read' && n.status !== 'seen').length);
}, []);
```

This calls the gateway proxy which forwards to `FRN GET /v1/notifications?user_id=<username>&channel=in_app`.

#### B. SSE real-time connection

```typescript
const connect = useCallback(async () => {
  // 1. Get SSE token from gateway
  const { data } = await axiosInstance.get('/notification/sse-token');

  // 2. Connect to FRN SSE directly
  const url = `${FRN_URL}/sse?sse_token=${encodeURIComponent(data.sse_token)}`;
  const es = new EventSource(url);

  // 3. Listen for notification events
  es.addEventListener('notification', (event) => {
    const payload = JSON.parse(event.data);
    const notif = payload.notification ?? payload;  // Unwrap SSE wrapper
    // Deduplicate and prepend
  });

  // 4. Auto-reconnect on error, refresh token before expiry
}, []);
```

The SSE token has an expiry. The component schedules a reconnect 60 seconds before expiry.

#### C. Mark all as read on dropdown open

```typescript
const markAllRead = useCallback(async () => {
  if (unreadCount === 0) return;
  await axiosInstance.post('/notification/frn/read-all');
  setUnreadCount(0);
  setNotifications((prev) => prev.map((n) => ({ ...n, status: 'read' })));
}, [unreadCount]);
```

Called when the dropdown opens via `onOpenChange`.

#### D. Rendering

Notifications display:
- **Category/channel** label (top-left)
- **Relative timestamp** (top-right) — "just now", "5m ago", "2h ago", "3d ago"
- **Content body** — `notif.content?.body || notif.content?.title`
- **Badge** — shows unread count on the bell icon (filled bell when unread)
- **Scrollable** — `max-h-[420px]` with overflow

---

## 8. FRN-Side Changes Required

The following changes were made in FreeRangeNotify to support this integration:

### 8.1 InAppProvider

**File:** `internal/infrastructure/providers/inapp_provider.go`

A no-op provider that marks `in_app` notifications as "sent" so the worker doesn't fail:

```go
type InAppProvider struct{}
func (p *InAppProvider) Send(ctx context.Context, notif *models.Notification) error {
    return nil // in_app is stored by the API, the worker just marks it delivered
}
```

Registered in `cmd/worker/main.go` after the SSE provider.

### 8.2 Template Seed Endpoint

`POST /v1/templates/seed` — accepts a bulk array of template definitions and creates them idempotently. Returns count of created/skipped.

### 8.3 Bulk User Upsert Endpoint

`POST /v1/users/bulk` — accepts an array of users with `upsert: true` flag. Creates new users or updates existing ones.

### 8.4 External ID Resolution

`GET /v1/notifications?user_id=<external_id>` — updated to resolve `external_id` (Monkeys username) to FRN internal UUID, so notifications can be queried by Monkeys username directly.

### 8.5 Read-All Endpoint

`POST /v1/notifications/read-all` with `{"user_id": "<external_id>"}` — marks all notifications as read for a user, resolving external_id.

### 8.6 User Update by External ID

`PUT /v1/users/<external_id>` with `{"external_id": "new_value"}` — resolves the user by `external_id` and updates their record. Used by the notification service when a Monkeys user changes their username.

### 8.7 SSE Token Endpoint

`POST /v1/sse/tokens` — generates a short-lived token for SSE connection. Accepts `user_id` (external_id).

---

## 9. Event-to-Notification Mapping

| RabbitMQ Action | Recipient | in_app Template | SSE Template | Email Template | Priority | Category |
|-----------------|-----------|-----------------|--------------|----------------|----------|----------|
| `USER_REGISTER` | self | `email_verified_inapp` | — | — | high | security |
| `USER_FOLLOWED` | followed user | `new_follower_inapp` | `new_follower_sse` | — | normal | social |
| `BLOG_LIKE` | blog author | `blog_liked_inapp` | — | — | low | social |
| `CO_AUTHOR_INVITE` | invitee | `coauthor_invite_inapp` | `coauthor_invite_sse` | `coauthor_invite_email` | high | collaboration |
| `CO_AUTHOR_ACCEPT` | inviter | `coauthor_accept_inapp` | `coauthor_accept_sse` | — | normal | collaboration |
| `CO_AUTHOR_DECLINE` | inviter | `coauthor_decline_inapp` | — | — | normal | collaboration |
| `CO_AUTHOR_REMOVED` | removed user | `coauthor_removed_inapp` | `coauthor_removed_sse` | — | normal | collaboration |
| `CO_AUTHOR_BLOG_PUBLISHED` | co-author | `blog_published_coauthor_inapp` | `blog_published_coauthor_sse` | — | normal | content |
| `PASSWORD_CHANGED` | self | `password_changed_inapp` | — | `password_changed_email` | high | security |
| `EMAIL_VERIFIED` | self | `email_verified_inapp` | — | — | high | security |
| `LOGIN_DETECTED` | self | `login_detected_inapp` | `login_detected_sse` | `login_detected_email` | high | security |
| `PASSWORD_RESET_REQUESTED` | self | `password_reset_requested_inapp` | — | `password_reset_requested_email` | high | security |
| `EMAIL_CHANGED` | self | `email_changed_inapp` | — | `email_changed_email` | high | security |
| `USERNAME_CHANGED` | self | `username_changed_inapp` | — | — | normal | security |

> **USERNAME_CHANGED special handling:** After sending the notification, the consumer also calls `frn.UpdateUserExternalID(oldUsername, newUsername)` to update the user's `external_id` in FRN. Without this, the user would not receive any future notifications after changing their username.

**Template variables** passed in `Data`:

| Variable | Used By |
|----------|---------|
| `follower_name` | USER_FOLLOWED |
| `liker_name`, `blog_title` | BLOG_LIKE |
| `inviter_name`, `blog_title` | CO_AUTHOR_INVITE |
| `coauthor_name`, `blog_title` | CO_AUTHOR_ACCEPT, CO_AUTHOR_DECLINE |
| `remover_name`, `blog_title` | CO_AUTHOR_REMOVED |
| `publisher_name`, `blog_title` | CO_AUTHOR_BLOG_PUBLISHED |
| `ip_address`, `client`, `login_method` | LOGIN_DETECTED |
| `ip_address` | PASSWORD_RESET_REQUESTED |
| `new_email` | EMAIL_CHANGED |
| `new_username` | USERNAME_CHANGED |

---

## 10. Testing & Verification

### 10.1 Trigger a Test Notification via RabbitMQ

Publish a `user_followed` event to the notification queue:

```powershell
$body = @{
    properties = @{ content_type = "application/json" }
    routing_key = "to_notification_svc_key"
    payload = '{"action":"user_followed","username":"test_user_123","new_username":"dave_augustus"}'
    payload_encoding = "string"
} | ConvertTo-Json

Invoke-RestMethod -Uri "http://localhost:15672/api/exchanges/%2F/smart_monkey/publish" `
  -Method POST `
  -Headers @{ Authorization = "Basic " + [Convert]::ToBase64String([Text.Encoding]::ASCII.GetBytes("myuser:mypassword")) } `
  -ContentType "application/json" `
  -Body $body
```

### 10.2 Verify Notification Delivery

1. **Notification service logs:** Should show `Received user follow: test_user_123 → dave_augustus` and successful FRN SDK calls.
2. **FRN API:** `GET http://localhost:8080/v1/notifications?user_id=dave_augustus&channel=in_app` should return the notification.
3. **Frontend:** If SSE is connected, the notification appears in real-time. On reload, it appears from the persisted fetch.

### 10.3 Verify SSE Connection

1. Open the frontend, log in, and check browser DevTools Network tab for an `sse?sse_token=...` request.
2. The EventStream should show `connected` and then `notification` events.

### 10.4 Verify Mark-All-Read

1. Click the notification bell icon.
2. The badge should clear and all notifications should show as read.
3. Refreshing the page should show 0 unread.

---

## Files Modified/Created

### The Monkeys Engine

| File | Change |
|------|--------|
| `.env` | Added FRN config section (BASE_URL, API_KEY, APP_ID, SSE_PUBLIC_URL, DEV_EMAIL) |
| `docker-compose.yml` | Uncommented notification service, added ES `depends_on` for activity |
| `microservices/the_monkeys_gateway/main.go` | Uncommented `notification.RegisterNotificationRoute()`, added import |
| `microservices/the_monkeys_gateway/internal/notification/routes.go` | Added `GetSSEToken`, `GetFRNNotifications`, `MarkAllFRNRead` handlers and routes |
| `microservices/the_monkeys_notification/internal/freerangenotify/client.go` | **New** — FRN SDK wrapper + `UpdateUserExternalID()` for username sync |
| `microservices/the_monkeys_notification/internal/freerangenotify/notify.go` | **New** — Multi-channel dispatch (in_app + SSE + email) |
| `microservices/the_monkeys_notification/internal/consumer/consumer.go` | **Rewritten** — 14 event handlers using FRN SDK; USERNAME_CHANGED also updates FRN external_id |
| `constants/frn_templates.go` | **New** — 26 template name constants |

### Frontend (local/the_monkeys)

| File | Change |
|------|--------|
| `.env.local` | Switched to localhost, added `NEXT_PUBLIC_FRN_URL` |
| `src/components/layout/navbar/WSNotificationDropdown.tsx` | **Rewritten** — SSE + persisted fetch + mark-all-read |
| `src/services/notification/notificationTypes.ts` | Added `FRNNotification` types with nested content |

### FreeRangeNotify

| File | Change |
|------|--------|
| `internal/infrastructure/providers/inapp_provider.go` | **New** — No-op in_app provider |
| `cmd/worker/main.go` | Registered InAppProvider |
| Various API endpoints | Template seed, bulk user upsert, external_id resolution |
