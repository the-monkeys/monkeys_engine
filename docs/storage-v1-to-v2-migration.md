# Storage V1 → V2 Migration Plan

> **Status:** IMPLEMENTED  
> **Author:** Engineering  
> **Date:** 2026-03-06  
> **Impact:** ~50K MAU, zero-downtime required

---

## 1. Problem Statement

All blog image URLs stored in Elasticsearch follow the **v1 pattern**:

```
https://monkeys.support/api/v1/files/post/{blog_id}/{fileName}
```

Example from a real document (`blog_id: 0ybpnn`):

```json
{
  "type": "image",
  "data": {
    "file": {
      "url": "https://monkeys.support/api/v1/files/post/0ybpnn/image.png"
    }
  }
}
```

These URLs are **hardcoded inside `blog.blocks[].data.file.url`** in every published/draft blog document in the `the_monkeys_blogs` Elasticsearch index.

### Current Architecture

| Component | V1 (Current) | V2 (Target) |
|---|---|---|
| **Gateway Route** | `GET /api/v1/files/post/:id/:fileName` | `GET /api/v2/storage/posts/:id/:fileName` |
| **Backend** | gRPC streaming via `the_monkeys_storage` service | Direct MinIO from gateway |
| **Upload Route** | `POST /api/v1/files/post/:id` | `POST /api/v2/storage/posts/:id` |
| **Profile GET** | `GET /api/v1/files/profile/:uid/profile` | `GET /api/v2/storage/profiles/:uid/profile` |
| **Frontend baseURL** | `axiosInstance` with `baseURL: '/api/v1'` | Needs update to `/api/v2` |
| **Frontend image URL construction** | `${API_URL}/files/post/${blogId}/${fileName}` | `${API_URL_V2}/storage/posts/${blogId}/${fileName}` |

### Why We Cannot Simply Switch

1. **~50K MAU** with cached pages, bookmarks, and external links pointing to v1 URLs.
2. **Every existing blog document** in Elasticsearch contains hardcoded v1 URLs in image blocks.
3. **SEO/OpenGraph metadata** (in `layout.tsx`) reads `block.data.file.url` directly for `og:image` tags — search engines and social platforms have indexed these.
4. The v1 backend (gRPC `the_monkeys_storage`) and v2 backend (MinIO) are **separate storage systems** — files must be physically migrated.

---

## 2. Recommended Strategy: 4-Phase Rolling Migration

The approach is **zero-downtime, backward-compatible, and incrementally reversible**.

```
Phase 1: Gateway URL Rewrite Layer (READ path)
Phase 2: Frontend + Upload Migration (WRITE path)
Phase 3: Bulk File Migration (DATA path)
Phase 4: Cleanup & Deprecation
```

---

## 3. Phase 1 — Gateway URL Rewrite Layer (Backend, Zero Frontend Changes)

**Goal:** Make old v1 URLs transparently serve files from MinIO without any client changes.

### 3A. Blog Response Rewriter (Highest Impact, Lowest Risk)

Add a URL rewrite step in `GetPublishedBlogByBlogId` and `GetDraftBlogByBlogIdV2` in the blog gateway handler. After unmarshalling the blog JSON from gRPC, scan and replace v1 URLs before sending the response.

**File:** `microservices/the_monkeys_gateway/internal/blog/routes.go`

```go
// rewriteStorageV1URLs rewrites legacy /api/v1/files/post/ URLs to
// /api/v2/storage/posts/ in blog block data so clients fetch from MinIO.
// Operates on the raw JSON bytes for zero-alloc string replacement.
func rewriteStorageV1URLs(data []byte) []byte {
    // Pattern: /api/v1/files/post/ → /api/v2/storage/posts/
    return bytes.ReplaceAll(data, 
        []byte("/api/v1/files/post/"), 
        []byte("/api/v2/storage/posts/"),
    )
}
```

Insert the rewrite **before** `json.Unmarshal` in each blog-fetch handler:

```go
// In GetPublishedBlogByBlogId, after receiving resp from gRPC:
rewritten := rewriteStorageV1URLs(resp.Value)

var blogMap map[string]interface{}
if err := json.Unmarshal(rewritten, &blogMap); err != nil {
    // ...
}
```

