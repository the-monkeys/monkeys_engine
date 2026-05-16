# Reindex blogs → v3 (Search-v2 Phase 2)

This directory contains the one-shot job that migrates the legacy
`the_monkeys_blogs` / `the_monkeys_blogs_v2` Elasticsearch index to the
new `the_monkeys_blogs_v3` index defined in
[`documents/reindex/mapping_v3.json`](../../documents/reindex/mapping_v3.json).

The end state is:

```
alias the_monkeys_blogs ──► the_monkeys_blogs_v3   (NEW)
                            the_monkeys_blogs_v2   (kept hot for 1 week, then dropped)
```

All read/write code targets the alias, never the physical index, so
the cut-over is invisible to the rest of the platform.

## Prerequisites

- Elasticsearch 8.x reachable from the machine running the script.
- `mapping_v3.json` deployed (next step).
- Optional: `ES_ADDR`, `ES_USERNAME`, `ES_PASSWORD` environment
  variables. Defaults to `http://localhost:9201` with no auth.

## 1. Create the v3 index

```powershell
$ES = "http://localhost:9201"
$body = Get-Content -Raw documents/reindex/mapping_v3.json
Invoke-RestMethod -Method Put -Uri "$ES/the_monkeys_blogs_v3" `
  -ContentType 'application/json' -Body $body
```

If the index already exists, delete it first only if it is empty:

```powershell
Invoke-RestMethod -Method Delete -Uri "$ES/the_monkeys_blogs_v3"
```

## 2. Reindex with denormalisation

The job uses [`searchdoc.Apply`](../../searchdoc/searchdoc.go)
to populate the flat `title` / `summary` / `body` / `tags` fields from
the legacy editorjs blocks. This is the same function the live write
path runs, so backfilled and live-written docs are byte-identical.

```powershell
$env:ES_ADDR = "http://localhost:9201"
go run ./scripts/reindex_blogs_v3 `
  -src the_monkeys_blogs_v2 `
  -dst the_monkeys_blogs_v3
```

Useful flags:

| Flag             | Default                    | Purpose                                     |
| ---------------- | -------------------------- | ------------------------------------------- |
| `-src`           | `the_monkeys_blogs_v2`     | Source index                                |
| `-dst`           | `the_monkeys_blogs_v3`     | Destination index                           |
| `-dry-run`       | `false`                    | Read + transform, skip bulk writes          |
| `-require-dst`   | `true`                     | Fail if destination index is missing        |

Progress is logged every batch (500 docs). The job is idempotent;
re-running overwrites by `_id` (blog_id).

## 3. Verify

```powershell
$src = Invoke-RestMethod "$ES/the_monkeys_blogs_v2/_count"
$dst = Invoke-RestMethod "$ES/the_monkeys_blogs_v3/_count"
"$($src.count) -> $($dst.count)"
```

Spot-check a few denormalised docs:

```powershell
Invoke-RestMethod "$ES/the_monkeys_blogs_v3/_search?pretty&size=3&_source=blog_id,title,summary,tags"
```

## 4. Atomic alias swap

```powershell
$payload = @{
  actions = @(
    @{ remove = @{ index = '*';                       alias = 'the_monkeys_blogs' } },
    @{ add    = @{ index = 'the_monkeys_blogs_v3';    alias = 'the_monkeys_blogs' } }
  )
} | ConvertTo-Json -Depth 5

Invoke-RestMethod -Method Post -Uri "$ES/_aliases" `
  -ContentType 'application/json' -Body $payload
```

The alias swap is atomic — a single request flips reads/writes from v2
to v3 with no downtime.

## 5. Roll back (if needed)

```powershell
$payload = @{
  actions = @(
    @{ remove = @{ index = '*';                       alias = 'the_monkeys_blogs' } },
    @{ add    = @{ index = 'the_monkeys_blogs_v2';    alias = 'the_monkeys_blogs' } }
  )
} | ConvertTo-Json -Depth 5

Invoke-RestMethod -Method Post -Uri "$ES/_aliases" `
  -ContentType 'application/json' -Body $payload
```

After one week of clean v3 traffic, drop v2:

```powershell
Invoke-RestMethod -Method Delete -Uri "$ES/the_monkeys_blogs_v2"
```
