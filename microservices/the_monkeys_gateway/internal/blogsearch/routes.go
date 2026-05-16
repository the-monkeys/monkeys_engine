package blogsearch

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/internal/cache/searchcache"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/middleware"
	"go.uber.org/zap"
)

// Validation/back-pressure constants. All values must clamp untrusted
// input BEFORE it reaches Elasticsearch — a fuzzy multi_match over a
// 10 KB query string costs orders of magnitude more than over 40 chars.
const (
	searchQueryMinLen  = 1
	searchQueryMaxLen  = 128
	searchLimitDefault = 20
	searchLimitMax     = 50

	suggestQueryMinLen  = 2 // anything less is too noisy for autocomplete
	suggestQueryMaxLen  = 64
	suggestLimitDefault = 5
	suggestLimitMax     = 10

	searchHTTPTimeout  = 1 * time.Second
	suggestHTTPTimeout = 300 * time.Millisecond

	searchCacheTTL  = 60 * time.Second
	suggestCacheTTL = 30 * time.Second
)

// Handlers carries the ES client + cache so the gin route handlers
// stay slim methods.
type Handlers struct {
	es    *Client
	cache *searchcache.Cache
	log   *zap.SugaredLogger
}

// RegisterRoutes wires the search-v2 (Phase 3) endpoints under the
// supplied gin engine. Public — no auth — because search is anonymous
// browsing. The rate limiter prevents enumeration scrapers from
// flooding the ES cluster.
func RegisterRoutes(router *gin.Engine, cfg *config.Config, cache *searchcache.Cache, log *zap.SugaredLogger) *Handlers {
	h := &Handlers{
		es:    NewClient(cfg, log),
		cache: cache,
		log:   log,
	}

	// Tighter limit than the generic /api/v2 group because search
	// queries are 5-10x more expensive than a typical metadata fetch.
	searchRL := middleware.RateLimiterMiddleware("30-S")

	blogV2 := router.Group("/api/v2/blog")
	{
		// `/search/v2` rather than overloading `/search` so the old
		// (v1) handler keeps working through the cut-over window.
		blogV2.GET("/search/v2", searchRL, h.Search)
	}

	searchV2 := router.Group("/api/v2/search")
	{
		searchV2.GET("/suggest", searchRL, h.Suggest)
	}

	return h
}

// Search handles GET /api/v2/blog/search/v2?q=&cursor=&limit=
func (h *Handlers) Search(ctx *gin.Context) {
	started := time.Now()

	q := strings.TrimSpace(ctx.Query("q"))
	// Back-compat: the v1 endpoint named the param `search_term`.
	// Accept both so the frontend can flip a single flag.
	if q == "" {
		q = strings.TrimSpace(ctx.Query("search_term"))
	}
	if len(q) < searchQueryMinLen {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "q is required"})
		return
	}
	if len(q) > searchQueryMaxLen {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "q too long"})
		return
	}

	limit, err := strconv.Atoi(ctx.DefaultQuery("limit", strconv.Itoa(searchLimitDefault)))
	if err != nil || limit <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "invalid limit"})
		return
	}
	if limit > searchLimitMax {
		limit = searchLimitMax
	}

	cursor := strings.TrimSpace(ctx.Query("cursor"))

	if !h.es.Available() {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"message": "search temporarily unavailable"})
		return
	}

	// Cache key intentionally lower-cases the query so equivalent
	// searches collapse onto one entry. The cursor is included raw
	// because pagination state has to be exact.
	cacheKey := searchcache.CacheKey(
		"blog:search:v2",
		strings.ToLower(q),
		cursor,
		strconv.Itoa(limit),
	)

	if cached, cErr := h.cache.Get(ctx.Request.Context(), cacheKey); cErr == nil && cached != "" {
		searchcache.LogSearchEvent(h.log, searchcache.Event{
			Query:       q,
			Type:        "posts",
			Limit:       limit,
			Cursor:      cursor,
			ResultCount: -1,
			CacheHit:    true,
			Latency:     time.Since(started),
			UserID:      searchcache.HashUserID(ctx.GetString("accountId")),
		})
		ctx.Data(http.StatusOK, "application/json", []byte(cached))
		return
	}

	esCtx, cancel := context.WithTimeout(ctx.Request.Context(), searchHTTPTimeout)
	defer cancel()

	res, sErr := h.es.Search(esCtx, SearchOpts{Query: q, Limit: limit, Cursor: cursor})
	if sErr != nil {
		status := http.StatusInternalServerError
		msg := "search failed"
		if esCtx.Err() != nil {
			status = http.StatusGatewayTimeout
			msg = "search timed out"
		}
		h.log.Warnw("blogsearch v2: search failed", "err", sErr, "query_len", len(q))
		ctx.JSON(status, gin.H{"message": msg})
		return
	}

	body, _ := json.Marshal(res)
	h.cache.Set(ctx.Request.Context(), cacheKey, string(body), searchCacheTTL)

	searchcache.LogSearchEvent(h.log, searchcache.Event{
		Query:       q,
		Type:        "posts",
		Limit:       limit,
		Cursor:      cursor,
		ResultCount: len(res.Hits),
		CacheHit:    false,
		Latency:     time.Since(started),
		UserID:      searchcache.HashUserID(ctx.GetString("accountId")),
	})
	ctx.Data(http.StatusOK, "application/json", body)
}

