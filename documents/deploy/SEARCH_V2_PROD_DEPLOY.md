# Search v2 — Production Deployment Runbook

Step-wise procedure to roll Search v2 (Phases 0–5) onto a live cluster.
Estimated wall-clock: **20–30 min** of work, plus a 1-week soak before
the final cleanup.

> **Prod topology assumed by this runbook**
>
> - **App services** (`the_monkeys_gateway`, `the_monkeys_user`,
>   `the_monkeys_blog`, etc.) run under `docker compose` on the prod node.
> - **Postgres** runs **directly on the host** (systemd / managed
>   service), not inside a container. Reached over TCP at
>   `$POSTGRES_HOST:$POSTGRES_PORT` from the app containers and from
>   your shell on the host.
> - **Elasticsearch** runs **directly on the host**, reached at
>   `$OPENSEARCH_HOST:$OPENSEARCH_HTTP_PORT` (typically
>   `http://127.0.0.1:9200` on the host; whatever the app `.env`
>   resolves `OPENSEARCH_OS_HOST` to).
> - You have shell on the prod node (sudo for systemd ops), the prod
>   `.env`, the `psql` CLI, the `migrate` CLI (or a one-off migrate
>   container), and Go toolchain available somewhere (host or jump
>   box) to run the reindex script.
>
> Every step below uses `psql` / `curl` directly. There is no
> `docker exec the-monkeys-psql` or `docker exec elasticsearch-node1`
> because those containers don't exist in this environment.

Define these once at the top of your shell:

```bash
export PG_DSN="host=$POSTGRES_HOST port=$POSTGRES_PORT dbname=$POSTGRESQL_PRIMARY_DB_DB_NAME user=$POSTGRESQL_PRIMARY_DB_DB_USERNAME password=$POSTGRESQL_PRIMARY_DB_DB_PASSWORD sslmode=require"
export ES="http://$OPENSEARCH_HOST:$OPENSEARCH_HTTP_PORT"
```

(Adjust `sslmode` to match prod — `require` / `verify-full` / `disable`.)

---

## 0. Pre-flight — take snapshots (non-negotiable)

```bash
# Postgres logical dump (run as a user that can read all blog/user tables,
# typically the same role the app uses, or postgres).
PGPASSWORD="$POSTGRESQL_PRIMARY_DB_DB_PASSWORD" pg_dump \
  -h "$POSTGRES_HOST" -p "$POSTGRES_PORT" \
  -U "$POSTGRESQL_PRIMARY_DB_DB_USERNAME" \
  -d "$POSTGRESQL_PRIMARY_DB_DB_NAME" \
  -Fc -f "/var/backups/postgres/preSearchV2_$(date +%F).dump"

# Elasticsearch snapshot (repo `local-fs` must already be registered on
# the host ES; see elasticsearch_snapshots/ for the path.repo setting in
# elasticsearch.yml).
curl -s -X PUT "$ES/_snapshot/local-fs/pre_search_v2_$(date +%F)?wait_for_completion=true" \
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

Postgres is on the host, so we apply migrations with the `migrate` CLI
directly against `$PG_DSN`. **Do not** rely on a `db-migrations`
compose service to reach a host Postgres unless its connection string
in `.env` already targets the host correctly (some prod compose files
strip the migrate sidecar entirely).

```bash
# Pin the same migrate version the project uses in dev (v4.15.2).
MIGRATE="migrate -path ./schema -database 'postgres://$POSTGRESQL_PRIMARY_DB_DB_USERNAME:$POSTGRESQL_PRIMARY_DB_DB_PASSWORD@$POSTGRES_HOST:$POSTGRES_PORT/$POSTGRESQL_PRIMARY_DB_DB_NAME?sslmode=require'"

# Current schema version (sanity check before migrating)
eval $MIGRATE version

# Apply v6 + v7
eval $MIGRATE up 2

# Verify
PGPASSWORD="$POSTGRESQL_PRIMARY_DB_DB_PASSWORD" psql \
  -h "$POSTGRES_HOST" -p "$POSTGRES_PORT" \
  -U "$POSTGRESQL_PRIMARY_DB_DB_USERNAME" \
  -d "$POSTGRESQL_PRIMARY_DB_DB_NAME" \
  -c "SELECT version, dirty FROM schema_migrations;"
```

Expected: `version=7  dirty=f`.

If you don't have `migrate` on the host, run it as a one-shot container
bound to the host network so it can see Postgres on `127.0.0.1`:

```bash
docker run --rm --network=host \
  -v "$PWD/schema:/migrations" migrate/migrate:v4.15.2 \
  -path=/migrations \
  -database "postgres://$POSTGRESQL_PRIMARY_DB_DB_USERNAME:$POSTGRESQL_PRIMARY_DB_DB_PASSWORD@127.0.0.1:$POSTGRES_PORT/$POSTGRESQL_PRIMARY_DB_DB_NAME?sslmode=require" \
  up 2
