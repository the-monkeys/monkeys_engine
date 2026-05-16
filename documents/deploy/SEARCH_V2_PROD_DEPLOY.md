# Search v2 — Production Deployment Runbook

Step-wise procedure to roll Search v2 (Phases 0–5) onto a live cluster.
Estimated wall-clock: **20–30 min** of work, plus a 1-week soak before
the final cleanup.

> Prerequisites: shell access to the prod node running `docker compose`,
> the prod `.env`, `kubectl` or `docker exec` reach into the
> `elasticsearch-node1` and `the-monkeys-psql` containers, and a recent
> snapshot taken (step 0).

---

## 0. Pre-flight — take snapshots (non-negotiable)

```bash
# Postgres logical dump
docker exec the-monkeys-psql pg_dump -U $POSTGRESQL_PRIMARY_DB_DB_USERNAME \
  -d $POSTGRESQL_PRIMARY_DB_DB_NAME -Fc -f /backup/preSearchV2_$(date +%F).dump

# Elasticsearch snapshot (repo `local-fs` must already exist; see
# elasticsearch_snapshots/ in the repo for the local-fs configuration).
ES=http://localhost:$OPENSEARCH_HTTP_PORT
curl -X PUT "$ES/_snapshot/local-fs/pre_search_v2_$(date +%F)?wait_for_completion=true" \
  -H 'Content-Type: application/json' \
  -d '{"indices":"the_monkeys_blogs*","include_global_state":false}'
```

Confirm both succeeded before touching anything else.

---

## 1. Ship the new images

```bash
git fetch --tags
git checkout <release-tag>          # e.g. v1.5.0-search-v2
docker compose --env-file .env build --pull \
  the_monkeys_gateway the_monkeys_user the_monkeys_blog
```

Builds are read-only; no traffic impact yet.

---

## 2. Apply Postgres migrations (v6 + v7)

Migrations land automatically on `compose up`, but applying them
first — before bringing new app containers up — surfaces failures
early and keeps the cut-over window small.

```bash
docker compose --env-file .env up -d --no-deps db-migrations
docker logs the-monkeys-migrate --tail 20
docker exec the-monkeys-psql psql -U $POSTGRESQL_PRIMARY_DB_DB_USERNAME \
  -d $POSTGRESQL_PRIMARY_DB_DB_NAME \
  -c "SELECT version, dirty FROM schema_migrations;"
```

Expected: `version=7  dirty=f`.

What v6 + v7 do:

- **v6** enables `pg_trgm` (CREATE EXTENSION). Idempotent.
- **v7** adds the STORED `search_doc` column on `user_account` plus two
  GIN trigram indexes. The `CREATE INDEX` is **not** `CONCURRENTLY` —
  on the prod table (≲100k users today) the AccessExclusiveLock is
  measured in seconds, but if your user table is larger, take a brief
  read-pause OR pre-create the indexes manually with `CONCURRENTLY`
  and then `migrate force 7`.

Rollback:

```bash
docker compose --env-file .env run --rm db-migrations down 2
```

---

## 3. Build the new Elasticsearch index

ES requires a manual cut-over because the new mapping (`mapping_v3.json`)
is incompatible with the live `..._v2` mapping — you cannot change an
analyzer on an existing field in place.

```bash
ES=http://localhost:$OPENSEARCH_HTTP_PORT

# 3.1 — create the v3 index with the new mapping
docker cp documents/reindex/mapping_v3.json elasticsearch-node1:/tmp/mapping_v3.json
docker exec elasticsearch-node1 curl -s -X PUT "$ES/the_monkeys_blogs_v3" \
  -H 'Content-Type: application/json' --data-binary "@/tmp/mapping_v3.json"
# expect: {"acknowledged":true,"shards_acknowledged":true,"index":"..."}

# 3.2 — dry-run the reindex to catch transform errors before touching v3
ES_ADDR=$ES go run ./scripts/reindex_blogs_v3 \
  -src the_monkeys_blogs_v2 -dst the_monkeys_blogs_v3 -dry-run

# 3.3 — for real (idempotent; safe to re-run)
ES_ADDR=$ES go run ./scripts/reindex_blogs_v3 \
  -src the_monkeys_blogs_v2 -dst the_monkeys_blogs_v3
```

Verify:

```bash
curl -s $ES/the_monkeys_blogs_v2/_count
curl -s $ES/the_monkeys_blogs_v3/_count
# Counts MUST match. Investigate before continuing if they don't.
```

Spot-check a denormalised doc:

```bash
curl -s "$ES/the_monkeys_blogs_v3/_search?size=2&_source=blog_id,title,summary,tags"
```

### 3.1 — Mapping breaking change you MUST be aware of

`mapping_v3.json` declares `blog_id`, `owner_account_id`, and `tags` as
**top-level `keyword`** fields. The legacy v2 mapping had them as `text`
with a `.keyword` sub-field. Every `term` query in the blog service
that used `"<field>.keyword"` returns **zero hits** against v3 — and ES
does not error on a non-existent field, so the failure mode is silent
404s on:

- `GET /api/v2/blog/:blog_id`
- `GET /api/v2/blog/:blog_id/stats`
- `GET /api/v2/blog/user/:username`
- `GET /api/v2/blog/trending`
- archive / delete / bookmark code paths

The fix is already in this release: 25 callsites across
[opensearch.go](../../microservices/the_monkeys_blog/internal/database/opensearch.go),
[v2_queries.go](../../microservices/the_monkeys_blog/internal/database/v2_queries.go),
[query.go](../../microservices/the_monkeys_blog/internal/database/query.go),
[blogs_matadata.go](../../microservices/the_monkeys_blog/internal/database/blogs_matadata.go)
were changed from `"blog_id.keyword"` → `"blog_id"` (same for
`owner_account_id` and `tags`). It is backward-compatible with the v2
mapping because the legacy `text` field tokenises blog IDs as a single
lowercase token.

Verification before the alias swap:

```bash
# Sanity: pick any known blog id and confirm v3 returns it via top-level field
curl -s -X POST "$ES/the_monkeys_blogs_v3/_search" -H 'Content-Type: application/json' \
  -d '{"query":{"term":{"blog_id":"<known_blog_id>"}},"_source":false}' | jq '.hits.total'
# expected: { value: 1, relation: "eq" }
```

If you ever cherry-pick the `mapping_v3.json` onto an older blog-svc
image that still has `.keyword` in its queries, GET-by-id breaks in
prod. Roll the blog image forward first, or apply the same s/\.keyword//
fix to the older code.

---

## 4. Atomic alias swap (the actual cut-over)

The application reads/writes through the alias `the_monkeys_blogs`,
never the physical index, so this single request flips traffic with
zero downtime.

```bash
ES=http://localhost:$OPENSEARCH_HTTP_PORT
curl -s -X POST "$ES/_aliases" -H 'Content-Type: application/json' -d '{
  "actions": [
    { "remove": { "index": "*",                    "alias": "the_monkeys_blogs" } },
    { "add":    { "index": "the_monkeys_blogs_v3", "alias": "the_monkeys_blogs" } }
  ]
}'

curl -s "$ES/_cat/aliases/the_monkeys_blogs?v"
# expected: the_monkeys_blogs -> the_monkeys_blogs_v3
```

**Rollback (one liner):** swap the index name back to `..._v2` and
re-issue. Keep `..._v2` around for at least 7 days.

---

## 5. Roll the application containers

Now everything the new code needs is in place. Recreate the services
that changed:

```bash
docker compose --env-file .env up -d \
  the_monkeys_gateway the_monkeys_user the_monkeys_blog
```

Watch the gateway boot log for the search-v2 init line:

```bash
docker logs the-monkeys-gateway --tail 50 | grep -Ei 'blogsearch|opensearch'
# expected: "blogsearch: connected  addr=http://elasticsearch-node1:9200 alias=the_monkeys_blogs"
```

If the gateway logs `dial tcp [::1]:9201: connect: connection refused`,
the OpenSearch env is wrong: the gateway must use **`OPENSEARCH_OS_HOST`**
(container-network), not `OPENSEARCH_ADDRESS` (host-network). Fix the
`.env` and `docker compose up -d the_monkeys_gateway` again.

---

## 6. Smoke test through the gateway

Replace `:8081` with your prod gateway port.

```bash
G=http://gateway.prod:8081

# People search (Postgres v2)
curl -s "$G/api/v2/user/search?search_term=a&limit=2" | jq .

# Blog search (ES v3)
curl -s "$G/api/v2/blog/search/v2?q=python&limit=3" | jq '.hits[] | {blog_id,title,score}'

# Autocomplete
curl -s "$G/api/v2/search/suggest?q=ru&limit=3" | jq .

# Legacy paths must 308 (preserves query string + GET method)
curl -sI "$G/api/v1/user/search?search_term=admin" | grep -E '^(HTTP/|Location:)'
curl -sI "$G/api/v2/blog/search?search_term=python"  | grep -E '^(HTTP/|Location:)'

# CRITICAL — non-search GET paths that read from the alias.
# These break silently if .keyword fix from §3.1 is not in the image.
curl -s -w '\nHTTP=%{http_code}\n' "$G/api/v2/blog/<known_published_blog_id>" | tail -c 200
curl -s -w '\nHTTP=%{http_code}\n' "$G/api/v2/blog/user/<known_username>" | tail -c 200
curl -s -w '\nHTTP=%{http_code}\n' "$G/api/v2/blog/trending?limit=3" | tail -c 200
```

Pass criteria:

- People returns ranked users (`limit`, `offset` echoed).
- Blog returns `hits[]` with `score`, `took_ms < 800`.
- Suggest returns `<=3` titles for a 2+ char prefix.
- Both legacy paths return `308` with a `Location:` pointing at the
  v2 endpoint and identical query string.
