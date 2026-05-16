package searchcache

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

// Event is the canonical structured-log payload emitted for every search
// request handled by the gateway. Keeping a typed shape (instead of free-form
// zap fields scattered across handlers) lets us build Grafana / Loki queries
// against stable field names: `search_event:"true"`, `latency_ms:>200`, etc.
type Event struct {
	Query       string        // raw user query (already-trimmed)
	Type        string        // "posts" | "authors" | "all" | "suggest"
	Limit       int           // requested page size after caps
	Offset      int           // legacy offset; 0 for cursor pagination
	Cursor      string        // opaque cursor, empty for first page
	ResultCount int           // results returned to client
	CacheHit    bool          // true when served from searchcache
	Latency     time.Duration // gateway-side wall clock
	UserID      string        // hashed account id, empty for anonymous
}

// LogSearchEvent emits one structured zap line per search call. The event is
// logged at Info because:
//   - Search is a low-volume, business-critical signal (≈10s of req/s peak).
//   - We need it in production aggregations regardless of LOG_LEVEL=info|warn.
//
// We never log raw user identifiers — only the hashed form — so dashboards
// can compute distinct-user counts without holding PII.
func LogSearchEvent(log *zap.SugaredLogger, e Event) {
	if log == nil {
		return
	}
	log.Infow("search_event",
		"q", e.Query,
		"type", e.Type,
		"limit", e.Limit,
		"offset", e.Offset,
		"cursor", e.Cursor,
		"result_count", e.ResultCount,
		"cache_hit", e.CacheHit,
		"latency_ms", e.Latency.Milliseconds(),
		"user_id_hash", e.UserID,
		"zero_result", e.ResultCount == 0,
	)
}

// HashUserID returns a stable, non-reversible identifier for a user. We use
// SHA-1 here (not for cryptographic security — only for cardinality control
// in logs). The full hex is 40 chars; we truncate to 16 to keep log lines
// compact while preserving practical uniqueness.
func HashUserID(accountID string) string {
	if accountID == "" {
		return ""
	}
	sum := sha1.Sum([]byte(accountID))
	return hex.EncodeToString(sum[:])[:16]
}

// CacheKey builds a deterministic cache key from a query, scope, and
// pagination tuple. It is intentionally NOT user-scoped — search results are
// the same for everyone — so multiple users searching the same term share a
// single cache entry.
//
// We hash the inputs rather than concatenating them raw so:
//  1. arbitrary user input cannot produce Redis-meta characters or
//     pathologically long keys.
//  2. the key length stays bounded at 28 base64 chars regardless of query
//     length.
func CacheKey(parts ...string) string {
	h := sha1.New()
	for i, p := range parts {
		if i > 0 {
			h.Write([]byte{0x1f}) // unit separator; cannot appear in user text
		}
		h.Write([]byte(strings.ToLower(p)))
	}
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

// CacheKeyInts is a small convenience for callers that mix string + int
// pagination fields, avoiding a strconv import in every handler.
func CacheKeyInts(s string, ints ...int) string {
	parts := make([]string, 0, len(ints)+1)
	parts = append(parts, s)
	for _, n := range ints {
		parts = append(parts, strconv.Itoa(n))
	}
	return CacheKey(parts...)
}