**Why `bytes.ReplaceAll` and not regex?**
- The pattern is deterministic (`/api/v1/files/post/` → `/api/v2/storage/posts/`).
- `bytes.ReplaceAll` is zero-alloc when no match is found (common case for new blogs).
- Regex adds ~10x overhead per call and requires compilation. Unacceptable at 50K MAU.

**Risk:** Near-zero. The replacement is purely additive — it only transforms the URL path prefix. The `{blog_id}/{fileName}` segments are identical in both v1 and v2 routes.

### 3B. V1 Route Fallback to MinIO

Modify the v1 `GetBlogFile` handler to **try MinIO first**, then fall back to the gRPC storage service. This covers:
- Direct browser navigation to v1 URLs
- Cached HTML pages with v1 URLs
- External links, RSS feeds, social media crawlers

**File:** `microservices/the_monkeys_gateway/internal/storage/routes.go`

```go
func (asc *FileServiceClient) GetBlogFile(ctx *gin.Context) {
    blogId := ctx.Param("id")
    fileName := ctx.Param("fileName")

    // --- NEW: Try MinIO (v2 backend) first ---
    if asc.minioFallback != nil {
        objectName := "posts/" + blogId + "/" + fileName
        obj, err := asc.minioFallback.GetObject(
            ctx.Request.Context(), asc.minioBucket, objectName, minio.GetObjectOptions{},
        )
        if err == nil {
            defer obj.Close()
            stat, statErr := obj.Stat()
            if statErr == nil {
                // File exists in MinIO — serve directly
                if stat.ContentType != "" {
                    ctx.Header("Content-Type", stat.ContentType)
                }
                ctx.Header("Cache-Control", "public, max-age=31536000")
                io.Copy(ctx.Writer, obj)
                return
            }
        }
    }
    // --- END NEW ---

    // Original gRPC fallback (existing code, unchanged)
    stream, err := asc.Client.GetBlogFile(context.Background(), &pb.GetBlogFileReq{
        BlogId:   blogId,
        FileName: fileName,
    })
    // ... rest of existing handler
}
```

Inject the MinIO client into `FileServiceClient` during registration:

```go
type FileServiceClient struct {
    Client        pb.UploadBlogFileClient
    log           *zap.SugaredLogger
    minioFallback *minio.Client  // NEW: optional MinIO fallback
    minioBucket   string         // NEW
}
```

### 3C. Profile URL Rewriting

Apply the same pattern for profile images:

```
/api/v1/files/profile/ → /api/v2/storage/profiles/
```

This affects `useProfileImage.ts` → `GET /api/v1/files/profile/{username}/profile`.

---

## 4. Phase 2 — Frontend + Upload Migration (WRITE Path)

**Goal:** New uploads go directly to v2. New blog documents get v2 URLs stored in Elasticsearch.

### 4A. Create v2 Axios Instance

**File:** `local/the_monkeys/apps/the_monkeys/src/services/api/axiosInstanceV2.ts`

```typescript
import axios from 'axios';

const axiosInstanceV2 = axios.create({
  baseURL: '/api/v2',
  timeout: 30000,
});

// Attach the same auth interceptor as axiosInstance
axiosInstanceV2.interceptors.request.use(/* same auth logic */);

export default axiosInstanceV2;
```

### 4B. Update EditorJS Image Config

**File:** `local/the_monkeys/apps/the_monkeys/src/config/editor/editorjs.config.ts`

Change the upload handler from:

```typescript
// OLD (v1)
const response = await axiosInstance.post(
  `/files/post/${blogId}`,
  formData
);
return {
  success: 1,
  file: {
    url: `${API_URL}/files/post/${blogId}/${response.data.new_file_name}`,
  },
};
```

To:

```typescript
// NEW (v2)
const response = await axiosInstanceV2.post(
  `/storage/posts/${blogId}`,
  formData
);
return {
  success: 1,
  file: {
    url: `${API_URL_V2}/storage/posts/${blogId}/${response.data.fileName}`,
  },
};
```

> **Note:** V2 returns `fileName` (not `new_file_name`). The response schema differs:
> - V1: `{ new_file_name: "image.png" }`
> - V2: `{ fileName: "uuid.png", object: "posts/blogId/uuid.png", url: "...", ... }`
>
> V2 already returns a full `url` field, so prefer using `response.data.url` directly.

