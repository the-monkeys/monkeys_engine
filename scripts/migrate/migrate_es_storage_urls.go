// migrate_es_storage_urls.go
//
// One-time migration script to convert ALL storage URLs in Elasticsearch
// blog documents to domain-free relative paths.
//
// WHAT IT DOES:
//   Scans every document in the `the_monkeys_blogs` index and rewrites
//   file-backed block URLs (image, attaches, video, audio, etc.) to
//   relative paths:
//
//     Absolute v2:  "https://monkeys.support/api/v2/storage/posts/{id}/{file}"
//     After:        "/api/v2/storage/posts/{id}/{file}"
//
//     Legacy v1:    "https://monkeys.support/api/v1/files/post/{id}/{file}"
//     After:        "/api/v2/storage/posts/{id}/{file}"
//
//     Already relative: "/api/v2/storage/posts/{id}/{file}"
//     After:            (unchanged — idempotent)
//
//   Profile URLs are rewritten similarly.
//
// WHY:
//   Relative paths make stored data environment-agnostic. The same ES data
//   works on localhost, staging, and production. Domain changes require zero
//   migration.
//
//   After this script runs, the runtime `rewriteV1StorageURLs()` function in
//   blog/storage_rewriter.go and all call sites in blog/routes.go can be
//   deleted. No more per-request byte scanning.
//
// SAFETY:
//   - Dry-run by default (set DRY_RUN=false to apply changes)
//   - Idempotent — safe to run multiple times
//   - Logs every document it would modify with before/after URLs
//   - Uses _update_by_query with painless script for atomic, in-place updates
//   - Does NOT reindex — no downtime, no data loss
//
// USAGE:
//   # Dry run (preview changes without writing):
//   OPENSEARCH_ADDRESS=http://localhost:9200 \
//   OPENSEARCH_OS_USERNAME=admin \
//   OPENSEARCH_OS_PASSWORD=admin \
//   DRY_RUN=true \
//   go run scripts/migrate/migrate_es_storage_urls.go
//
//   # Apply changes:
//   DRY_RUN=false go run scripts/migrate/migrate_es_storage_urls.go
//
// ROLLBACK:
//   Take an ES snapshot before running this. There is no automatic rollback
//   since we are stripping domains (information loss by design).

package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
)

const (
	indexName = "the_monkeys_blogs"

	// v1 path segments to find (domain-agnostic).
	v1PostPath    = "/api/v1/files/post/"
	v1ProfilePath = "/api/v1/files/profile/"

	// v2 relative path targets.
	v2PostPath    = "/api/v2/storage/posts/"
	v2ProfilePath = "/api/v2/storage/profiles/"
	v2StoragePath = "/api/v2/storage/"
)

