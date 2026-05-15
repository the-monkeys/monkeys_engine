# Smart File Storage: Microservices Deduplication Architecture

## 1. Executive Summary
Currently, our system stores a completely new file every time a user uploads an image, even if that exact image has been uploaded 100 times before. This wastes disk space and increases storage costs.

**The Solution:** We are implementing **Content-Addressable Storage (CAS)** with a strict microservices architecture. When a file is uploaded, we give it a unique fingerprint (a SHA-256 Hash). If the fingerprint exists globally across the platform, we reuse the existing physical asset. If it's new, we store it.

**Important Design Rule:** A physical asset is not the same thing as a blog's file reference. Multiple blogs may point to the same physical asset. Deleting or updating one blog's file must only remove or replace that blog's reference; it must not delete the shared MinIO object in the request path.

We are adopting a **"Lazy Fix" strategy**: existing files remain untouched. The deduplication logic only applies to *new* uploads going forward.

---

## 2. Microservice Responsibilities

To maintain a scalable and decoupled system, the responsibilities are strictly divided between the API Gateway and the Storage Microservice.

### 2.1 The API Gateway (`the_monkeys_gateway`)
*   **Role:** The stateless orchestrator.
*   **Hashing:** Calculates the SHA-256 hash in real-time as the file streams from the client (`Editor.js`).
*   **gRPC Caller:** Makes an internal gRPC call to the Storage Microservice to check if the hash already exists.
*   **MinIO Client:** Only pushes the actual binary file to MinIO if the Storage Service confirms the hash is entirely new.
*   **Reference Caller:** Tells the Storage Service which blog/profile/draft now references the asset.
*   **Delete/Update Rule:** Never directly deletes a deduplicated MinIO object because another blog may still reference it.
*   **Direct API Owner:** Handles user-initiated upload/update/delete requests in `internal/storage_v2/routes.go` by creating, replacing, or deleting references.
*   *Rule:* The Gateway **never** connects directly to the Postgres database.

### 2.2 The Storage Service (`the_monkeys_storage`)
*   **Role:** The stateful index manager.
*   **Database Owner:** Exclusively owns the `storage_assets` and `storage_asset_refs` PostgreSQL tables.
*   **gRPC Server:** Exposes methods for asset lookup, asset registration, reference registration, reference deletion, and metadata updates.
*   **Data Integrity:** Tracks metadata like image dimensions, BlurHash, and future AI classifications (like NSFW scores).
*   **Event Consumer:** Handles RabbitMQ cleanup events such as `BLOG_DELETE` and `USER_ACCOUNT_DELETE` by deleting references, not shared assets.
*   **Garbage Collection Owner:** Decides when an unreferenced physical object may be removed from MinIO, preferably by a background job with a grace period.

---

## 3. End-to-End Implementation Plan

### Phase 1: Database Migration
We will create a new SQL migration to track asset fingerprints and logical references.
**File:** `schema/000005_add_storage_assets_table.up.sql`
```sql
CREATE TABLE storage_assets (
    checksum      VARCHAR(64) PRIMARY KEY, -- SHA-256 Fingerprint
    object_key    TEXT NOT NULL,          -- Canonical MinIO object path, ideally derived from checksum
    content_type  VARCHAR(100),
    size          BIGINT,
    width         INT,
    height        INT,
    blurhash      TEXT,
    is_nsfw       BOOLEAN DEFAULT FALSE,  -- NSFW flag for AI scanning
    nsfw_score    FLOAT DEFAULT 0,
    created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_storage_assets_checksum ON storage_assets(checksum);

CREATE TABLE storage_asset_refs (
    id            UUID PRIMARY KEY,
    checksum      VARCHAR(64) NOT NULL REFERENCES storage_assets(checksum),
    owner_type    VARCHAR(32) NOT NULL,   -- blog, profile, draft, etc.
    owner_id      TEXT NOT NULL,          -- blog id, user id, draft id, etc.
    purpose       VARCHAR(64) NOT NULL,   -- editor_image, profile_image, attachment, etc.
    file_name     TEXT,
    created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at    TIMESTAMP NULL
);

CREATE INDEX idx_storage_asset_refs_checksum ON storage_asset_refs(checksum);
CREATE INDEX idx_storage_asset_refs_owner ON storage_asset_refs(owner_type, owner_id, deleted_at);
```

The recommended canonical object key is content-based, for example `assets/sha256/ab/cd/<checksum>.<ext>`. It should not be tied to the first blog that uploaded it, because that blog might later delete its reference while other blogs still need the same physical file.

### Phase 2: Protobuf Definition Updates
We will update the Storage Service Protobuf interface.
**File:** `apis/serviceconn/gateway_file_service/pb/gw_file.proto`
1. Add `CheckAssetReq` (contains `checksum`) and `CheckAssetRes` (contains `exists`, `object_key`, metadata).
2. Add `RegisterAssetReq` (contains `checksum`, `object_key`, metadata) and `RegisterAssetRes`.
3. Add `CreateAssetRefReq` (contains `checksum`, `owner_type`, `owner_id`, `purpose`, `file_name`) and `CreateAssetRefRes`.
4. Add `DeleteAssetRefReq` (contains `ref_id` or owner identity + file identity) and `DeleteAssetRefRes`.
5. Add `ReplaceAssetRefReq` for update flows, or implement update as `DeleteAssetRef(old)` + `CreateAssetRef(new)` inside one Storage Service transaction.
6. Run the `protoc` generation script to create the updated Go stubs for both the Gateway and Storage service.