### 4C. Update Profile Upload

Switch `useProfileImage` and profile upload components to use `/api/v2/storage/profiles/`.

### 4D. Rollout Strategy

Deploy the frontend update behind a **feature flag** or as a gradual rollout:

```typescript
const STORAGE_VERSION = process.env.NEXT_PUBLIC_STORAGE_VERSION || 'v1';

// In editor config:
if (STORAGE_VERSION === 'v2') {
  // use v2 upload
} else {
  // use v1 upload (existing)
}
```

This allows instant rollback by flipping an env var.

---

## 5. Phase 3 — Bulk File Migration (DATA Path)

**Goal:** Copy all existing files from the gRPC storage backend to MinIO so v2 URLs work for old content.

### 5A. Migration Script

The old storage service stores files on disk (typically under a volume mount). Create a migration script that:

1. Lists all blog directories in the old storage.
2. For each file, uploads to MinIO under `posts/{blog_id}/{fileName}`.
3. Logs success/failure per file for audit.

```go
// scripts/migrate_storage_v1_to_v2.go
package main

import (
    "context"
    "log"
    "os"
    "path/filepath"

    "github.com/minio/minio-go/v7"
    "github.com/minio/minio-go/v7/pkg/credentials"
)

func main() {
    sourceDir := os.Getenv("V1_STORAGE_DIR")       // e.g., /data/blogs/
    endpoint  := os.Getenv("MINIO_ENDPOINT")
    accessKey := os.Getenv("MINIO_ACCESS_KEY")
    secretKey := os.Getenv("MINIO_SECRET_KEY")
    bucket    := os.Getenv("MINIO_BUCKET_NAME")

    mc, err := minio.New(endpoint, &minio.Options{
        Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
        Secure: false,
    })
    if err != nil {
        log.Fatalf("minio client: %v", err)
    }

    ctx := context.Background()
    migrated, skipped, failed := 0, 0, 0

    err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
        if err != nil || info.IsDir() {
            return err
        }

        rel, _ := filepath.Rel(sourceDir, path)
        objectName := "posts/" + filepath.ToSlash(rel) // e.g., posts/0ybpnn/image.png

        // Check if already exists in MinIO (idempotent)
        if _, statErr := mc.StatObject(ctx, bucket, objectName, minio.StatObjectOptions{}); statErr == nil {
            skipped++
            return nil
        }

        f, err := os.Open(path)
        if err != nil {
            log.Printf("FAIL open %s: %v", path, err)
            failed++
            return nil
        }
        defer f.Close()

        _, err = mc.PutObject(ctx, bucket, objectName, f, info.Size(), minio.PutObjectOptions{
            CacheControl: "public, max-age=31536000",
        })
        if err != nil {
            log.Printf("FAIL upload %s: %v", objectName, err)
            failed++
            return nil
        }

        migrated++
        log.Printf("OK %s (%d bytes)", objectName, info.Size())
        return nil
    })

    if err != nil {
        log.Fatalf("walk error: %v", err)
    }

    log.Printf("Migration complete: %d migrated, %d skipped, %d failed", migrated, skipped, failed)
}
```

### 5B. Profile Image Migration

Same pattern for profile images:
- Source: `profile/{user_id}/profile.png` (old storage)
- Destination: `profiles/{user_id}/profile` (MinIO)

### 5C. Validation

After migration, run a verification script that:
1. Queries all documents from `the_monkeys_blogs` ES index.
2. Extracts all v1 image URLs.
3. For each URL, verifies the corresponding MinIO object exists (`StatObject`).
4. Reports any missing files.

---

## 6. Phase 4 — Cleanup & Deprecation

**Timeline:** 3–6 months after Phase 3, once metrics confirm zero v1 traffic.

### 6A. Optional: Bulk ES Document Update

Once all files are confirmed in MinIO, optionally rewrite URLs in Elasticsearch so the gateway rewriter (Phase 1) becomes a no-op:

```bash
# ES update_by_query to replace v1 URLs with v2 in all blog documents
POST /the_monkeys_blogs/_update_by_query
{
  "script": {
    "source": "ctx._source = ctx._source.toString().replace('/api/v1/files/post/', '/api/v2/storage/posts/')",
    "lang": "painless"
  },
  "query": { "match_all": {} }
}
```

