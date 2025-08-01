# Admin API Documentation

## Overview
The Admin API provides secure administrative functions for managing users, particularly for removing bot and fake accounts. This API is **restricted to local network access only** for security purposes.

## Security Features

### 1. Local Network Restriction
- API only accessible from local network IP ranges:
  - `127.0.0.0/8` (localhost)
  - `10.0.0.0/8` (private class A)
  - `172.16.0.0/12` (private class B)
  - `192.168.0.0/16` (private class C)
  - `::1/128` (IPv6 localhost)
  - `fc00::/7` (IPv6 unique local)

### 2. Admin Key Authentication
- All endpoints require `X-Admin-Key` header
- Default key: `monkeys-admin-2024-secure-key`
- **IMPORTANT**: Change this key in production!

## API Endpoints

### Base URL
```
http://localhost:8080/admin/api/v1
```

### Headers Required
```
X-Admin-Key: monkeys-admin-2024-secure-key
Content-Type: application/json
```

## User Management Endpoints

### 1. Force Delete User
**DELETE** `/admin/api/v1/users/{id}`

Deletes a user without normal authorization checks.

**Parameters:**
- `id` (path): Username/User ID to delete
- `reason` (query, optional): Reason for deletion

**Example:**
```bash
curl -X DELETE \
  "http://localhost:8080/admin/api/v1/users/suspicious_user?reason=Bot%20detected" \
  -H "X-Admin-Key: monkeys-admin-2024-secure-key"
```

**Response:**
```json
{
  "message": "User successfully deleted",
  "user_id": "suspicious_user",
  "reason": "Bot detected",
  "deleted_by": "admin",
  "result": {}
}
```

### 2. Bulk Delete Users
**DELETE** `/admin/api/v1/users/bulk`

Deletes multiple users at once (max 100 users per request).

**Request Body:**
```json
{
  "user_ids": ["bot1", "fake_user2", "spam_account3"],
  "reason": "Bulk bot removal"
}
```

**Example:**
```bash
curl -X DELETE \
  "http://localhost:8080/admin/api/v1/users/bulk" \
  -H "X-Admin-Key: monkeys-admin-2024-secure-key" \
  -H "Content-Type: application/json" \
  -d '{
    "user_ids": ["bot1", "fake_user2"],
    "reason": "Detected as fake accounts"
  }'
```

**Response:**
```json
{
  "message": "Bulk deletion completed",
  "total_users": 2,
  "successful": 2,
  "failed": 0,
  "reason": "Detected as fake accounts",
  "results": {
    "bot1": {"status": "success"},
    "fake_user2": {"status": "success"}
  },
  "processed_by": "admin"
}
```

### 3. Flag User as Bot/Fake
**POST** `/admin/api/v1/users/{id}/flag`

Flags a user as suspicious without deleting them.

**Request Body:**
```json
{
  "reason": "Suspicious posting pattern",
  "type": "bot"
}
```

**Valid types:** `bot`, `fake`, `spam`

**Example:**
```bash
curl -X POST \
  "http://localhost:8080/admin/api/v1/users/suspicious_user/flag" \
  -H "X-Admin-Key: monkeys-admin-2024-secure-key" \
  -H "Content-Type: application/json" \
  -d '{
    "reason": "Automated posting behavior",
    "type": "bot"
  }'
```

### 4. Unflag User
**POST** `/admin/api/v1/users/{id}/unflag`

Removes flags from a user.

**Parameters:**
- `reason` (query, optional): Reason for unflagging

**Example:**
```bash
curl -X POST \
  "http://localhost:8080/admin/api/v1/users/user123/unflag?reason=False%20positive" \
  -H "X-Admin-Key: monkeys-admin-2024-secure-key"
```

### 5. Get Suspicious Users
**GET** `/admin/api/v1/users/suspicious`

Returns users that might be bots or fake accounts (placeholder endpoint).

### 6. Get Flagged Users
**GET** `/admin/api/v1/users/flagged`

Returns all flagged users.

**Parameters:**
- `type` (query, optional): Filter by flag type (bot, fake, spam)

### 7. Get User Statistics
**GET** `/admin/api/v1/users/stats`

Returns user statistics for admin monitoring (placeholder endpoint).

## System Endpoints

### 1. Admin Health Check
**GET** `/admin/api/v1/health`

**Response:**
```json
{
  "status": "healthy",
  "service": "admin-api",
  "timestamp": "2024-01-01T00:00:00Z",
  "access_ip": "127.0.0.1"
}
```

### 2. System Statistics
**GET** `/admin/api/v1/system/stats`

Returns system-wide statistics (placeholder endpoint).

## Error Responses

### Access Denied (Non-local IP)
```json
{
  "error": "Access denied: Admin API only accessible from local network"
}
```

### Invalid Admin Key
```json
{
  "error": "Invalid admin key"
}
```

### User Not Found
```json
{
  "error": "User not found",
  "user_id": "nonexistent_user"
}
```

## Security Best Practices

1. **Change the default admin key** before deploying to production
2. **Monitor admin API access** through logs
3. **Use VPN or SSH tunneling** when accessing remotely
4. **Implement rate limiting** for additional security
5. **Regular audit** of admin actions
6. **Backup user data** before bulk deletions

## Usage Examples

### Detecting and Removing Bot Accounts

1. **Identify suspicious users** (manual or automated process)
2. **Flag for review:**
   ```bash
   curl -X POST "http://localhost:8080/admin/api/v1/users/bot_account/flag" \
     -H "X-Admin-Key: monkeys-admin-2024-secure-key" \
     -H "Content-Type: application/json" \
     -d '{"reason": "Suspicious activity pattern", "type": "bot"}'
   ```

3. **Review flagged users:**
   ```bash
   curl "http://localhost:8080/admin/api/v1/users/flagged?type=bot" \
     -H "X-Admin-Key: monkeys-admin-2024-secure-key"
   ```

4. **Bulk remove confirmed bots:**
   ```bash
   curl -X DELETE "http://localhost:8080/admin/api/v1/users/bulk" \
     -H "X-Admin-Key: monkeys-admin-2024-secure-key" \
     -H "Content-Type: application/json" \
     -d '{
       "user_ids": ["bot1", "bot2", "bot3"],
       "reason": "Confirmed bot accounts"
     }'
   ```

## Integration with Disposable Email Detection

The system already includes disposable email detection in the registration process. Users with disposable emails are automatically rejected during registration, reducing the number of fake accounts created.

## Future Enhancements

- **Machine learning integration** for automatic bot detection
- **Pattern analysis** for suspicious behavior
- **Bulk user analysis** tools
- **Integration with external reputation services**
- **Automated cleanup schedules**
