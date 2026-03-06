package blog

import (
	"bytes"
	"regexp"
)

// rewriteStorageURLs normalises file URLs in raw blog JSON to relative paths.
//
// Two categories of legacy URLs exist in Elasticsearch:
//
//  1. v1 path:  "https://monkeys.support/api/v1/files/post/{id}/{file}"
//     Rewritten to: "/api/v2/storage/posts/{id}/{file}"
//
//  2. Absolute v2: "https://monkeys.support/api/v2/storage/posts/{id}/{file}"
//     Rewritten to: "/api/v2/storage/posts/{id}/{file}"
//
// The result is always a domain-free relative path, making stored data
// environment-agnostic (works on localhost, staging, production).
//
// The regex-based approach strips ANY domain — no configuration required.
// Works with monkeys.support, dev.monkeys.support, localhost:8081, etc.
//
// TEMPORARY: Once the one-time ES migration script
// (scripts/migrate/migrate_es_storage_urls.go) has been executed in all
// environments, this function and all call sites can be deleted.

// absStorageURLRe matches an absolute URL (scheme + host) immediately before
// a storage API path. The captured group ($1) is the path we keep.
//
// Examples of what it matches:
//   - https://monkeys.support/api/v2/storage/  → keeps /api/v2/storage/
//   - http://localhost:8081/api/v1/files/       → keeps /api/v1/files/
//   - https://dev.monkeys.support/api/v2/storage/ → keeps /api/v2/storage/
//
// [^"/\s]+ matches the host portion (no quotes, slashes, or whitespace),
// so it stops correctly at the path boundary.
var absStorageURLRe = regexp.MustCompile(`https?://[^"/\s]+(/api/v[12]/(?:storage|files)/)`)

// Legacy v1 path prefixes — used after domain stripping to normalise v1→v2.
var (
	v1PostPrefix    = []byte("/api/v1/files/post/")
	v2PostPrefix    = []byte("/api/v2/storage/posts/")
	v1ProfilePrefix = []byte("/api/v1/files/profile/")
	v2ProfilePrefix = []byte("/api/v2/storage/profiles/")
)

// rewriteV1StorageURLs normalises legacy storage URLs to relative v2 paths.
//
// Processing order:
//  1. Regex-strip any domain prefix from storage URLs. This converts
//     "https://monkeys.support/api/v2/storage/" → "/api/v2/storage/"
//     regardless of which domain is in the data.
//  2. Replace v1 path prefixes with v2 equivalents. This handles any
//     remaining v1 paths (with or without domain, since step 1 may have
//     already stripped the domain).
//
// The regex only runs when an "http" prefix is found in the data (fast-path
// check via bytes.Contains), so there is zero overhead for already-clean data.
func rewriteV1StorageURLs(data []byte) []byte {
	// Step 1: Strip domain prefixes → make URLs relative.
	// Fast-path: skip regex if no "http" appears in the data.
	if bytes.Contains(data, []byte("http")) {
		data = absStorageURLRe.ReplaceAll(data, []byte("${1}"))
	}

	// Step 2: Rewrite v1 paths to v2.
	data = bytes.ReplaceAll(data, v1PostPrefix, v2PostPrefix)
	data = bytes.ReplaceAll(data, v1ProfilePrefix, v2ProfilePrefix)
	return data
}