```

What v6 + v7 do:

- **v6** enables `pg_trgm` (CREATE EXTENSION). Idempotent. Requires
  superuser the first time it runs in prod — if your app role can't
  `CREATE EXTENSION`, run this single statement as `postgres` manually,
  then `migrate force 6` and continue.
- **v7** adds the STORED `search_doc` column on `user_account` plus two
  GIN trigram indexes. The `CREATE INDEX` is **not** `CONCURRENTLY` —
  on the prod table (≲100k users today) the AccessExclusiveLock is
  measured in seconds, but if your user table is larger, take a brief
  read-pause OR pre-create the indexes manually with `CONCURRENTLY`
  and then `migrate force 7`.

Rollback:

```bash
eval $MIGRATE down 2
```

---

## 3. Build the new Elasticsearch index

`the_monkeys_blogs` is an **alias** in prod pointing at a backing
index (`BACKING`). Verified in dev (ES 8.16.1) and confirmed by
reading the live config:

```
alias              index                 is_write_index
the_monkeys_blogs  the_monkeys_blogs_v2  -
```

So prod's `BACKING = the_monkeys_blogs_v2` unless your ES says
otherwise. **Confirm before doing anything**:

```bash
curl -s -u "elastic:pass" "$ES/_cat/aliases/the_monkeys_blogs?v"
# record the value in the `index` column — that's BACKING.

# Abort if v3 already exists (means someone half-ran this before):
curl -s -u "elastic:pass" -o /dev/null -w '%{http_code}\n' \
  "$ES/the_monkeys_blogs_v3"
# expected: 404. If 200, investigate before continuing.
```

Two files in the current working directory:

- `mapping_v3.json` — copy of [documents/reindex/mapping_v3.json](../reindex/mapping_v3.json).
- `reindex.json` — the reindex body:

  ```json
  {
    "source": { "index": "the_monkeys_blogs" },
    "dest":   { "index": "the_monkeys_blogs_v3" }
  }
  ```

  ES resolves `source.index` through the alias to `BACKING`, so this
  works without hardcoding the backing name.

### Linux

```bash
# 3.1 — create v3 with the new mapping
curl -u "elastic:pass" -X PUT "$ES/the_monkeys_blogs_v3" \
  -H "Content-Type: application/json" -d @mapping_v3.json

# 3.2 — reindex (blocking; for large corpora drop wait_for_completion
#        and poll /_tasks/<id> instead)
curl -u "elastic:pass" -X POST "$ES/_reindex?wait_for_completion=true" \
  -H "Content-Type: application/json" -d @reindex.json

# 3.3 — verify counts before the alias swap
curl -s -u "elastic:pass" "$ES/the_monkeys_blogs/_count"
curl -s -u "elastic:pass" "$ES/the_monkeys_blogs_v3/_count"
# counts MUST match (dev currently shows alias=566, v3=566)
```

### Windows (PowerShell, using `curl.exe`)

```powershell
cmd /c "curl.exe -X PUT http://localhost:9200/the_monkeys_blogs_v3 -H ""Content-Type: application/json"" -d @mapping_v3.json"

cmd /c "curl.exe -X POST http://localhost:9200/_reindex?wait_for_completion=true -H ""Content-Type: application/json"" -d @reindex.json"

curl.exe http://localhost:9200/the_monkeys_blogs/_count
curl.exe http://localhost:9200/the_monkeys_blogs_v3/_count
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
curl -s -u "elastic:pass" -X POST "$ES/the_monkeys_blogs_v3/_search" \
  -H 'Content-Type: application/json' \
  -d '{"query":{"term":{"blog_id":"<known_blog_id>"}},"_source":false}' \
  | jq '.hits.total'
# expected: { value: 1, relation: "eq" }
```

If you ever apply `mapping_v3.json` on an older blog-svc image that
still has `.keyword` in its queries, GET-by-id breaks in prod. Roll
the blog image forward first.

---

## 4. Atomic alias swap (the actual cut-over)

Single request — ES applies all actions atomically. Zero downtime,
no 404 window. **Do NOT `DELETE /the_monkeys_blogs`** — since it's
an alias, that would delete the backing index `BACKING` and destroy
your rollback path.

Note: dev's alias has no `is_write_index` set and writes work fine
because the alias resolves to a single index. We match that here.

### Linux

```bash
curl -u "elastic:pass" -X POST "$ES/_aliases" \
  -H "Content-Type: application/json" \
  -d '{
    "actions": [
      { "remove": { "index": "*",                    "alias": "the_monkeys_blogs" } },
      { "add":    { "index": "the_monkeys_blogs_v3", "alias": "the_monkeys_blogs" } }
    ]
  }'