> **Warning:** This is a destructive operation on production data. Take an ES snapshot first. Test on a staging index. Run in small batches using `slice` or `scroll`.

### 6B. Remove v1 Routes

Once analytics confirm zero traffic on v1 endpoints:

1. Remove `storage.RegisterFileStorageRouter(...)` from `microservices/the_monkeys_gateway/main.go`.
2. Delete the `microservices/the_monkeys_gateway/internal/storage/` package.
3. Remove the `rewriteStorageV1URLs()` call from blog handlers.
4. Decommission the `the_monkeys_storage` gRPC service.

### 6C. Remove Feature Flags

Remove `NEXT_PUBLIC_STORAGE_VERSION` and any v1 code paths from the frontend.

---

## 7. URL Mapping Reference

| Resource | V1 Path | V2 Path | MinIO Object Key |
|---|---|---|---|
| Blog image (GET) | `/api/v1/files/post/{id}/{file}` | `/api/v2/storage/posts/{id}/{file}` | `posts/{id}/{file}` |
| Blog image (POST) | `/api/v1/files/post/{id}` | `/api/v2/storage/posts/{id}` | `posts/{id}/{uuid}.{ext}` |
| Blog image (DELETE) | `/api/v1/files/post/{id}/{file}` | `/api/v2/storage/posts/{id}/{file}` | `posts/{id}/{file}` |
| Profile image (GET) | `/api/v1/files/profile/{uid}/profile` | `/api/v2/storage/profiles/{uid}/profile` | `profiles/{uid}/profile` |
| Profile image (POST) | `/api/v1/files/profile/{uid}/profile` | `/api/v2/storage/profiles/{uid}/profile` | `profiles/{uid}/profile` |
| Profile image (DELETE) | `/api/v1/files/profile/{uid}/profile` | `/api/v2/storage/profiles/{uid}/profile` | `profiles/{uid}/profile` |
| Profile image (stream) | `/api/v1.1/files/profile/{uid}/profile` | `/api/v2/storage/profiles/{uid}/profile` | `profiles/{uid}/profile` |

---

## 8. Risk Assessment

| Risk | Probability | Impact | Mitigation |
|---|---|---|---|
| MinIO file missing for old blog | Medium | User sees broken image | Phase 1B: v1 handler falls back to gRPC if MinIO miss |
| Frontend caching old v1 URLs | Low | Stale URL, still works | v1 routes stay alive throughout migration |
| ES rewrite corrupts data | Low | Blog content lost | Take ES snapshot before; rewrite in small batches |
| v2 upload returns different schema | Medium | Frontend upload fails | Map `response.data.url` directly; feature-flag rollout |
| CDN/proxy caches v1 URLs | Low | Old URL cached, still works | v1 routes stay alive; set short `Cache-Control` on v1 responses |
| SEO impact from URL change | Low | Crawlers index old URLs | og:image URLs rewritten at gateway level (Phase 1A) |

---

## 9. Implementation Order & Timeline

| Phase | Scope | Estimated Effort | Dependencies |
|---|---|---|---|
| **1A** | Gateway blog response rewriter | 1 day | None |
| **1B** | V1 GET handler MinIO fallback | 1 day | MinIO client accessible from storage pkg |
| **2A–B** | Frontend v2 axios + editor config | 1 day | Phase 1A deployed |
| **2D** | Feature flag rollout | 0.5 day | Phase 2A–B |
| **3A** | Bulk file migration script | 2 days | Access to v1 storage volume |
| **3C** | Validation script | 0.5 day | Phase 3A |
| **4A** | ES document URL update (optional) | 1 day | Phase 3C passes |
| **4B–C** | V1 deprecation & cleanup | 0.5 day | 3–6 months after Phase 3 |

**Total active engineering time:** ~7 days  
**Total calendar time:** 3–6 months (due to deprecation soak period)

---

## 10. Monitoring & Rollback

### Metrics to Track

