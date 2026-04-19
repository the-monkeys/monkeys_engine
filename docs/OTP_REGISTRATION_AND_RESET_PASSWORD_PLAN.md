# OTP-Based Registration & Password Reset

**Date:** April 19, 2026
**Status:** Implemented
**Reference:** FreeRangeNotify (`otp_repository.go` — Redis SET/GET/DEL with TTL)

---

## Problem

1. **Spam registrations** — accounts created immediately on `POST /register` with garbage emails (`dfgdf@123.com`). JWT returned before email verified.
2. **Insecure password reset** — reset secret exposed in URL query parameters (browser history, proxy logs, Referer headers).

## Solution

### Registration: Two-step OTP flow

| Step | Endpoint | What happens |
|------|----------|-------------|
| 1 | `POST /api/v1/auth/register/initiate` | Validate input + email format + MX lookup + disposable check. Generate 6-digit OTP. Store `{email, password_hash, otp_hash, ...}` in **Redis** with 10-min TTL. Send OTP email. **No account created.** |
| 2 | `POST /api/v1/auth/register/verify-otp` | Verify OTP (bcrypt). Create account, publish RabbitMQ, return JWT. |
| 2a | `POST /api/v1/auth/register/resend-otp` | Regenerate OTP (30-sec cooldown). |

### Password Reset: OTP instead of URL

| Step | Endpoint | What happens |
|------|----------|-------------|
| 1 | `POST /api/v1/auth/forgot-pass` (modified) | Generate 6-digit OTP. Store hash in **Redis** with 10-min TTL. Email OTP. |
| 2 | `POST /api/v1/auth/verify-reset-otp` (new) | Verify OTP. Return short-lived (5 min) `password_reset` JWT. |
| 3 | `POST /api/v1/auth/update-password` (existing) | Accepts the reset JWT. |

## Architecture: Redis (not PostgreSQL)

Following FreeRangeNotify's exact pattern:

```
SET  tm:otp:register:<email>  <json>  EX 600    # auto-expires, no cleanup
GET  tm:otp:register:<email>                     # returns nil if expired
DEL  tm:otp:register:<email>                     # after successful verification
```

- `go-redis/v9` already in `go.mod`
- `Config.Redis` already wired via `.env` (`REDIS_HOST`, etc.)
- Redis handles TTL natively — **no cleanup goroutine, no migration, no DB table**
- Same pattern for reset OTP: `tm:otp:reset:<email>`

## Backward Compatibility

| Concern | Status |
|---------|--------|
| `POST /register` (old) | Still works, calls old `RegisterUser` RPC. Frontend should migrate to `/register/initiate` + `/register/verify-otp`. |
| RabbitMQ messages | Identical payload (`USER_REGISTER`, same routing keys) — published after OTP verification. |
| Google OAuth | Untouched. Bypasses OTP entirely. |
| `GET /reset-password` (old URL flow) | Kept for deprecation period. |
| Existing users | No change. Normal login works. |
| JWT structure | Unchanged. New `password_reset` token type only valid for reset flow. |

## Files Changed

### Created
- `microservices/the_monkeys_authz/internal/db/otp_redis.go` — Redis OTP repository (`PendingRegistration`, `ResetOTPData` structs, Store/Get/Increment/Delete for both registration and reset)
- `microservices/the_monkeys_authz/internal/services/otp_registration.go` — 4 service methods: `InitiateRegistration`, `VerifyRegistrationOTP`, `ResendRegistrationOTP`, `VerifyResetOTP`

### Modified
- `apis/serviceconn/gateway_authz/pb/gw_auth.proto` — 4 new RPCs + 7 new messages
- `microservices/the_monkeys_authz/internal/services/services.go` — Added `otpRepo *db.OTPRepository` to `AuthzSvc` struct. Modified `ForgotPassword` to use OTP via Redis instead of URL-based token.
- `microservices/the_monkeys_authz/internal/utils/utils.go` — Added `GenerateOTP()`, `ValidateEmailFormat()`
- `microservices/the_monkeys_authz/internal/utils/html.go` — Added `RegistrationOTPEmailHTML()`, `PasswordResetOTPEmailHTML()`
- `microservices/the_monkeys_authz/internal/utils/jwt.go` — Added `GeneratePasswordResetToken()`
- `microservices/the_monkeys_authz/main.go` — Creates `OTPRepository`, passes to `NewAuthzSvc`
- `microservices/the_monkeys_gateway/internal/auth/models.go` — 4 new request body structs
- `microservices/the_monkeys_gateway/internal/auth/routes.go` — 4 new handlers + route registration + `handleGRPCError` helper

