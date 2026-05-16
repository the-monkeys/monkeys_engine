# Search v2: Implementation Plan

> Companion to [SEARCH_V2_DESIGN.md](SEARCH_V2_DESIGN.md). This file is the **build order**: what we change, in what sequence, with checkpoints.
>
> The plan is split into 5 phases. Each phase is shippable on its own — we never have a "big bang" merge.

---

## Phase 0 — Foundations (1–2 days)

Goal: nothing user-visible. Get the platform ready so the rest is safe.

- [x] Add Redis client wrapper in the gateway with a tiny `searchcache` package (`Get`, `Set`, `Del` with key prefix `search:v2:`).
- [x] Add a structured-log helper for search events (zap field set: `q`, `type`, `latency_ms`, `result_count`, `cache_hit`, `user_id_hash`).
- [x] Postgres migration `schema/000006_enable_pg_trgm.up.sql`:
  ```sql
  CREATE EXTENSION IF NOT EXISTS pg_trgm;
  ```
  Down migration drops it (`IF EXISTS`, no cascade).
- [ ] _Deferred_: Prometheus metrics namespace. No metrics infra exists in the repo yet; we will instead use the structured `search_event` log line as our primary telemetry until a Prom exporter lands. Re-open in Phase 5.

Acceptance: app boots, no behaviour change, `pg_trgm` available in dev DB.

---

## Phase 1 — People search v2 (2–3 days)

Smallest blast radius, biggest visible win.

- [x] Migration `schema/000007_user_search_index.up.sql`:
  ```sql
  ALTER TABLE user_account
    ADD COLUMN IF NOT EXISTS search_doc TEXT
    GENERATED ALWAYS AS (
      coalesce(username,'')   || ' ' ||
      coalesce(first_name,'') || ' ' ||
      coalesce(last_name,'')  || ' ' ||
      coalesce(bio,'')
    ) STORED;

  CREATE INDEX IF NOT EXISTS idx_user_account_search_doc_trgm
    ON user_account USING GIN (search_doc gin_trgm_ops);

  CREATE INDEX IF NOT EXISTS idx_user_account_username_trgm
    ON user_account USING GIN (username gin_trgm_ops);
  ```
- [x] User service: new method `FindUsersV2(ctx, term, limit, offset)` in `database/user_search_v2.go`. Uses ranked SQL from §6.4. Caps `limit` to 50 server side.
- [x] User service: rewired existing `SearchUser` bidi RPC to call `FindUsersV2` (proto unchanged this phase; unary refactor deferred to avoid stub regen). Bounded 500ms server-side context.
- [x] Gateway: new `SearchUserV2` handler with `context.WithTimeout(ctx, 500*time.Millisecond)`, query length 1–128, limit cap 50, Redis read-through (30 s TTL), structured `search_event` log.
- [x] Gateway: route `GET /api/v2/user/search` registered behind `RateLimiterMiddleware("100-S")`.
- [ ] Unit tests: exact-username wins, trigram match wins over no match, inactive users are excluded, limit cap enforced.

Acceptance: `EXPLAIN ANALYZE` shows GIN index used; p95 < 100 ms on a 100k-row seed; v1 endpoint still works.

---

## Phase 2 — Blog search v2 indexing (3–5 days)

The mapping/indexing work. No new query yet.

- [x] Create the new mapping file `documents/reindex/mapping_v3.json`. Custom `blog_text_analyzer` (light english stem + asciifold + possessive strip), edge-ngram autocomplete subfield on `title.autocomplete`, `lowercase_keyword` normalizer for case-insensitive exact match. Flat search fields denormalised alongside the preserved legacy `blog.*` subtree.
- [x] Alias plan documented in `scripts/reindex_blogs_v3/README.md`: `the_monkeys_blogs` → `..._v3` after reindex via atomic `_aliases` swap.
- [x] Blog service write path: `SaveBlog` now calls `searchdoc.Apply` to populate `title` / `summary` / `body` / `tags` from the editorjs blocks. Same helper backs the reindex job so live and backfilled docs are byte-identical.
- [x] Reindex job: `scripts/reindex_blogs_v3/main.go` — bounded-memory scroll + bulk in 500-doc batches, idempotent on `blog_id`, `-dry-run` and `-require-dst` flags.
- [x] Cut-over: PowerShell `_aliases` swap snippet in the README (atomic). Roll-back snippet included.

Acceptance: count of docs in v3 == v2; spot-check 20 random blogs and confirm `title`, `summary`, `body` look correct; old query still works against the alias (because we did not change it yet).

---

## Phase 3 — Blog search v2 query path (2–3 days)

- [x] New ES query builder in `microservices/the_monkeys_gateway/internal/blogsearch/query.go`. `function_score` (gauss recency 90d + log1p(like_count)) wrapping `multi_match` (best_fields, AND, fuzziness AUTO) over `title^4` / `summary^2` / `body` / `tags.text^2` / `author_*`. Highlights on title/summary/body with `<mark>` tags. `search_after` cursor pagination (no `from`, ever).
- [x] Implemented at the gateway tier instead of a new gRPC RPC to avoid proto regen across services; talks to the `the_monkeys_blogs` alias so it tracks the Phase 2 cut-over automatically. Response includes opaque `next_cursor` (base64(json(sort vector))).
- [x] Gateway handler `GET /api/v2/blog/search/v2`. Rate limit 30/s, 1 s ES deadline, query 1–128 chars, limit cap 50. Redis read-through TTL 60 s.
- [x] `GET /api/v2/search/suggest?q=`. Match on `title.autocomplete` only, size ≤10, 300 ms deadline. Fails closed (empty list) on any backend error so the typing UX never shows a 5xx.