func main() {
	// ── Config from environment ──────────────────────────────────────
	esAddress := os.Getenv("OPENSEARCH_ADDRESS")
	esUser := os.Getenv("OPENSEARCH_OS_USERNAME")
	esPass := os.Getenv("OPENSEARCH_OS_PASSWORD")
	dryRun := strings.ToLower(os.Getenv("DRY_RUN")) != "false"

	if esAddress == "" {
		log.Fatal("Required env var: OPENSEARCH_ADDRESS")
	}

	log.Printf("=== ES Storage URL Migration (to relative paths) ===")
	log.Printf("Index:       %s", indexName)
	log.Printf("ES Address:  %s", esAddress)
	log.Printf("Dry run:     %v", dryRun)
	log.Println()

	// ── Elasticsearch client ─────────────────────────────────────────
	es, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{esAddress},
		Username:  esUser,
		Password:  esPass,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Self-signed certs in dev
		},
	})
	if err != nil {
		log.Fatalf("Failed to create ES client: %v", err)
	}

	ctx := context.Background()

	// ── Phase 1: Dry-run scan with scroll API ────────────────────────
	// Scroll through ALL documents that have file URLs containing either
	// v1 paths or absolute URLs (http:// or https://) and log what changes.
	scrollSize := 500
	totalDocs := 0
	docsNeedingUpdate := 0
	urlsFound := 0

	// Match documents with:
	// 1. v1 paths (/api/v1/files/post/ or /api/v1/files/profile/)
	// 2. Absolute URLs (http:// or https://) in data.file.url
	// Already-relative v2 paths won't match → idempotent.
	searchBody := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"should": []map[string]interface{}{
					{"wildcard": map[string]interface{}{
						"blog.blocks.data.file.url": map[string]interface{}{
							"value": "*" + v1PostPath + "*",
						},
					}},
					{"wildcard": map[string]interface{}{
						"blog.blocks.data.file.url": map[string]interface{}{
							"value": "*" + v1ProfilePath + "*",
						},
					}},
					{"wildcard": map[string]interface{}{
						"blog.blocks.data.file.url": map[string]interface{}{
							"value": "https://*" + v2StoragePath + "*",
						},
					}},
					{"wildcard": map[string]interface{}{
						"blog.blocks.data.file.url": map[string]interface{}{
							"value": "http://*" + v2StoragePath + "*",
						},
					}},
				},
				"minimum_should_match": 1,
			},
		},
		"size":    scrollSize,
		"_source": []string{"blog_id", "blog.blocks"},
	}

	bodyBytes, _ := json.Marshal(searchBody)
	res, err := es.Search(
		es.Search.WithContext(ctx),
		es.Search.WithIndex(indexName),
		es.Search.WithBody(bytes.NewReader(bodyBytes)),
		es.Search.WithScroll(2*time.Minute),
		es.Search.WithSize(scrollSize),
	)
	if err != nil {
		log.Fatalf("Search failed: %v", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		log.Fatalf("Search error: %s", body)
	}

	type esHit struct {
		ID     string                 `json:"_id"`
		Source map[string]interface{} `json:"_source"`
	}
	type esResult struct {
		ScrollID string `json:"_scroll_id"`
		Hits     struct {
			Total struct {
				Value int `json:"value"`
			} `json:"total"`
			Hits []esHit `json:"hits"`
		} `json:"hits"`
	}

	var result esResult
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		log.Fatalf("Decode failed: %v", err)
	}

	log.Printf("Total documents matching migration patterns: %d", result.Hits.Total.Value)

	// toRelativePath converts an absolute or v1 URL to a relative v2 path.
	// Returns the relative path and true if conversion happened, or empty and false.
	toRelativePath := func(urlStr string) (string, bool) {
		// Case 1: v1 URL — extract tail after v1 path prefix, prepend v2 path
		if idx := strings.Index(urlStr, v1PostPath); idx >= 0 {
			return v2PostPath + urlStr[idx+len(v1PostPath):], true
		}
		if idx := strings.Index(urlStr, v1ProfilePath); idx >= 0 {
			return v2ProfilePath + urlStr[idx+len(v1ProfilePath):], true
		}
		// Case 2: Absolute v2 URL — strip domain, keep path
		if idx := strings.Index(urlStr, v2StoragePath); idx >= 0 && (strings.HasPrefix(urlStr, "http://") || strings.HasPrefix(urlStr, "https://")) {
			return urlStr[idx:], true
		}
		return "", false
	}

	// Process function for each batch of hits
	processHits := func(hits []esHit) {
		for _, hit := range hits {
			totalDocs++
			blogID := hit.ID

			// Walk through blocks to find v1 URLs
			blogData, ok := hit.Source["blog"].(map[string]interface{})
			if !ok {
				continue
			}
			blocks, ok := blogData["blocks"].([]interface{})
			if !ok {
				continue
			}

			docHasV1 := false
			for _, b := range blocks {
				block, ok := b.(map[string]interface{})
				if !ok {
					continue
				}
				// Check ANY block type that has data.file.url — not just "image".
				// EditorJS convention: image, attaches, video, audio all store
				// uploaded file URLs under data.file.url. Embed blocks use
				// data.url (external URLs) which won't match our patterns.
				blockType, _ := block["type"].(string)
				data, ok := block["data"].(map[string]interface{})
				if !ok {
					continue
				}
				fileMap, ok := data["file"].(map[string]interface{})
				if !ok {
					continue
				}
				urlStr, ok := fileMap["url"].(string)
				if !ok || urlStr == "" {
					continue
				}

				if newURL, changed := toRelativePath(urlStr); changed {
					docHasV1 = true
					urlsFound++
					log.Printf("  [%s] (%s) %s → %s", blogID, blockType, urlStr, newURL)
				}
			}

			if docHasV1 {
				docsNeedingUpdate++
			}
		}
	}

	processHits(result.Hits.Hits)

	// Scroll through remaining results
	scrollID := result.ScrollID
	for {
		scrollBody := fmt.Sprintf(`{"scroll":"2m","scroll_id":"%s"}`, scrollID)
		scrollRes, err := es.Scroll(
			es.Scroll.WithContext(ctx),
			es.Scroll.WithBody(strings.NewReader(scrollBody)),
		)
		if err != nil {
			log.Fatalf("Scroll failed: %v", err)
		}

		var scrollResult esResult
		if err := json.NewDecoder(scrollRes.Body).Decode(&scrollResult); err != nil {
			scrollRes.Body.Close()
			log.Fatalf("Scroll decode failed: %v", err)
		}
		scrollRes.Body.Close()

		if len(scrollResult.Hits.Hits) == 0 {
			break
		}

		scrollID = scrollResult.ScrollID
		processHits(scrollResult.Hits.Hits)
	}

	// Clear scroll
	es.ClearScroll(es.ClearScroll.WithScrollID(scrollID))

	log.Println()
	log.Printf("=== Scan Complete ===")
	log.Printf("Documents scanned:              %d", totalDocs)
	log.Printf("Documents needing migration:    %d", docsNeedingUpdate)
	log.Printf("Total URLs to rewrite:          %d", urlsFound)

	if docsNeedingUpdate == 0 {
		log.Println("No documents need migration. Exiting.")
		return
	}

	if dryRun {
		log.Println()
		log.Println("DRY RUN — no changes written. Set DRY_RUN=false to apply.")
		return
	}

	// ── Phase 2: Apply changes with _update_by_query ─────────────────
	//
	// Uses a Painless script that iterates all blocks in each matching
	// document and converts file URLs to domain-free relative paths.
	// Handles any block type with data.file.url (image, attaches, video, etc.).
	// This is atomic per-document and does not require reindexing.
	log.Println()
	log.Println("=== Applying Migration ===")

	// Painless script: for each block with data.file.url:
	//   1. If URL contains v1 post path → extract tail, prepend v2 post path
	//   2. Else if URL contains v1 profile path → extract tail, prepend v2 profile path
	//   3. Else if URL starts with http(s) and contains v2 storage path → strip domain
	//   4. Otherwise → leave unchanged (already relative or unknown format)
	painlessScript := fmt.Sprintf(`
		if (ctx._source.blog != null && ctx._source.blog.blocks != null) {
			for (def block : ctx._source.blog.blocks) {
				if (block.data != null && block.data.file != null && block.data.file.url != null) {
					def url = block.data.file.url;

					def v1PostIdx = url.indexOf('%s');
					if (v1PostIdx >= 0) {
						block.data.file.url = '%s' + url.substring(v1PostIdx + %d);
						continue;
					}

					def v1ProfileIdx = url.indexOf('%s');
					if (v1ProfileIdx >= 0) {
						block.data.file.url = '%s' + url.substring(v1ProfileIdx + %d);
						continue;
					}

					def v2Idx = url.indexOf('%s');
					if (v2Idx >= 0 && (url.startsWith('http://') || url.startsWith('https://'))) {
						block.data.file.url = url.substring(v2Idx);
					}
				}
			}
		}
	`, v1PostPath, v2PostPath, len(v1PostPath),
		v1ProfilePath, v2ProfilePath, len(v1ProfilePath),
		v2StoragePath)

	updateBody := map[string]interface{}{
		"script": map[string]interface{}{
			"source": painlessScript,
			"lang":   "painless",
		},
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"should": []map[string]interface{}{
					{"wildcard": map[string]interface{}{
						"blog.blocks.data.file.url": map[string]interface{}{
							"value": "*" + v1PostPath + "*",
						},
					}},
					{"wildcard": map[string]interface{}{
						"blog.blocks.data.file.url": map[string]interface{}{
							"value": "*" + v1ProfilePath + "*",
						},
					}},
					{"wildcard": map[string]interface{}{
						"blog.blocks.data.file.url": map[string]interface{}{
							"value": "https://*" + v2StoragePath + "*",
						},
					}},
					{"wildcard": map[string]interface{}{
						"blog.blocks.data.file.url": map[string]interface{}{
							"value": "http://*" + v2StoragePath + "*",
						},
					}},
				},
				"minimum_should_match": 1,
			},
		},
	}

	updateBytes, _ := json.Marshal(updateBody)

	updateRes, err := es.UpdateByQuery(
		[]string{indexName},
		es.UpdateByQuery.WithContext(ctx),
		es.UpdateByQuery.WithBody(bytes.NewReader(updateBytes)),
		es.UpdateByQuery.WithConflicts("proceed"),
		es.UpdateByQuery.WithRefresh(true),
		es.UpdateByQuery.WithWaitForCompletion(true),
	)
	if err != nil {
		log.Fatalf("UpdateByQuery failed: %v", err)
	}
	defer updateRes.Body.Close()

	respBody, _ := io.ReadAll(updateRes.Body)

	if updateRes.IsError() {
		log.Fatalf("UpdateByQuery error: %s", respBody)
	}

	var updateResult struct {
		Updated  int           `json:"updated"`
		Total    int           `json:"total"`
		Failures []interface{} `json:"failures"`
	}
	if err := json.Unmarshal(respBody, &updateResult); err != nil {
		log.Fatalf("Failed to parse update response: %v", err)
	}

	log.Printf("Updated:  %d / %d documents", updateResult.Updated, updateResult.Total)
	if len(updateResult.Failures) > 0 {
		log.Printf("Failures: %d", len(updateResult.Failures))
		failBytes, _ := json.MarshalIndent(updateResult.Failures, "", "  ")
		log.Printf("%s", failBytes)
	}

	log.Println()
	log.Println("=== Migration Complete ===")
	log.Println("All file URLs are now relative paths (e.g. /api/v2/storage/posts/{id}/{file})")
	log.Println()
	log.Println("Next steps:")
	log.Println("  1. Verify blogs render correctly — images should load from the current origin")
	log.Println("  2. Remove rewriteV1StorageURLs() from blog/storage_rewriter.go")
	log.Println("  3. Remove all rewriteV1StorageURLs() call sites in blog/routes.go")
	log.Println("     (revert json.Unmarshal(rewriteV1StorageURLs(x), ...) → json.Unmarshal(x, ...))")
}