### Removed
- `schema/000004_add_pending_registrations.up.sql` — No longer needed (Redis, not PostgreSQL)
- `schema/000004_add_pending_registrations.down.sql`
- `microservices/the_monkeys_authz/internal/db/queries.go` — PostgreSQL pending_reg CRUD removed
- `PendingRegistration` model from `models/auth.go` — Moved to `db/otp_redis.go` as Redis data struct
- 5 pending registration methods from `db/db.go` interface — Replaced by `OTPRepository`

## Security

- OTP generated with `crypto/rand` (not `math/rand`)
- OTP hashed with bcrypt before storage (timing-safe comparison)
- Max 5 failed attempts per OTP (auto-deletes from Redis)
- 30-second cooldown between resends
- Email format + MX record validation + disposable email blocklist
- `ForgotPassword` no longer reveals whether email exists (always returns success message)
- Password reset JWT: 5-min expiry, `token_type: "password_reset"` — rejected by normal auth validation

## API Contracts

### POST /api/v1/auth/register/initiate
```json
// Request
{"first_name": "John", "last_name": "Doe", "email": "john@example.com", "password": "securePass123"}

// Response 200
{"message": "Verification code sent to your email", "expires_in": 600}
```

### POST /api/v1/auth/register/verify-otp
```json
// Request
{"email": "john@example.com", "otp_code": "482916"}

// Response 201
{"email": "john@example.com", "username": "a1b2c3...", "account_id": "...", "email_verified": true, ...}
// (token + refresh_token set as HTTP-only cookies, stripped from response body)
```

### POST /api/v1/auth/register/resend-otp
```json
// Request
{"email": "john@example.com"}

// Response 200
{"message": "Verification code resent to your email", "expires_in": 600}
```

### POST /api/v1/auth/verify-reset-otp
```json
// Request
{"email": "john@example.com", "otp_code": "739201"}

// Response 200
{"token": "<password_reset_jwt>"}
// Frontend uses this token in Authorization header for POST /update-password
```

## Infrastructure

### Docker Compose
- Added `the_monkeys_cache` service (Redis 7 Alpine) with:
  - Persistent AOF storage (`redis_data` volume)
  - 128MB memory limit with LRU eviction
  - Health check via `redis-cli ping`
  - Connected to `monkeys-network`
- `the_monkeys_authz` now `depends_on` `the_monkeys_cache`
- `.env` already had `REDIS_HOST=the_monkeys_cache:6379` configured

## Frontend (Next.js)

### Registration Flow (Two-step with policy agreement)
**Step 1** — User fills registration form:
- First name, last name, email, password, confirm password
- **Checkbox**: "I agree to the Terms of Service, Privacy Policy, and Code of Conduct" (links to `/terms`, `/privacy`)
- Zod schema enforces `agreeToTerms: z.literal(true)` — form won't submit without consent
- Calls `POST /api/v1/auth/register/initiate`

**Step 2** — OTP verification screen:
- Shows email the code was sent to
- 6-digit input (numeric, centered, tracking-widest)
- "Verify & Create Account" button
- "Resend Code" with 30-second cooldown timer
- "Back to registration" link
- Calls `POST /api/v1/auth/register/verify-otp`

### Password Reset Flow (Two-step OTP)
**Step 1** — Enter email → sends OTP (always shows success to not reveal user existence)
**Step 2** — Enter 6-digit OTP → verifies → redirects to `/auth/reset-password?token=<jwt>&email=<email>`

### Files Changed
- `src/services/auth/auth.ts` — Added 4 new API functions: `initiateRegistration`, `verifyRegistrationOTP`, `resendRegistrationOTP`, `verifyResetOTP`
- `src/lib/schema/auth.ts` — Added `otpVerificationSchema` (6-digit numeric), added `agreeToTerms` to `registerUserSchema`
- `src/app/auth/components/forms/RegisterUserForm.tsx` — Complete rewrite: two-step form (register → OTP), added terms/policy checkbox
- `src/app/auth/components/forms/ForgotPasswordForm.tsx` — Complete rewrite: two-step form (email → OTP → redirect to reset page with JWT)