1. **V1 GET request count** — should trend toward zero after Phase 2 deployment.
2. **V2 GET request count** — should absorb all new traffic.
3. **MinIO fallback miss rate** in v1 handler — indicates files not yet migrated.
4. **4xx/5xx rates** on both v1 and v2 storage endpoints.
5. **Image load errors** in frontend (browser console, Sentry).

### Rollback Procedures

| Phase | Rollback |
|---|---|
| 1A (rewriter) | Remove `rewriteStorageV1URLs` call — old URLs pass through unchanged |
| 1B (MinIO fallback) | Set `minioFallback = nil` — v1 handler uses gRPC only |
| 2 (frontend v2) | Set `NEXT_PUBLIC_STORAGE_VERSION=v1` — frontend reverts to v1 uploads |
| 3 (file migration) | No rollback needed — files in MinIO don't affect v1 |
| 4A (ES rewrite) | Restore from ES snapshot |

---

## 11. Key Architectural Decisions

### Why NOT bulk-rewrite ES documents as the primary strategy?

1. **Irreversible** — if the rewrite has a bug, production data is corrupted.
2. **Does not fix cached pages** — browsers, CDN, Google cache still have v1 URLs.
3. **Does not fix external links** — any URL shared on social media, email, or other sites is a v1 URL.
4. **Requires file migration first** — v2 URLs only work if files exist in MinIO.

The gateway rewriter + v1 fallback approach handles ALL of these cases with zero data mutation.

### Why NOT use an HTTP 301 redirect from v1 → v2?

1. **Image tags don't follow redirects gracefully** — `<img>` follows them, but it adds a round-trip per image on every page load.
2. **OpenGraph crawlers** may not follow redirects.
3. **301 is cacheable** — if the redirect is cached and we need to rollback, clients are stuck.

The transparent proxy/fallback approach avoids all these issues.

### Why rewrite at the gateway and not in the blog service?

1. **Single responsibility** — the blog service should not know about storage URL formats.
2. **The gateway already orchestrates** — it fetches from gRPC and enriches the response (like/bookmark counts).
3. **The rewrite is a transport concern**, not a domain concern.

---

## 12. `the_monkeys_storage` Periodic Sync Analysis

### What the Sync Does Today

File: `microservices/the_monkeys_storage/internal/consumer/consumer.go`

The storage service runs **two bidirectional sync loops** as goroutines:

| Sync | Direction | Frequency | Function |
|---|---|---|---|
| `startPeriodicSync` | Filesystem → MinIO | Every 3 hours (first run after 1 min) | `syncFilesystemToMinio()` |
| `startMinioToFileSystemSync` | MinIO → Filesystem | Every 4 hours (first run after 2 min) | `syncMinioToFileSystem()` |

**Filesystem → MinIO (`syncFilesystemToMinio`):**
- Walks `blogs/` directory → uploads to MinIO under `posts/{blog_id}/{file}` (skip if exists).
- Walks `profile/` directory → uploads to MinIO under `profiles/{username}/{file}` (skip if exists).
- This is the **migration bridge** that copies v1 files into MinIO so v2 routes can serve them.

**MinIO → Filesystem (`syncMinioToFileSystem`):**
- Lists MinIO objects under `posts/` → downloads to `local_blogs/{blog_id}/{file}`.
- Lists MinIO objects under `profiles/` → downloads to `local_profiles/{username}/{file}`.
- Compares by size + modification time; skips if local file is current.
- This ensures the **local filesystem has a copy of v2-uploaded files** so the gRPC `GetBlogFile` handler can still serve them.

### Why Both Directions Exist

```
┌──────────────┐   v1 upload (gRPC)   ┌──────────────┐   sync every 3h   ┌─────────┐
│   Frontend   │ ──────────────────► │  Filesystem   │ ──────────────► │  MinIO  │
│  (EditorJS)  │                      │ blogs/{id}/   │                  │ posts/  │
└──────────────┘                      └──────────────┘                  └─────────┘
                                             ▲                               │
                                             │         sync every 4h         │
                                             └───────────────────────────────┘
```

The v1 gRPC service (`GetBlogFile`) reads from the **filesystem**. The v2 gateway handler (`GetPostFile`) reads from **MinIO**. The sync loops keep both in agreement.

### Recommendation: Phase-Dependent Deactivation

#### During Migration (Phase 1–3): **KEEP BOTH SYNCS ACTIVE**

