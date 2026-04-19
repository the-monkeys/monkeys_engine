# Relative Path Storage — Implementation Plan

## Problem Statement

Image/file URLs stored in Elasticsearch contain absolute domains:
```
https://monkeys.support/api/v2/storage/posts/00japq/image.png
```

This causes:
1. **Local testing broken** — `monkeys.support` is unreachable from localhost
2. **Domain lock-in** — losing or changing `monkeys.support` requires full ES migration
3. **Multi-env mismatch** — staging/dev/prod all need different domains

## Decision: Store Relative Paths

Store only the path portion in ES. Let the browser resolve the domain:
```
/api/v2/storage/posts/00japq/image.png
```

Browser on `localhost:3000` → loads from `localhost:3000/api/v2/storage/posts/...`
Browser on `monkeys.support` → loads from `monkeys.support/api/v2/storage/posts/...`

Zero code, zero migration needed for domain changes.

---

## Current State (from live data)

All existing file URLs in ES follow ONE pattern:
```
https://monkeys.support/api/v2/storage/posts/{blogId}/{fileName}
```
- Domain: `https://monkeys.support` (production)
- No v1 URLs are visible in responses (rewriteV1StorageURLs converts them at runtime)
- No other domains found in the data

---

## Changes Required

### Change 1: `presignedOrCDNURL()` → Return Relative Path

**File:** `microservices/the_monkeys_gateway/internal/storage_v2/routes.go` (line ~253)

**Before:**
```go
if s.cdnURL != "" {
    return strings.TrimRight(s.cdnURL, "/") + "/" + objectName, nil
}
```

**After:**
```go
// Return a relative path — no domain prefix.
// The browser resolves this against the current origin, making it
// environment-agnostic (works on localhost, staging, production).
return "/api/v2/storage/" + objectName, nil
```

**Why this works:** The CDN URL check (`if s.cdnURL != ""`) currently decides between
CDN URL vs presigned URL. With relative paths, we always return the gateway path
regardless of `MINIO_CDN_URL` config. The gateway's own `GetPostFile` handler
streams the object from MinIO.

**What happens:** New uploads return `/api/v2/storage/posts/{id}/{file}` — stored
as-is in ES. Browser resolves against current origin.

### Change 2: `rewriteV1StorageURLs()` → Strip Domain + Fix Path

**File:** `microservices/the_monkeys_gateway/internal/blog/storage_rewriter.go`

Current rewriter replaces path prefixes only:
- `/api/v1/files/post/` → `/api/v2/storage/posts/`
- `/api/v1/files/profile/` → `/api/v2/storage/profiles/`

But existing v2 data ALSO has the domain baked in:
```
https://monkeys.support/api/v2/storage/posts/...
```

Need to additionally strip the production domain so documents return relative paths.

**New approach:** Use a compiled regex to strip ANY domain prefix:
```go
var absStorageURLRe = regexp.MustCompile(`https?://[^"/\s]+(/api/v[12]/(?:storage|files)/)`)
```
This matches `https://ANY-HOST/api/v2/storage/` and replaces with just `$1`
(the captured path group). Works for monkeys.support, dev.monkeys.support,
localhost:8081, or any future domain — no configuration required.

Then v1 path rewriting is applied as before:
- `/api/v1/files/post/` → `/api/v2/storage/posts/`
- `/api/v1/files/profile/` → `/api/v2/storage/profiles/`

No `InitStorageRewriter()` call needed. The regex is compiled at package init time.

### Change 3: ES Migration Script → Target Relative Paths

**File:** `scripts/migrate/migrate_es_storage_urls.go`

Update the Painless script to:
1. Find any URL containing `/api/v1/files/post/` → replace with `/api/v2/storage/posts/{tail}`
2. Find any URL containing `/api/v2/storage/` with a domain prefix → strip to `/api/v2/storage/{tail}`
3. Result is always a relative path

No longer depends on `MINIO_CDN_URL` — the target is always the relative path.

### Change 4: No Frontend Changes

EditorJS stores whatever URL the upload response returns. Since `presignedOrCDNURL()`
now returns `/api/v2/storage/posts/{id}/{file}`, that's what gets stored.

The browser resolves relative paths against the page's origin automatically.
No changes to `editorjs.config.ts`, `getBlogContent.tsx`, or any dialog.

---

## Execution Order

1. **Update `presignedOrCDNURL()`** — new uploads return relative paths
2. **Update `rewriteV1StorageURLs()`** — existing data with absolute domains gets stripped at runtime
3. **Update ES migration script** — one-time fix to convert all stored URLs to relative
4. **Rebuild Docker, verify locally** — upload image, confirm relative URL stored, confirm image loads

## Post-Migration Cleanup

After running the ES migration script in all environments:
- Remove `rewriteV1StorageURLs()` and all call sites in `blog/routes.go`
- `MINIO_CDN_URL` env var can be removed from `.env` (no longer used for URL generation)
- `MINIO_PUBLIC_BASE_URL` stays (used for presigned URL fallback if needed)

## Risk Assessment

| Risk | Mitigation |
|---|---|
| RSS/email needs absolute URLs | Prepend domain at rendering layer, not storage layer |
| EditorJS preview during editing | Relative paths work in browser — resolved against origin |
| Third-party embeds | Not applicable — embed blocks use external URLs (YouTube, etc.) |
| Existing presigned URL fallback | Kept intact — only the CDN code path changes |