### Phase 3: Storage Service Implementation
**Files:** `microservices/the_monkeys_storage/main.go` and `internal/server/server.go`
1. **DB Setup:** Connect to the `the_monkeys_user_dev` Postgres database on startup using the existing environment variables.
2. **Implement `CheckAsset`:** Run `SELECT * FROM storage_assets WHERE checksum = $1`. Return the metadata if found.
3. **Implement `RegisterAsset`:** Run `INSERT INTO storage_assets ... ON CONFLICT DO NOTHING`, then return the canonical row. If another request inserted the same checksum first, return that existing canonical `object_key`.
4. **Implement `CreateAssetRef`:** Insert or restore a logical reference from the blog/profile/draft to the checksum.
5. **Implement `DeleteAssetRef`:** Soft-delete the reference only. Do not delete the MinIO object in the request path.
6. **Implement Garbage Collection:** A background job may delete physical objects only when no active references exist, and only after a grace period.

### Phase 4: Gateway Logic Update
**File:** `microservices/the_monkeys_gateway/internal/storage_v2/routes.go`
1. **Hash First:** Compute the SHA-256 hash before uploading to MinIO. Small files can be buffered in memory; larger files should be spooled to a temporary file so we can hash first without loading the full body into RAM.
2. **Check:** Call `storageClient.CheckAsset(hash)`.
3. **Fast Reuse:** If the service returns `exists: true`, create a new logical reference for the current blog/profile/draft and return the existing asset URL. Do not upload to MinIO.
4. **Upload & Register:** If not found, upload to MinIO using a checksum-derived object key, then call `storageClient.RegisterAsset(...)`.
5. **Create Reference:** After registration succeeds, call `CreateAssetRef(...)` for the current owner.
6. **Delete Flow:** Delete only the reference associated with the blog/profile/draft. Never call `RemoveObject` for the shared asset from the HTTP delete handler.
7. **Update Flow:** Treat update as "point this logical reference at a new checksum." The old physical asset remains available for other references and is eligible for background garbage collection only if it becomes unreferenced.

### Phase 5: Delete and Update Entry Points
There are multiple places where a user's files can be updated or deleted. Every path must follow the same reference-first rule.

1. **Direct File API:** `microservices/the_monkeys_gateway/internal/storage_v2/routes.go`
   * `UpdatePostFile` must hash/register the new asset and replace the existing blog-file reference.
   * `DeletePostFile` must delete the blog-file reference only.
   * Profile update/delete should follow the same pattern for profile references.
   * These handlers must not call `mc.RemoveObject` for CAS-managed keys.

2. **Blog Delete Event:** `microservices/the_monkeys_storage/internal/consumer/consumer.go` with `case constants.BLOG_DELETE`
   * This path currently deletes `posts/{blogId}/` from filesystem/MinIO.
   * Under CAS, it must soft-delete every active reference where `owner_type = 'blog'` and `owner_id = blogId`.
   * It must not delete checksum-based MinIO assets directly.
   * Legacy `posts/{blogId}/...` objects can still be removed during the lazy migration period if they are not CAS-managed.

3. **Account Delete Event:** `microservices/the_monkeys_storage/internal/consumer/consumer.go` with `case constants.USER_ACCOUNT_DELETE`
   * This path must delete the user's profile reference and all blog references for the account's blog IDs.
   * It must not remove shared checksum-based MinIO assets directly.
   * Legacy profile/blog prefixes can still be cleaned separately until all old files are migrated.

4. **Garbage Collector:** Storage Service background job
   * Finds assets with zero active references.
   * Waits for a grace period to protect against delayed messages, retries, and race conditions.
   * Deletes the physical MinIO object and optionally tombstones the `storage_assets` row.

This means the implementation needs to distinguish CAS-managed object keys from legacy path-based keys. CAS-managed keys should use the checksum path, such as `assets/sha256/ab/cd/<checksum>.<ext>`, while legacy keys like `posts/{blogId}/...` can continue to use prefix deletion until migrated.

---

## 4. Interview & Stakeholder Q&A

**Q1: Why doesn't the Gateway just check the database directly? It would be faster.**  
**A:** Checking the database directly violates the microservices architecture by making the Gateway "stateful" and tightly coupling it to the database schema. By using a gRPC call to the Storage service, we maintain strict domain boundaries. The gRPC call over the internal Docker network takes less than 2 milliseconds, so the performance impact is negligible compared to the architectural benefit.