The syncs are essential during migration:

- **FS → MinIO sync** ensures files uploaded via v1 (if any legacy upload paths remain) get copied to MinIO so v2 reads work.
- **MinIO → FS sync** ensures files uploaded via v2 get copied to the filesystem so v1 reads (gRPC) still work for any remaining v1 URL hits.

Without both syncs running, there will be a window where:
- Old blogs with v1 URLs fail if MinIO doesn't have the file yet.
- New uploads via v2 fail on v1 routes if the filesystem doesn't have the file yet.

#### After Phase 3 (All Files Migrated, v2 Fully Active): **KEEP ONLY FS → MinIO**

Once all new uploads go through v2 and the bulk migration (Phase 3) is validated:
- **Disable `startMinioToFileSystemSync`** — the filesystem is no longer the source of truth.
- **Keep `startPeriodicSync` (FS → MinIO)** as a safety net for any edge case where a file ends up on the filesystem but not in MinIO (e.g., manual operations, recovery scenarios).

#### After Phase 4 (v1 Fully Decommissioned): **DISABLE ALL SYNCS**

Once the v1 gRPC storage service is decommissioned:
- **Remove both sync goroutines entirely.**
- The filesystem mounts (`./profile`, `./blogs`, `./local_profiles`, `./local_blogs`) in `docker-compose.yml` can be unmounted.
- The `the_monkeys_storage` container itself can be stopped and eventually removed.

### Deactivation Implementation

Add config flags to control sync behavior without code changes:

```go
// In consumer.go, modify ConsumeFromQueue:
if mc != nil && cfg != nil {
    if cfg.Storage.EnableFSToMinioSync {   // NEW: env STORAGE_ENABLE_FS_TO_MINIO_SYNC
        go startPeriodicSync(cfg, mc, log)
    }
    if cfg.Storage.EnableMinioToFSSync {   // NEW: env STORAGE_ENABLE_MINIO_TO_FS_SYNC
        go startMinioToFileSystemSync(cfg, mc, log)
    }
}
```

Default both to `true`. Phase 3 completion → set `STORAGE_ENABLE_MINIO_TO_FS_SYNC=false`. Phase 4 completion → set both to `false`, then remove the code.

### What MUST Stay Even After Sync Removal

The **RabbitMQ consumer handlers** in `consumer.go` for user/blog lifecycle events should be retained and adapted:

| Action | Current Behavior | Post-Migration Behavior |
|---|---|---|
| `USER_REGISTER` | Creates filesystem folder + default profile.png | Upload default profile to MinIO only |
| `USERNAME_UPDATE` | Renames FS folder **and** MinIO prefix | Rename MinIO prefix only |
| `USER_ACCOUNT_DELETE` | Deletes FS folder **and** MinIO prefix | Delete MinIO prefix only |
| `BLOG_DELETE` | Deletes FS folder **and** MinIO prefix | Delete MinIO prefix only |

The MinIO operations (`UpdateMinioProfileFolder`, `DeleteMinioProfileFolder`, `DeleteMinioBlogFolder`) are already implemented and correct. Post-migration, strip the filesystem operations and keep only the MinIO ones.

---

## 13. Future-Proofing: Preventing V3 Migration Pain

The v1→v2 migration is painful because of one fundamental design flaw: **absolute storage URLs are embedded in application data (Elasticsearch documents)**. If we repeat this pattern with v2, a future v3 migration will require the exact same multi-phase effort.

### The Root Cause

```json
// PROBLEM: Application data contains infrastructure URLs
{
  "type": "image",
  "data": {
    "file": {
      "url": "https://monkeys.support/api/v1/files/post/0ybpnn/image.png"
    }
  }
}
```

The URL encodes:
1. **Protocol + domain** (`https://monkeys.support`) — ties data to a specific deployment.
2. **API version** (`/api/v1`) — ties data to a specific backend version.
3. **Route structure** (`/files/post/`) — ties data to a specific route layout.

Change any of these three things and every stored document breaks.

### The Solution: Content-Addressable References

Store **logical references** in Elasticsearch, resolve to **physical URLs** at read time.

#### Option A: Relative Object Keys (Recommended)

Store only the MinIO object key in blog data:

