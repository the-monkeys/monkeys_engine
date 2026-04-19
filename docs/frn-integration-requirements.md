# FreeRangeNotify (FRN) Integration — Feature & API Requirements

## Overview
This document tracks which FRN features The Monkeys Engine requires for full user lifecycle management, their current status, and any gaps that need addressing on the FRN side or the Monkeys publisher side.

**Last updated:** Post FRN PR #72 merge — all bugs and core feature requests resolved. Client code simplified to use SDK exclusively (no raw HTTP).

---

## User Lifecycle Operations

| # | Operation | FRN API Endpoint | FRN Status | Publisher Status | Notes |
|---|-----------|-----------------|------------|-----------------|-------|
| 1 | **Create user** | `POST /v1/users` | ✅ Working | ✅ `authz` (register + Google OAuth) | SDK `CreateUserParams` now includes `FullName`. |
| 2 | **Update email** | `PUT /v1/users/{identifier}` | ✅ Working | ✅ `authz` → `RoutingKeys[4]` | SDK accepts external_id as identifier. |
| 3 | **Update username (external_id)** | `PUT /v1/users/{identifier}` | ✅ Working | ✅ `authz` → `RoutingKeys[4]` | Data race fixed — message marshalled before goroutine. |
| 4 | **Delete user** | `DELETE /v1/users/{identifier}` | ✅ Working | ✅ `users` → `RoutingKeys[4]` | SDK accepts external_id as identifier. |
| 5 | **Update preferences** | `PUT /v1/users/{identifier}/preferences` | ✅ Working | ❌ No publisher | No service emits `preferences_changed` yet. |
| 6 | **Deactivate (DND on)** | `PUT /v1/users/{identifier}/preferences` | ✅ Working | ❌ No publisher | No service emits `user_deactivated` yet. |
| 7 | **Reactivate (DND off)** | `PUT /v1/users/{identifier}/preferences` | ✅ Working | ❌ No publisher | No service emits `user_reactivated` yet. |
| 8 | **Bulk sync** | `POST /v1/users/bulk` | ✅ Working | ✅ Startup sync | SDK `BulkCreate` with `SkipExisting: true`. |

---

## Monkeys-Side Bugs Found & Fixed

### FIXED: USERNAME_CHANGED publisher data race
- **Root cause:** `notifBx` was marshalled **inside** a goroutine, but `user.Username` was mutated to `req.NewUsername` after the goroutine was spawned (line ~1098 in `services.go`). By the time the goroutine ran, `user.Username` was already the new value → both `Username` and `NewUsername` were identical.
- **Evidence:** Logs showed `old_username: "tom", new_username: "tom"` but FRN external_id was `5871cf00e6f92c7c18a41b756c9ac648`.
- **Fix:** Marshal `notifBx` **before** the goroutine, alongside `bx`.

### FIXED: USER_ACCOUNT_DELETE not published to notification queue
- **Root cause:** `DeleteUserAccount()` in `the_monkeys_users` only published to `RoutingKeys[0]` (user service) and `RoutingKeys[3]` (blog service) — not `RoutingKeys[4]` (notification service).
- **Fix:** Added publish to `RoutingKeys[4]`.

### PENDING: Missing event publishers
The notification consumer handles these events but no service emits them yet:
- `user_deactivated` — needs publisher in authz or user service when account is deactivated
- `user_reactivated` — needs publisher in authz or user service when account is reactivated
- `preferences_changed` — needs publisher in user service when notification preferences are updated

---

## FRN API Bugs — ALL RESOLVED in PR #72

### BUG-1: POST /notifications returns 500 for missing users (should be 404) — ✅ FIXED
- **Fix:** `notification_handler.go` now checks for typed `*AppError` and returns correct HTTP status (404).

### BUG-2: GET /users/:id returns 500 for non-existent users (should be 404) — ✅ FIXED
- **Fix:** `user_service_impl.go` now checks `errors.IsNotFound(err)` and returns typed NotFound errors.

### BUG-3: GET /users/?external_id= filter is ignored — ✅ FIXED
- **Fix:** `user_repository.go` added `ExternalID` to `UserFilter` and query builder. `user_handler.go` parses `external_id` query param.

### BUG-4: resolveUserID errors fall through to catch-all 500 — ✅ FIXED
- **Fix:** `notification_service.go` `resolveUserID` now returns typed `errors.NotFound()`.

### BUG-5: base_repository returns untyped error for ES 404 — ✅ FIXED
- **Fix:** `base_repository.go` now returns `errors.NotFound()` instead of `fmt.Errorf("document not found")`.

---

## Feature Requests — Status

### FR-1: SDK `CreateUserParams` should include `full_name` — ✅ RESOLVED
- **Fix:** `types.go` added `FullName string` to `CreateUserParams`, `UpdateUserParams`, and `User`.
- **Impact:** Eliminated raw HTTP for user creation.

### FR-2: SDK support for external_id in all User methods — ✅ RESOLVED
- **Fix:** `users.go` `Get()`, `Update()`, `Delete()` now accept `identifier` (UUID or external_id).
- **Impact:** Eliminated all raw HTTP calls — client.go now uses SDK exclusively.

### FR-3: Get user by external_id — ✅ RESOLVED
- **Fix:** New `GET /v1/users/by-external-id/:external_id` endpoint + SDK `GetByExternalID()` method.

### FR-4: Bulk user upsert endpoint — ✅ RESOLVED
- **Fix:** `BulkCreateUsersParams` now has `SkipExisting bool` and `Upsert bool` fields.
- **Impact:** Startup sync uses single BulkCreate call instead of one-by-one registration.

### FR-5: Proper HTTP status codes for all error cases — ✅ RESOLVED
- **Fix:** Addressed by BUG-1 through BUG-5 fixes.

### FR-6: Webhook/callback for delivery status — OPEN
- **Current state:** Notification delivery is fire-and-forget.
- **Request:** Optional webhook callback for notification delivery status (delivered, bounced, failed).
- **Why:** Enables delivery tracking and alerting on failed notifications.

---

## Monkeys Engine Integration Summary

```
┌─────────────────────────────────────────────────────────────────┐
│                    RabbitMQ Event                                │
│  (user_register, email_changed, username_changed, etc.)         │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│              consumer.go — handleUserAction()                   │
│  Routes events to FRN client methods + sends notifications      │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│              client.go — FRN Client wrapper (SDK only)          │
│  RegisterUser | UpdateUserEmail | UpdateUserExternalID          │
│  DeleteUser | DeactivateUser | ReactivateUser                   │
│  UpdateUserPreferences | Send                                   │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│              FRN Go SDK (v1.0.1+)                               │
│  Users: Create, BulkCreate, Update, Delete, Get,                │
│         GetByExternalID, UpdatePreferences                      │
│  Notifications: Send                                            │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│              FreeRangeNotify API (v1)                            │
│  Elasticsearch backend, per-app user scoping                    │
└─────────────────────────────────────────────────────────────────┘
```