// Suggest handles GET /api/v2/search/suggest?q=&limit=
//
// Suggest is intentionally cheaper than full search: smaller limit,
// shorter cache TTL is irrelevant (a suggest re-issued in the typing
// window must still hit fresh-ish data), no fuzziness, no highlights.
func (h *Handlers) Suggest(ctx *gin.Context) {
	started := time.Now()

	q := strings.TrimSpace(ctx.Query("q"))
	if len(q) < suggestQueryMinLen {
		// Empty / too-short queries return an empty list instead of
		// an error so the frontend autocomplete can call this on
		// every keystroke without special-casing.
		ctx.JSON(http.StatusOK, gin.H{"hits": []Hit{}})
		return
	}
	if len(q) > suggestQueryMaxLen {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "q too long"})
		return
	}

	limit, err := strconv.Atoi(ctx.DefaultQuery("limit", strconv.Itoa(suggestLimitDefault)))
	if err != nil || limit <= 0 {
		limit = suggestLimitDefault
	}
	if limit > suggestLimitMax {
		limit = suggestLimitMax
	}

	if !h.es.Available() {
		// Suggest fails closed — better an empty dropdown than a 5xx
		// while the user is mid-typing.
		ctx.JSON(http.StatusOK, gin.H{"hits": []Hit{}})
		return
	}

	cacheKey := searchcache.CacheKey("blog:suggest:v2", strings.ToLower(q), strconv.Itoa(limit))

	if cached, cErr := h.cache.Get(ctx.Request.Context(), cacheKey); cErr == nil && cached != "" {
		searchcache.LogSearchEvent(h.log, searchcache.Event{
			Query:       q,
			Type:        "suggest",
			Limit:       limit,
			ResultCount: -1,
			CacheHit:    true,
			Latency:     time.Since(started),
			UserID:      searchcache.HashUserID(ctx.GetString("accountId")),
		})
		ctx.Data(http.StatusOK, "application/json", []byte(cached))
		return
	}

	esCtx, cancel := context.WithTimeout(ctx.Request.Context(), suggestHTTPTimeout)
	defer cancel()

	res, sErr := h.es.Suggest(esCtx, SuggestOpts{Query: q, Limit: limit})
	if sErr != nil {
		h.log.Debugw("blogsearch v2: suggest failed (returning empty)", "err", sErr, "query_len", len(q))
		ctx.JSON(http.StatusOK, gin.H{"hits": []Hit{}})
		return
	}

	body, _ := json.Marshal(gin.H{"hits": res.Hits})
	h.cache.Set(ctx.Request.Context(), cacheKey, string(body), suggestCacheTTL)

	searchcache.LogSearchEvent(h.log, searchcache.Event{
		Query:       q,
		Type:        "suggest",
		Limit:       limit,
		ResultCount: len(res.Hits),
		CacheHit:    false,
		Latency:     time.Since(started),
		UserID:      searchcache.HashUserID(ctx.GetString("accountId")),
	})
	ctx.Data(http.StatusOK, "application/json", body)
}