- `GET /api/v2/blog/:id` returns `200` with body — **not** `404 the blog
  does not exist`. A 404 here means the running blog-svc image is
  missing the §3.1 keyword fix; do not proceed.

---

## 7. Front-end deploy

The Next.js app must be redeployed alongside or after the gateway
(never before — the new hooks call endpoints that don't exist on the
old gateway).

```bash
cd local/the_monkeys/apps/the_monkeys
pnpm install --frozen-lockfile
pnpm build
# deploy `.next/` via your usual mechanism (Vercel, k8s, etc.)
```

Acceptance in browser:

- DevTools shows **≤ 1 request per 250 ms** while typing in the navbar.
- Search results bold matched words (`<mark>` tags via DOMPurify).
- `/search?query=…` paginates with cursor (Next/Prev work, no `?page=`
  parameter appears in the URL for the posts tab).

---

## 8. Observability checks (first 24h)

Watch `search_event` log lines from the gateway (they're emitted by
`searchcache.LogSearchEvent` on every hit):

```bash
docker logs -f the-monkeys-gateway | grep search_event
```

Key fields and thresholds:

| Field          | Healthy        | Investigate if         |
| -------------- | -------------- | ---------------------- |
| `cache_hit`    | rising over h1 | flat 0% after warm-up  |
| `latency_ms`   | p95 < 200      | p95 > 500 sustained    |
| `zero_result`  | < 5%           | > 15%                  |
| `result_count` | non-zero mode  | mode 0 → broken filter |

A sustained spike in `zero_result` is the canary for the
`is_archived/is_draft` filter bug we hit in dev — see §10 below.

---

## 9. Cleanup — **schedule for T+7 days**

Only after the 24h metrics look good AND a full week of v3 traffic:

```bash
ES=http://localhost:$OPENSEARCH_HTTP_PORT

# Sanity: alias still on v3?
curl -s "$ES/_cat/aliases/the_monkeys_blogs?v"

# Final snapshot of v2 (paranoia)
curl -s -X PUT "$ES/_snapshot/local-fs/pre_drop_v2_$(date +%F)?wait_for_completion=true" \
  -H 'Content-Type: application/json' \
  -d '{"indices":"the_monkeys_blogs_v2","include_global_state":false}'

# Drop the old index
curl -s -X DELETE "$ES/the_monkeys_blogs_v2"
```

The legacy `/api/v1/user/search` and `/api/v2/blog/search` 308 shims
stay in place for one more release after that, then are deleted.

---

## 10. Roll-back playbook

| Symptom                                                  | Action                                                                                                                                                                       |
| -------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Gateway 5xx on any `/api/v2/*search`                     | Re-deploy previous gateway image. Cache and ES are unaffected.                                                                                                               |
| Blog search results suddenly all zero                    | Verify alias → v3, then check `is_draft/is_archived` field presence (see §6 above).                                                                                          |
| `GET /api/v2/blog/:id` returns 404 "the blog does not exist" but doc exists in ES | Blog-svc image still queries `"blog_id.keyword"` against v3. Either roll the blog image forward, or **temporarily** swing alias back to `_v2` until the new image is in place. See §3.1. |
| ES v3 corrupted mid-reindex                              | `POST _aliases` to swing alias back to `the_monkeys_blogs_v2`. v3 can be rebuilt.                                                                                            |
| Postgres v7 migration fails dirty                        | `migrate force 6`, drop the indexes, fix, re-run.                                                                                                                            |
| Front-end calls 404                                      | Old front-end shipped against new gateway — redeploy front-end immediately.                                                                                                  |

Every step in §§1–5 is independently reversible. Cleanup (§9) is the
only destructive action and is intentionally gated behind a one-week
soak.

---

## Appendix A — Files & paths referenced

- Migrations: [schema/000006_enable_pg_trgm.up.sql](../../schema/000006_enable_pg_trgm.up.sql), [schema/000007_user_search_index.up.sql](../../schema/000007_user_search_index.up.sql)
- ES mapping: [documents/reindex/mapping_v3.json](../reindex/mapping_v3.json)
- Reindex script: [scripts/reindex_blogs_v3/main.go](../../scripts/reindex_blogs_v3/main.go) (full README: [scripts/reindex_blogs_v3/README.md](../../scripts/reindex_blogs_v3/README.md))
- Gateway search package: [microservices/the_monkeys_gateway/internal/blogsearch/](../../microservices/the_monkeys_gateway/internal/blogsearch/)
- Cache + event log: [microservices/the_monkeys_gateway/internal/cache/searchcache/](../../microservices/the_monkeys_gateway/internal/cache/searchcache/)
- API contracts: [documents/apis/blog_svc_v2.yaml](../apis/blog_svc_v2.yaml), [documents/apis/user_service.yaml](../apis/user_service.yaml)
- Original design: [documents/design/SEARCH_V2_IMPLEMENTATION_PLAN.md](../design/SEARCH_V2_IMPLEMENTATION_PLAN.md)