Acceptance: searching the exact title returns that blog as the first result; typing `monky` finds `monkey`; deep pagination via cursor stays under 200 ms.

---

## Phase 4 — Front-end rewrite (3–4 days)

- [x] New hooks in `src/hooks/search/useSearchV2.ts`: `useSearchPeopleV2`, `useSearchBlogsV2` (cursor), `useSearchSuggest`. React Query v5, 250 ms debounce lives in `SearchInput`, `placeholderData: prev` for smooth pagination.
- [x] New autocomplete hook `useSearchSuggest(q)` hitting `/api/v2/search/suggest` (gated to `q.length >= 2`, staleTime 15 s).
- [ ] _Deferred to follow-up_: Wire `useSearchStore` for recent-query ring buffer + result-id cache. Not required by acceptance criteria; tracked in Phase 5 backlog.
- [x] `SearchInput` dropdown:
  - [ ] Recent searches when empty + focused — _deferred with `useSearchStore` work above_.
  - [ ] Live suggestion list — _hook ready (`useSearchSuggest`), UI wiring deferred to follow-up_.
  - [x] "See all results for *X* →" footer link (mousedown+click to beat blur).
- [x] Result cards render `dangerouslySetInnerHTML` of highlight HTML through `sanitizeHighlight` (DOMPurify allow-list: only `<mark>`, no attributes) for `title` and `summary`.
- [x] Switched the `/search` page to v2: posts use cursor pagination via `useSearchBlogsV2`; authors use offset via `useSearchPeopleV2` (people endpoint exposes offset, not cursor).
- [x] Removed `searchUsers.slice(0, 5)` / `searchBlogs.slice(0, 3)` client truncation; v2 hooks request the exact `limit` rendered.

Acceptance: Chrome devtools shows ≤ 1 request per 250 ms while typing; pressing Esc cancels in-flight requests; refresh of `/search?cursor=…` returns the same page 2; matched words are bolded.

---

## Phase 5 — Cleanup & deprecation (1 day)

- [x] Remove the old endpoints: `/api/v2/blog/search` (old query handler) and `/api/v1/user/search` now issue `308 Permanent Redirect` to the v2 paths, preserving the raw query string. Dead handler bodies (`BlogServiceClient.SearchBlogsQuery`, the legacy `UserServiceClient.SearchUser` stream wiring) removed. Old hooks `useGetSearchBlog`, `useGetSearchUser` and the unused `useSearchStore` deleted; legacy `SearchUser` / `GetUserSearchResponse` types removed from `searchTypes.ts`.
- [ ] _Operational, post-deploy_: Drop `the_monkeys_blogs_v2` index after one week of clean v3 traffic. Procedure documented in [scripts/reindex_blogs_v3/README.md](../../scripts/reindex_blogs_v3/README.md).
- [x] Updated API docs: [documents/apis/blog_svc_v2.yaml](../apis/blog_svc_v2.yaml) adds `/blog/search/v2` and `/search/suggest`, marks `/blog/search` as deprecated (308); [documents/apis/user_service.yaml](../apis/user_service.yaml) adds `/v2/user/search` and marks `/v1/user/search` as deprecated (308).
- [ ] _Post-launch_: Review the success metrics in §9 of the design doc against a 24h sample. Open follow-up tickets for misses.

---

## Sequencing summary

```
Day  1   2   3   4   5   6   7   8   9  10  11  12  13
     ├P0─┼───────P1───────┼─────P2─────────┼───P3───┼───P4───┼P5
```

(Estimates assume one engineer focused on this; can compress with parallel work on P2 indexing + P4 front-end since they touch different layers.)

---

## Roll-back plan

Every phase is reversible:

| Phase | How to roll back |
|---|---|
| P0 | Drop the extension (no rows depend on it yet). |
| P1 | Drop the GIN indexes + `search_doc` column; gateway falls back to v1 endpoint. |
| P2 | Point alias `the_monkeys_blogs` back at `..._v2`; delete `..._v3`. |
| P3 | Disable the v2 route at the gateway; old `/blog/search` still works. |
| P4 | Revert the front-end PR; old hooks still call the old endpoints (both live for one release on purpose). |
| P5 | Cleanup is the only non-reversible step. We only run it after one week of clean metrics. |

---

## Definition of done for the whole project

1. All success metrics from §9 of the design doc are tracked on a Grafana board.
2. Zero-result rate measured over a 24-hour sample is below 5%.
3. p95 search latency over a 1-hour peak is under 200 ms (blog) and 100 ms (user).
4. Old endpoints removed, v1 cache cleared.
5. Design doc, this plan, and the video script are linked from the team handbook.
