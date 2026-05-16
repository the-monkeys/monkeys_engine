# Search v2: Design, Industry Standards, and Implementation Plan

> Audience: Monkeys engineering team. Written in simple English so everyone, including non-native English speakers, can follow.
>
> Status: Draft for team review.
> Owner: Platform / Search working group.
> Last updated: 2026-05-16

---

## 1. Why this document exists

Right now Monkeys lets users search **blogs** and **people**. The search works, but it is slow and it often does not return what the user is looking for. We will explain:

1. How search works today (front-end, gateway, blog service, user service, Elasticsearch, Postgres).
2. What top platforms do (Medium, GitHub, Twitter/X, Reddit, LinkedIn).
3. What we should change, and why.
4. A clear, step-by-step plan to build it.

If you only have 5 minutes, read **Section 2 (today's pain points)** and **Section 6 (target architecture)**.

---

## 2. What is broken today (the short version)

### 2.1 Blog search
- We never index the **blog title** as a separate field. We only search the body text and the tags. So if a user searches the exact title, the result can be missing or buried.
- There is **no fuzzy matching**. If a user types `monky` instead of `monkey`, we return zero results.
- Sorting is always **newest first**, never **most relevant first**. A perfect title match from last month loses to a weak body match from yesterday.
- No **highlighting**. The user does not see which words matched.
- No **autocomplete** index. The dropdown waits for full tokens.

### 2.2 People search
- We use Postgres `ILIKE '%term%'`. The leading `%` **forbids the index from being used**. Every search reads the whole `user_account` table.
- We only search `username`, `first_name`, `last_name`. We ignore `bio`.
- We do not filter out inactive or hidden users.
- There is **no ranking**. An exact username match has the same weight as a partial last-name match.
- No timeout, no `limit` cap. A bad query can hurt the database.

### 2.3 Front-end
- The Zustand cache store exists but is **not wired in**. Each keystroke triggers a fresh round trip.
- No recent searches, no result highlighting, no "see all results" link from the dropdown.
- Two different API versions: blogs use `/api/v2`, users use `/api/v1`. Inconsistent.

---

## 3. How top platforms do search (in simple terms)

| Platform | Engine | Key tricks they use |
|---|---|---|
| **Medium** | Elasticsearch | Separate indices for stories, people, tags. Heavy use of synonyms and stemming. Personalised re-ranking. |
| **GitHub** | Elasticsearch (Code Search uses a custom engine called *Blackbird*) | Per-field boosting (title > description > body). Filters as part of the query language (`stars:>10 language:go`). |
| **Twitter / X** | Earlybird (custom Lucene fork) | Real-time indexing. Top-k early termination. Cheap "first page" path that is much faster than deep scrolling. |
| **Reddit** | Elasticsearch with their own ranker | Combines lexical match score with social signals (upvotes, recency, subreddit). |
| **LinkedIn** | Galene (custom on Lucene) | Heavy autocomplete (typeahead), learning-to-rank for people, strict spam filters. |

Common patterns we will steal:
1. **Multiple fields with different boosts**. Title >> tags >> body.
2. **Fuzziness for short tokens**, exact for long ones.
3. **Edge n-grams** (or `search_as_you_type` field) for autocomplete.
4. **Hybrid ranking**: relevance score, then a small recency boost (not the other way around).
5. **Highlight** the matched words in the response.
6. **Cursor pagination** (`search_after`), not deep `from/size`.
7. **Hard caps** on `limit`, **timeouts** on every call.

---

## 4. Current architecture (as built)

```
┌─────────────┐    HTTPS    ┌──────────────────┐  gRPC   ┌────────────────────┐  HTTP   ┌────────────────┐
│  Next.js    │ ──────────▶│   Gateway (Gin)  │ ──────▶ │  Blog service (Go) │ ──────▶ │ Elasticsearch  │
│  (browser)  │             │                  │         │                    │         │ the_monkeys_   │
│             │             │  /api/v2/blog/   │         │  SearchBlogsMeta…  │         │    blogs       │
│             │             │     search       │         │                    │         └────────────────┘
│             │             │                  │  gRPC   ┌────────────────────┐  SQL    ┌────────────────┐
│             │             │  /api/v1/user/   │ ──────▶ │  User service (Go) │ ──────▶ │  PostgreSQL    │
│             │             │     search       │         │  SearchUser stream │         │  user_account  │
└─────────────┘             └──────────────────┘         └────────────────────┘         └────────────────┘
```

### 4.1 Blog search call chain
- Route: `GET /api/v2/blog/search?search_term=...&limit=&offset=` — [blog/routes.go:169](microservices/the_monkeys_gateway/internal/blog/routes.go#L169)
- Handler: `searchBlogsQuery` — [blog/handler.go:346](microservices/the_monkeys_gateway/internal/blog/handler.go#L346)
- gRPC: `SearchBlogsMetadata` — [gw_blog.proto:450](apis/serviceconn/gateway_blog/pb/gw_blog.proto#L450)
- Service: [service_v2.go:634](microservices/the_monkeys_blog/internal/services/service_v2.go#L634)
- ES query builder: [blogs_matadata.go:388](microservices/the_monkeys_blog/internal/database/blogs_matadata.go#L388)
- Index: `the_monkeys_blogs` — mapping at [documents/reindex/mapping_v2.json](documents/reindex/mapping_v2.json)

### 4.2 User search call chain
- Route: `GET /api/v1/user/search` — [user_service/routes.go:77](microservices/the_monkeys_gateway/internal/user_service/routes.go#L77)
- Handler: `SearchUser` — [user_service/routes.go:917](microservices/the_monkeys_gateway/internal/user_service/routes.go#L917)
- gRPC: `SearchUser` (bidi stream) — [gw_user.proto:295](apis/serviceconn/gateway_user/pb/gw_user.proto#L295)
- Service: [service.go:965](microservices/the_monkeys_users/internal/services/service.go#L965)
- SQL: [query.go:727](microservices/the_monkeys_users/internal/database/query.go#L727)

### 4.3 The Elasticsearch query we send today (blog)

```jsonc
{
  "from": 0, "size": 20,
  "sort": [
    { "published_time": { "order": "desc", "unmapped_type": "date" } },
    { "blog.time":      { "order": "desc" } }
  ],
  "query": {
    "bool": {
      "must": [
        { "bool": {
            "should": [
              { "match":        { "blog.blocks.data.text": { "query": "<term>", "boost": 2.0 } } },
              { "match":        { "tags":                   { "query": "<term>", "boost": 2.5 } } },
              { "match_phrase": { "blog.blocks.data.text": { "query": "<term>", "boost": 3.0, "slop": 3 } } }
            ],
            "minimum_should_match": 1
        } },
        { "term": { "is_draft": false } }
      ],
      "must_not": [ { "term": { "is_archived": true } } ]
    }
  }
}
```

Problems:
- No `title` field.
- No fuzziness.
- Default analyzer (no stemming, no synonyms).
- Sort by date kills the relevance score.

### 4.4 The Postgres query we send today (people)

```sql
SELECT username, first_name, last_name, bio, avatar_url
FROM   user_account
WHERE  username   ILIKE $1
   OR  first_name ILIKE $1
   OR  last_name  ILIKE $1
ORDER  BY username
LIMIT  $2 OFFSET $3;
-- $1 = '%term%'
```

Problems:
- Leading `%` → full table scan, every time.
- No ranking, no FTS, no trigram.
- No `user_status = Active` filter.
- No upper bound on `limit`.

---

## 5. Design goals for v2

In priority order:

1. **Correctness first.** A user typing the exact title of a published blog must see that blog on top.
2. **Speed.** p95 search latency under 200 ms for the first page of results.
3. **Forgiving.** Typos, plural/singular, accents — all should still find the right thing.
4. **Predictable.** Same query should produce stable results between page 1 and page 2.
5. **Safe.** Hard caps, timeouts, no way to take down the DB with one bad query.
6. **Observable.** We must be able to see what people search and what we returned.

Out of scope for v2 (parked for v3):
- Personalised re-ranking per user.
- Vector / semantic search.
- Cross-language search.

---

## 6. Target architecture

```
                         ┌─────────────────────────────────────┐
                         │  Frontend (Next.js)                 │
                         │                                     │
                         │  /search?q=…  + global SearchInput  │
                         │  React Query, 250 ms debounce,      │
                         │  Zustand history & recent searches  │
                         └───────────────┬─────────────────────┘
                                         │ HTTPS
                                         ▼
                         ┌─────────────────────────────────────┐
                         │  Gateway (Gin)                      │
                         │                                     │
                         │  GET /api/v2/search?q=&type=…&…     │  ◀── single unified endpoint
                         │   • input validation                │
                         │   • timeout 1 s                     │
                         │   • short-TTL Redis cache (60 s)    │
                         │   • fan-out to blog + user RPCs     │
                         └───────────┬───────────────┬─────────┘
                                     │ gRPC          │ gRPC
                                     ▼               ▼
                       ┌─────────────────────┐  ┌────────────────────┐
                       │ Blog svc            │  │ User svc           │
                       │ SearchBlogsV2       │  │ SearchUserV2       │
                       │ (unary, not stream) │  │ (unary)            │
                       └──────────┬──────────┘  └─────────┬──────────┘
                                  │                       │
                                  ▼                       ▼
                       ┌─────────────────────┐  ┌────────────────────┐
                       │ Elasticsearch       │  │ PostgreSQL         │
                       │  the_monkeys_blogs  │  │  user_account      │
                       │  (new mapping v3)   │  │  + pg_trgm GIN     │
                       └─────────────────────┘  └────────────────────┘
```

### 6.1 Front-end changes
- One hook, `useSearch({ q, type, limit, cursor })`. Type is `posts | authors | all`.
- 250 ms debounce, abort previous in-flight request on new keystroke (use `AbortController`).
- Zustand store wired in: cache last 15 queries, also store last 10 "recent searches" in `localStorage`.
- Result cards highlight the matched words (`<mark>`).
- Dropdown gets a "See all results for *X*" footer link.

### 6.2 Gateway changes
- New endpoint `GET /api/v2/search`. Old `/blog/search` and `/user/search` stay for one release as deprecated.
- Validate `q` (1–128 chars), cap `limit` at 50, default 20.
- Wrap every RPC in `context.WithTimeout(ctx, 1*time.Second)`.
- Add a Redis read-through cache keyed by `(q, type, cursor)` with 60 s TTL. Cache only the **id list**, not the full body — keeps cache small and consistent if a blog is edited.

### 6.3 Blog service & Elasticsearch
- **New mapping `the_monkeys_blogs_v3`** with these top-level fields written at index time:
  - `title` (text, analyzer `english`, plus `.keyword` sub-field, plus `.autocomplete` sub-field using `search_as_you_type`).
  - `summary` (text, `english` analyzer) — first 300 chars of body, stored at write time, not read time.
  - `body` (text, `english` analyzer) — concatenated text of all paragraph blocks.
  - `tags` (keyword for filter + text for match, `english` analyzer).
  - `author_username`, `author_display_name` (text + keyword).
  - `is_draft`, `is_archived`, `is_scheduled` (boolean).
  - `published_time` (date).
  - `like_count`, `view_count` (long) — for tie-breaking.
- Index alias: `the_monkeys_blogs` → `…_v3`. Lets us reindex with zero downtime.
- New query DSL (sketch):

```jsonc
{
  "size": 20,
  "track_total_hits": false,                  // faster
  "query": {
    "function_score": {
      "query": {
        "bool": {
          "must": [{
            "multi_match": {
              "query": "<term>",
              "fields": [
                "title^6",
                "title.autocomplete^4",
                "tags^3",
                "summary^2",
                "body^1"
              ],
              "type": "best_fields",
              "fuzziness": "AUTO",            // typo tolerance
              "operator": "and",              // all words must appear
              "minimum_should_match": "75%"
            }
          }],
          "filter": [
            { "term": { "is_draft": false } },
            { "term": { "is_archived": false } }
          ]
        }
      },
      "functions": [
        {                                     // small recency boost
          "exp": { "published_time": { "origin": "now", "scale": "30d", "decay": 0.5 } },
          "weight": 1.0
        },
        {                                     // tiny popularity nudge
          "field_value_factor": { "field": "like_count", "modifier": "log1p", "missing": 0 }
        }
      ],
      "score_mode": "sum",
      "boost_mode": "sum"
    }
  },
  "highlight": {
    "pre_tags": ["<mark>"], "post_tags": ["</mark>"],
    "fields": { "title": {}, "summary": {}, "body": { "fragment_size": 140, "number_of_fragments": 1 } }
  },
  "sort": [ "_score", { "published_time": "desc" } ]
}
```

- Pagination: use `search_after` with `[score, published_time, blog_id]` so deep pages stay fast.
- Write path: the same RabbitMQ flow that already triggers indexing also writes the new flat fields (`title`, `summary`, `body`, `author_*`). Drafts stay in the index but are filtered out at search time.
- Add a tiny `/api/v2/search/suggest?q=` endpoint backed by `title.autocomplete` for the navbar dropdown.

### 6.4 User service & Postgres
- Enable `pg_trgm` extension (one-time migration).
- Add a generated column and a GIN index:

```sql
ALTER TABLE user_account
  ADD COLUMN search_doc TEXT
  GENERATED ALWAYS AS (
    coalesce(username,'')   || ' ' ||
    coalesce(first_name,'') || ' ' ||
    coalesce(last_name,'')  || ' ' ||
    coalesce(bio,'')
  ) STORED;

CREATE INDEX idx_user_account_search_trgm
  ON user_account USING GIN (search_doc gin_trgm_ops);

CREATE INDEX idx_user_account_username_trgm
  ON user_account USING GIN (username gin_trgm_ops);
```

- New query (ranks exact username highest, then trigram similarity, hides non-active accounts):

```sql
SELECT account_id, username, first_name, last_name, bio, avatar_url,
       CASE WHEN lower(username) = lower($1)        THEN 100
            WHEN lower(username) LIKE lower($1)||'%' THEN 50
            ELSE 0 END
       + similarity(search_doc, $1) * 30           AS rank
FROM   user_account
JOIN   user_status s ON s.id = user_account.user_status
WHERE  s.status = 'Active'
  AND  (search_doc % $1 OR username ILIKE $1 || '%')
ORDER  BY rank DESC, username ASC
LIMIT  $2 OFFSET $3;
```

- Cap `limit` at 50 server side.
- Add 500 ms `context.WithTimeout` in the service.
- Replace bidi gRPC stream with a simple unary RPC. Streaming gave us nothing here and made the code harder.

### 6.5 Observability
- One structured log line per search request: `query`, `type`, `latency_ms`, `result_count`, `cache_hit`, `user_id_hash`.
- Activity service already swallows events; add a `search_event` topic and dashboard "top searches with zero results" — this is how we find missing content / synonyms to add.

---

## 7. Capacity & cost notes
- The new mapping roughly doubles the size of the blog index (we store `body` separately). At today's volume (~few thousand blogs) this is tiny.
- `search_after` is cheaper than `from/size` after the first page. p95 should drop on page 3+.
- `pg_trgm` GIN index size is roughly 30–50% of the table. Acceptable at our user count.
- Redis cache: 60 s TTL on the id-list cache. Memory budget ~5 MB.

---

## 8. Risks and mitigations

| Risk | Mitigation |
|---|---|
| Reindex takes long / breaks search | Build `_v3` alongside, switch alias at the end, keep `_v2` for rollback. |
| Highlight HTML reaches Editor.js block content (XSS) | Highlight only on whitelisted fields (`title`, `summary`), sanitise on the server before returning. |
| Boost weights become magic numbers | Document them here; track "search quality" weekly using zero-result rate + click-through rate. |
| `search_after` cursor leaks user data | Cursor is opaque base64 of `[score, time, id]`. Treat it as untrusted on the way in. |
| Postgres trigram extension missing in prod | Migration must include `CREATE EXTENSION IF NOT EXISTS pg_trgm;`. Ship in its own migration file. |

---

## 9. Success metrics

- **Zero-result rate**: % of searches that return 0 hits. Target: < 5%.
- **p95 latency**: target < 200 ms for blog search, < 100 ms for people search.
- **Click-through rate**: % of search sessions where the user clicks a result. Target: > 40%.
- **Cache hit rate**: target > 50% during peak.

We will look at all four weekly for the first month after launch.

---

## 10. Cross-references

- Implementation plan: [SEARCH_V2_IMPLEMENTATION_PLAN.md](SEARCH_V2_IMPLEMENTATION_PLAN.md)
- Team video script + Q&A: [SEARCH_V2_VIDEO_SCRIPT.md](SEARCH_V2_VIDEO_SCRIPT.md)
- Existing blog mapping: [documents/reindex/mapping_v2.json](../reindex/mapping_v2.json)
