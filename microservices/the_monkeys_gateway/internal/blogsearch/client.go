// Package blogsearch implements the search-v2 (Phase 3) blog search and
// suggest endpoints at the gateway tier.
//
// Design rationale:
//
//  1. Why query Elasticsearch directly from the gateway?
//     The blog gRPC service exposes only typed metadata search RPCs that
//     are heavily coupled to the v2 document shape. Reusing them would
//     either require an intrusive proto change (new RPC + regenerated
//     stubs across every service) or returning untyped `google.protobuf.Any`
//     wrappers that the gateway must immediately decode. Going direct
//     keeps the blast radius of search-v2 inside the gateway, where the
//     cache, rate limit, and observability already live.
//
//  2. Why hit the alias, not the physical index?
//     The Phase 2 cut-over swings `the_monkeys_blogs` between
//     `..._v2` and `..._v3` atomically. Targeting the alias means this
//     code keeps working through rollbacks with no redeploy.
//
//  3. Read-only by design. This package only issues `_search`. No bulk,
//     no scroll, no index writes — even by accident — so it cannot
//     corrupt the corpus during an incident.
package blogsearch

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/the-monkeys/the_monkeys/config"
	"go.uber.org/zap"
)

// BlogIndexAlias is the public alias the rest of the platform reads
// from. Centralised so the swap script and search handlers agree.
const BlogIndexAlias = "the_monkeys_blogs"

// Client wraps an Elasticsearch HTTP client with a short-deadline
// transport. Constructed once at gateway boot.
type Client struct {
	es  *elasticsearch.Client
	log *zap.SugaredLogger
}

// NewClient builds a Client. The underlying HTTP transport caps idle
// connections and the per-request response-header timeout so a slow ES
// node can never tie up gateway goroutines for the whole request
// lifetime. Returns a usable (but degraded) Client on dial failure —
// the search handler will fall back to a 503 rather than panicking the
// gateway during a partial outage.
func NewClient(cfg *config.Config, log *zap.SugaredLogger) *Client {
	// Prefer Host (OPENSEARCH_OS_HOST) so we line up with the rest of
	// the platform — blog & activity services dial Host too. Address
	// (OPENSEARCH_ADDRESS) is the host-facing override used by local
	// scripts; only fall back to it when Host is unset so dev tooling
	// keeps working outside docker compose.
	addr := cfg.Opensearch.Host
	if addr == "" {
		addr = cfg.Opensearch.Address
	}
	if addr == "" {
		log.Warn("blogsearch: no opensearch address configured; v2 blog search disabled")
		return &Client{log: log}
	}

	transport := &http.Transport{
		MaxIdleConnsPerHost:   16,
		ResponseHeaderTimeout: 800 * time.Millisecond,
		IdleConnTimeout:       90 * time.Second,
	}

	es, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{addr},
		Username:  cfg.Opensearch.Username,
		Password:  cfg.Opensearch.Password,
		Transport: transport,
	})
	if err != nil {
		log.Warnw("blogsearch: ES client init failed; v2 blog search disabled", "err", err)
		return &Client{log: log}
	}

	// Cheap eager ping so misconfiguration surfaces at boot, not the
	// first user query. We don't fail boot on error — search must
	// still serve traffic during a transient outage.
	pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if res, perr := es.Info(es.Info.WithContext(pingCtx)); perr != nil {
		log.Warnw("blogsearch: ES ping failed at boot", "addr", addr, "err", perr)
	} else {
		_ = res.Body.Close()
		log.Infow("blogsearch: connected", "addr", addr, "alias", BlogIndexAlias)
	}

	return &Client{es: es, log: log}
}

// Available reports whether the underlying ES client is usable.
// Handlers check this before issuing a request and return a 503 when
// false, so we never write a confusing 500 during a known outage.
func (c *Client) Available() bool {
	return c != nil && c.es != nil
}

// errSearchUnavailable is returned when the ES client is missing
// (mis-config at boot). Handlers map it to 503.
var errSearchUnavailable = errors.New("blogsearch: backend unavailable")