curl -s -u "elastic:pass" "$ES/_cat/aliases/the_monkeys_blogs?v"
# expected:
# alias              index                 ... is_write_index
# the_monkeys_blogs  the_monkeys_blogs_v3  ... -
```

### Windows (PowerShell)

```powershell
cmd /c "curl.exe -X POST http://localhost:9200/_aliases -H ""Content-Type: application/json"" -d ""{\""actions\"":[{\""remove\"":{\""index\"":\""*\"",\""alias\"":\""the_monkeys_blogs\""}},{\""add\"":{\""index\"":\""the_monkeys_blogs_v3\"",\""alias\"":\""the_monkeys_blogs\""}}]}"""

curl.exe http://localhost:9200/_cat/aliases/the_monkeys_blogs?v
```

**Rollback (one liner):** re-issue the same `POST /_aliases` with
`BACKING` (recorded in §3, expected: `the_monkeys_blogs_v2`) in
place of `the_monkeys_blogs_v3`. Keep `BACKING` around for at least
7 days; drop it in §9.

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
# expected: "blogsearch: connected  addr=<your $OPENSEARCH_OS_HOST> alias=the_monkeys_blogs"
```

### Container → host networking (CRITICAL)

Because ES and Postgres run on the **host**, the app containers cannot
reach them on `127.0.0.1` inside the container. Pick **one** of these
in your `docker-compose.yml` / `.env` and verify it before the rollout:

1. **`network_mode: host`** on the affected services (Linux only). The
   container shares the host network namespace, so `127.0.0.1:5432`
   and `127.0.0.1:9200` Just Work. Simplest, but you lose Docker's
   service-to-service DNS.
2. **Bridge network + `host.docker.internal`** (or the host's LAN IP /
   `172.17.0.1` on Linux). Set in `.env`:

   ```env
   POSTGRES_HOST=host.docker.internal
   OPENSEARCH_OS_HOST=http://host.docker.internal:9200
   ```

   On Linux, add `extra_hosts: ["host.docker.internal:host-gateway"]`
   to each app service in `docker-compose.yml`.
3. **Explicit LAN IP / hostname** of the prod node. Cleanest if you ever
   plan to split the app tier off the DB tier onto separate hosts.

Failure modes you'll see in the gateway log if this is wrong:

- `dial tcp [::1]:9200: connect: connection refused` → `OPENSEARCH_OS_HOST`
  is pointing at the loopback inside the container. Fix per option 2
  or 3 above.
- `dial tcp 127.0.0.1:5432: connect: connection refused` → same problem
  for Postgres.
- `dial tcp <ip>:9200: i/o timeout` → host firewall blocks the bridge
  subnet. Allow `172.17.0.0/16` (or whatever the compose network CIDR
  is) inbound to ES / Postgres.

Also make sure the **host** services bind on an address the containers
can reach, not just `127.0.0.1`:

- Postgres `postgresql.conf`: `listen_addresses = '127.0.0.1, 172.17.0.1'`
  (or `*` + tight `pg_hba.conf`).
- Elasticsearch `elasticsearch.yml`: `network.host: 0.0.0.0` (bound to a
  private interface, never the public one), and an ES-level allow-list
  if you run with security enabled.

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

Only after the 24h metrics look good AND a full week of v3 traffic.
Replace `<BACKING>` with the old backing index you recorded in §3.

```bash
# Sanity: alias still on v3?
curl -s -u "elastic:pass" "$ES/_cat/aliases/the_monkeys_blogs?v"

# Final snapshot of the old backing index (paranoia)
curl -s -u "elastic:pass" -X PUT \
  "$ES/_snapshot/local-fs/pre_drop_backing_$(date +%F)?wait_for_completion=true" \
  -H 'Content-Type: application/json' \
  -d '{"indices":"<BACKING>","include_global_state":false}'

# Drop the old backing index
curl -s -u "elastic:pass" -X DELETE "$ES/<BACKING>"
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
| ES v3 corrupted mid-reindex                              | `POST _aliases` to swing alias back to `<BACKING>` (recorded in §3). v3 can be rebuilt.                                                                                            |
| Postgres v7 migration fails dirty                        | `migrate force 6`, drop the indexes, fix, re-run.                                                                                                                            |
| Front-end calls 404                                      | Old front-end shipped against new gateway — redeploy front-end immediately.                                                                                                  |
| App container logs `dial tcp 127.0.0.1:5432` or `[::1]:9200` refused | Container is resolving DB/ES to its own loopback. ES + Postgres are on the **host** in prod — fix `POSTGRES_HOST` / `OPENSEARCH_OS_HOST` per §5 (use `host.docker.internal` + `extra_hosts`, the LAN IP, or `network_mode: host`). |
| App container logs `dial tcp <host-ip>:9200 i/o timeout` | Host firewall is blocking the docker bridge. Allow the compose network CIDR (commonly `172.17.0.0/16`) inbound to ES/Postgres on the host. |

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