```json
// FUTURE: Application data contains only a logical reference
{
  "type": "image",
  "data": {
    "file": {
      "ref": "posts/0ybpnn/image.png"
    }
  }
}
```

The gateway resolves `ref` to a full URL at response time, using whatever storage backend is current:

```go
// In GetPublishedBlogByBlogId, after unmarshal:
resolveFileRefs(blogMap, cfg.Storage.PublicBaseURL)
// e.g., "posts/0ybpnn/image.png" → "https://monkeys.support/api/v2/storage/posts/0ybpnn/image.png"
```

A V3 migration becomes a one-line config change:

```bash
# V2
STORAGE_PUBLIC_BASE_URL=https://monkeys.support/api/v2/storage
# V3 (just change this)
STORAGE_PUBLIC_BASE_URL=https://cdn.monkeys.support
```

No ES document rewrites. No sync loops. No multi-phase migration.

#### Option B: Indirection via `/api/storage/` (Version-Free Route)

Register a version-free route that **always** points to the current storage backend:

```go
// Register a permanent, version-free alias
router.GET("/api/storage/posts/:id/:fileName", svc.GetPostFile)   // same handler as v2
router.GET("/api/storage/profiles/:uid/profile", svc.GetProfileImage)
```

Store this version-free URL in blog data:

```json
{
  "file": {
    "url": "https://monkeys.support/api/storage/posts/0ybpnn/image.png"
  }
}
```

Switching backends becomes an internal handler change — the URL never changes.

#### Option C: CDN-First with Object Keys

If `MINIO_CDN_URL` is set (which it already is in dev.env), the v2 service already generates CDN URLs via `presignedOrCDNURL()`. The CDN URL format is:

```
{MINIO_CDN_URL}/{objectKey}
```

Store the CDN URL in blog data and configure the CDN to route to whatever backend is current. The storage layer becomes invisible to the application.

### Recommended Implementation Plan for V3-Proofing

**This should be done AS PART of the v2 migration (Phase 2), not later.**

1. **Frontend change (Phase 2):** When EditorJS receives the upload response, store `response.data.object` (the MinIO key, e.g., `posts/0ybpnn/uuid.png`) instead of a full URL.

    ```typescript
    // editorjs.config.ts — after v2 upload
    return {
      success: 1,
      file: {
        ref: response.data.object,  // "posts/{blogId}/{uuid}.ext"
      },
    };
    ```

2. **Gateway change:** In `GetPublishedBlogByBlogId`, after the v1→v2 URL rewrite (Phase 1A), add a **ref resolver** that expands `ref` fields into full URLs:

    ```go
    func resolveFileRefs(blogJSON []byte, baseURL string) []byte {
        // Find "ref":"posts/..." and replace with "url":"{baseURL}/posts/..."
        // This handles new-format documents with ref fields
    }
    ```

3. **Backward compatibility:** The resolver handles both formats:
   - Old docs: have `url` field with full URL → pass through (rewriter handles v1→v2).
   - New docs: have `ref` field with object key → resolve to current base URL.

4. **Config:**
    ```bash
    STORAGE_PUBLIC_BASE_URL=https://monkeys.support/api/v2/storage
    ```

### Migration Effort Comparison

| Scenario | Without V3-Proofing | With V3-Proofing |
|---|---|---|
| V2 → V3 migration | 7+ engineering days, 3-6 month soak | 1 config change, instant |
| V3 → V4 migration | Same pain again | 1 config change, instant |
| Domain change (monkeys.support → monkeys.io) | Full ES rewrite | 1 config change |
| CDN provider switch | Full ES rewrite | 1 config change |

### Summary: The Three Things to Do Differently This Time

| # | Action | Where | Why |
|---|---|---|---|
| 1 | Store object keys (`ref`), not full URLs | Frontend EditorJS config | Decouples data from infrastructure |
| 2 | Resolve refs at read time in the gateway | `GetPublishedBlogByBlogId` handler | Single point of URL construction |
| 3 | Add a version-free route alias | Gateway route registration | Permanent URLs that survive backend changes |

This eliminates the entire class of "hardcoded URL in stored data" problems. Future backend changes (v3, CDN migration, domain change) become configuration-only operations with zero data migration.
