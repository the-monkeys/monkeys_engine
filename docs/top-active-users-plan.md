# Top Active Users — Implementation Plan

## 1. Problem

We need a **"Top Active Users"** feature that surfaces the most engaged users on the platform over a configurable time window. This is useful for leaderboards, community highlights, and author discovery.

## 2. What Already Exists

| Component | Status | Notes |
|-----------|--------|-------|
| `GET /api/v2/user/active-users` | ✅ Exists | Returns users active in last N hours (default 3h), ordered by recency — **NOT by engagement** |
| `GetActiveUsers` gRPC | ✅ Exists | Returns `user_id` + `last_active` timestamp only |
| ES `activity-events*` index | ✅ Rich data | Tracks `action`, `user_id`, `account_id`, `category`, `resource_id`, `duration_ms`, `@timestamp` |

### Gap Analysis

The existing `GetActiveUsers` answers **"who is online now?"** — it returns users with any recent activity, sorted by last-seen time. It does **not** answer **"who are the most active/engaged users?"** which requires:

- Counting meaningful actions per user (writes > reads)
- Weighting actions by engagement value (publish > like > view)
- Scoring and ranking users
- Returning activity breakdown (blogs published, likes received, comments, etc.)

## 3. Proposed Solution

### 3.1 New gRPC Method

Add `GetTopActiveUsers` to the `ActivityService` in the proto file.

```protobuf
// In gw_activity.proto — service ActivityService
rpc GetTopActiveUsers(GetTopActiveUsersRequest) returns (GetTopActiveUsersResponse);

message GetTopActiveUsersRequest {
  string time_range = 1;  // "24h", "7d", "30d", "90d" (default: "7d")
  int32  limit      = 2;  // max users to return (default: 10, max: 50)
}

message TopActiveUser {
  string user_id          = 1;  // account_id
  int64  activity_count   = 2;  // total weighted actions
  int64  blogs_published  = 3;
  int64  blogs_read       = 4;
  int64  likes_given      = 5;
  int64  comments_made    = 6;
  double engagement_score = 7;  // weighted composite score
}

message GetTopActiveUsersResponse {
  int32                status_code = 1;
  repeated TopActiveUser users     = 2;
  Error                error       = 3;
}
```

### 3.2 Engagement Scoring Formula

Weighted scoring to rank users by **meaningful** activity, not just page views:

| Action | Weight | Rationale |
|--------|--------|-----------|
| `publish` / `create` | **10** | Content creation is highest value |
| `edit` | **3** | Improving existing content |
| `comment` | **5** | Community engagement |
| `like` | **2** | Social signal |
| `share` | **3** | Distribution effort |
| `bookmark` | **1** | Passive engagement |
| `read_blog` | **1** | Consumption (high volume, low weight) |
| `follow` | **2** | Community building |

**Score = Σ (action_count × weight)**

### 3.3 Elasticsearch Query

Single aggregation query on `activity-events*`:

```
POST activity-events*/_search
{
  "size": 0,
  "query": {
    "bool": {
      "must": [
        { "range": { "@timestamp": { "gte": "now-7d" } } },
        { "exists": { "field": "user_id" } }
      ],
      "must_not": [
        { "term": { "client_info.is_bot": true } },
        { "term": { "user_id.keyword": "" } }
      ],
      "filter": [
        { "terms": { "action.keyword": [
            "publish", "create", "edit", "comment",
            "like", "share", "bookmark", "read_blog", "follow"
          ]
        }}
      ]
    }
  },
  "aggs": {
    "top_users": {
      "terms": {
        "field": "user_id.keyword",
        "size": <limit>
      },
      "aggs": {
        "action_breakdown": {
          "terms": { "field": "action.keyword", "size": 20 }
        }
      }
    }
  }
}
```

Score computation happens **server-side in Go** after receiving buckets — not in ES. This keeps the query simple and the scoring logic easy to tune.

### 3.4 Gateway Endpoint

```
GET /api/v2/user/top-active
  ?time_range=7d    (default: "7d")
  &limit=10         (default: 10, max: 50)
```

**Response:**
```json
{
  "status_code": 200,
  "users": [
    {
      "user_id": "acc_xxx",
      "username": "dave",
      "first_name": "Dave",
      "avatar": "https://...",
      "engagement_score": 142.0,
      "activity_count": 87,
      "blogs_published": 3,
      "blogs_read": 45,
      "likes_given": 12,
      "comments_made": 8
    }
  ]
}
```

The gateway enriches the activity service response with user profile details via `GetBatchUserDetails` (same pattern as the existing `GetActiveUsers` handler).

## 4. Files to Modify

### Layer 1: Proto (generate after editing)

| File | Change |
|------|--------|
| `apis/serviceconn/gateway_activity/pb/gw_activity.proto` | Add `rpc GetTopActiveUsers`, request/response messages |
| Run `protoc` | Regenerate `gw_activity.pb.go` and `gw_activity_grpc.pb.go` |

### Layer 2: Activity Service (Elasticsearch → gRPC)

| File | Change |
|------|--------|
| `microservices/the_monkeys_activity/internal/database/elasticsearch.go` | Add `GetTopActiveUsers()` method — ES query + score computation |
| `microservices/the_monkeys_activity/internal/database/elasticsearch.go` | Add to `ActivityDatabase` interface |
| `microservices/the_monkeys_activity/internal/services/activity_methods.go` | Add gRPC handler, call `db.GetTopActiveUsers()` |

### Layer 3: Gateway (HTTP → gRPC)

| File | Change |
|------|--------|
| `microservices/the_monkeys_gateway/internal/user_service/routes.go` | Add `GetTopActiveUsers` handler + enrich with user details |
| `microservices/the_monkeys_gateway/internal/user_service/routes.go` | Register `GET /api/v2/user/top-active` route |

## 5. Implementation Order

```
Step 1 ─── Proto definition + protoc generate
              │
Step 2 ─── ES query in elasticsearch.go
              │  (GetTopActiveUsers method + interface update)
              │
Step 3 ─── gRPC handler in activity_methods.go
              │
Step 4 ─── Gateway handler + route in routes.go
              │
Step 5 ─── Test with curl / Postman
```

## 6. Caching Strategy

Follow the same pattern as `GetTrendingBlogs` (which caches global 24h results):

- Cache key: `top_active_users:{time_range}:{limit}`
- TTL: **5 minutes** for `≤24h`, **15 minutes** for `7d+`
- Invalidation: TTL-based only (no write-through needed)
- Storage: In-memory (same `trendingCache` pattern)

## 7. Performance Considerations

| Concern | Mitigation |
|---------|------------|
| Large ES result set | `"size": 0` (no hits, aggs only); `terms.size` capped at 50 |
| Hot path latency | In-memory cache with short TTL |
| Bot noise | `must_not` filter on `client_info.is_bot` |
| Empty/anonymous users | Filter `user_id` exists + non-empty |
| Score computation | Go-side, O(users × actions) — trivially fast for ≤50 users |

## 8. Not In Scope (Future)

- Trending topics (separate feature)
- User badges / achievements
- Streak tracking (consecutive days active)
- Per-category leaderboards