**Q2: What happens if the gRPC call to the Storage service fails or times out?**  
**A:** We can only degrade safely before shared state is involved. If `CheckAsset` fails, the Gateway may upload the binary as a new candidate, but the request must not be considered successful until the Storage Service registers the asset and creates the reference. If the Storage Service is unavailable for registration or reference creation, the Gateway should return a retryable error or use a clearly marked legacy/non-deduplicated path. It must not return a shared deduplicated URL without a reference row.

**Q3: What if a user deletes their blog post? Does the deduplicated file disappear?**  
**A:** No. The delete operation removes that blog's reference to the asset, not the shared physical object in MinIO. If another blog points to the same checksum, it keeps working because its reference still exists. Physical cleanup is handled later by a Storage Service garbage collector that only deletes assets with zero active references after a grace period.

**Q4: What happens if a user updates an image that was deduplicated with another blog's image?**  
**A:** Updating is treated as replacing a reference, not overwriting or deleting the shared object. The Gateway hashes the new upload, the Storage Service finds or registers the new asset, and then the user's blog reference is moved from the old checksum to the new checksum. Other blogs that referenced the old checksum continue to render the old image.

**Q5: Where can delete/update happen, and how do we keep all of those paths safe?**
**A:** There are three main paths: direct file API calls in `storage_v2/routes.go`, `BLOG_DELETE` messages in the Storage Service consumer, and `USER_ACCOUNT_DELETE` messages in the Storage Service consumer. All three must delete or replace references, not shared checksum-based MinIO objects. Only the garbage collector is allowed to remove CAS-managed physical assets, and only after it proves there are zero active references.

**Q6: Why do we need a separate reference table? Isn't checksum enough?**
**A:** The checksum tells us whether the physical bytes already exist. It does not tell us who is using those bytes. The reference table answers that second question: which blog, profile, draft, or attachment points to which checksum. Without references, a delete from one owner could accidentally remove a file still used by another owner.

**Q7: Should the canonical MinIO key be `posts/{blogId}/{fileName}`?**
**A:** No for deduplicated assets. A key tied to the first uploader creates ownership confusion. If blog A first uploads `cat.png`, blog B later reuses it, and blog A deletes its post, the object key would still look like blog A owns it. The canonical object key should be content-based, such as `assets/sha256/ab/cd/<checksum>.png`. Blog-specific URLs can be logical API routes that resolve through the reference table.

**Q8: What about the current code that deletes `posts/{blogId}/` or `profiles/{username}/` from MinIO?**
**A:** That remains valid only for legacy path-based objects during the lazy migration period. Once an object is CAS-managed and stored under a checksum-based key, prefix deletion must not touch it. The safest implementation is to route CAS cleanup through reference deletion and let a background garbage collector remove unreferenced checksum assets later.

**Q9: Should we track NSFW content here?**
**A:** Yes. We added `is_nsfw` to the schema. If an AI service scans an uploaded file and flags it as NSFW, we save that result. If someone else uploads the *exact same* malicious or inappropriate file later, the system instantly knows it's NSFW based on the hash without needing to run the expensive AI scan again.

**Q10: What happens to files already in someone's "Draft" blog?**
**A:** Nothing breaks. Drafts use the exact same upload API. If a draft already has an image, it stays there. If a user uploads a new image to a draft, it gets deduplicated perfectly.

**Q11: We already have many duplicate files in MinIO. How do we fix them?**
**A:** We are using the **"Lazy Fix"** strategy. We only stop *new* duplicates from being created. This guarantees we don't accidentally break old blog posts. A background migration script can be run later to hash old files, create asset rows and reference rows, and then safely consolidate duplicate physical objects only after verifying every old URL has a valid replacement reference.

**Q12: What happens if reference creation fails after the binary upload succeeds?**
**A:** The user request should fail gracefully and the uploaded object should be treated as unreferenced. The Storage Service garbage collector can remove it later. We should not return a successful blog image URL unless the asset reference was created, because the reference is what makes update/delete semantics safe.

---

## 5. Video Script for Feature Release

**Scene 1: The Problem**
*(Screen shows an architectural diagram with a bottleneck at the storage bucket)*  
"Hi Team! Currently, when users upload identical images, we store multiple copies. This wastes space. But more importantly, our API Gateway was at risk of becoming too tightly coupled if we tried to fix it poorly."

**Scene 2: The Microservices Solution**  
*(Animation showing the Gateway computing a Hash, passing it via gRPC to the Storage Service, and the Storage Service querying Postgres)*  
"Today we are launching **Smart Storage Deduplication**, built correctly for scale. The API Gateway now calculates a unique SHA-256 fingerprint for every upload. It sends a lightning-fast gRPC message to our Storage Microservice to ask, 'Do we already have this?'"

**Scene 3: The Demo**  
*(Screen recording of a user uploading a 5MB image. It takes 3 seconds. Then the user uploads the same image again. It finishes in 0.1 seconds.)*  
"Watch this. The first upload pushes the file to MinIO. The second time I upload it? The Gateway asks the Storage service, gets a 'Yes', and skips the upload entirely. It's instant, and perfectly decoupled."

**Scene 4: Closing**  
"This makes our platform faster for users, cheaper to run, and keeps our microservice architecture pristine. Thanks for watching!"
