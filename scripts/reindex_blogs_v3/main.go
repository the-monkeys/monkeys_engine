// Command reindex_blogs_v3 is a one-shot job that scrolls the legacy
// the_monkeys_blogs (or the_monkeys_blogs_v2) Elasticsearch index,
// denormalises each document using the same searchdoc.Apply helper the
// live write path uses, and bulk-indexes the result into a fresh
// the_monkeys_blogs_v3 index.
//
// Operational flow:
//
//  1. PUT  /the_monkeys_blogs_v3    with documents/reindex/mapping_v3.json
//  2. go run ./scripts/reindex_blogs_v3 -src the_monkeys_blogs_v2 -dst the_monkeys_blogs_v3
//  3. POST /_aliases   { actions: [{ remove: { index: "*", alias: "the_monkeys_blogs" }},
//     { add:    { index: "the_monkeys_blogs_v3", alias: "the_monkeys_blogs" }} ] }
//
// Safety properties:
//
//   - Idempotent. Documents are re-indexed using their existing `_id`
//     (the blog_id) so re-runs overwrite, never duplicate.
//   - Bounded memory. Uses scroll + bulk in fixed batches of 500;
//     never holds the full corpus in memory.
//   - Read-only against the source. We only POST to `/{src}/_search`
//     and `/_search/scroll`, never to `/{src}/_doc`.
//   - Defensive on transport. A failing bulk request fails the job
//     loudly rather than silently dropping batches.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/the-monkeys/the_monkeys/searchdoc"
)

// scrollBatchSize and bulkBatchSize are deliberately equal so the
// number of in-flight docs is bounded at exactly one window. Larger
// values trade memory for throughput; 500 is a safe default that
// keeps the bulk request under the 100 MB ES limit on average docs.
const (
	scrollBatchSize = 500
	scrollKeepAlive = "2m"
	bulkBatchSize   = 500
)

func main() {
	var (
		addr      = flag.String("addr", envOr("ES_ADDR", "http://localhost:9201"), "Elasticsearch URL")
		username  = flag.String("user", os.Getenv("ES_USERNAME"), "Elasticsearch basic auth username (optional)")
		password  = flag.String("pass", os.Getenv("ES_PASSWORD"), "Elasticsearch basic auth password (optional)")
		srcIndex  = flag.String("src", "the_monkeys_blogs_v2", "Source index to read from")
		dstIndex  = flag.String("dst", "the_monkeys_blogs_v3", "Destination index to write to")
		dryRun    = flag.Bool("dry-run", false, "Read+transform but do not write")
		mustExist = flag.Bool("require-dst", true, "Fail fast if the destination index does not already exist (set false to allow auto-creation)")
	)
	flag.Parse()

	es, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{*addr},
		Username:  *username,
		Password:  *password,
		Transport: &http.Transport{
			MaxIdleConnsPerHost:   16,
			ResponseHeaderTimeout: 30 * time.Second,
		},
	})
	if err != nil {
		log.Fatalf("reindex: build ES client: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if *mustExist && !*dryRun {
		if err := assertIndexExists(ctx, es, *dstIndex); err != nil {
			log.Fatalf("reindex: %v", err)
		}
	}

	stats, err := run(ctx, es, *srcIndex, *dstIndex, *dryRun)
	if err != nil {
		log.Fatalf("reindex: failed after %s: %v", stats, err)
	}
	log.Printf("reindex: done %s", stats)
}

type stats struct {
	read    int
	written int
	skipped int
	errors  int
	started time.Time
}

func (s stats) String() string {
	return fmt.Sprintf("read=%d written=%d skipped=%d errors=%d elapsed=%s",
		s.read, s.written, s.skipped, s.errors, time.Since(s.started).Round(time.Millisecond))
}

func run(ctx context.Context, es *elasticsearch.Client, src, dst string, dryRun bool) (stats, error) {
	st := stats{started: time.Now()}

	// Initial scroll: match_all, sorted by _doc for cheapest traversal.
	body := strings.NewReader(`{"size":` + itoa(scrollBatchSize) + `,"sort":["_doc"],"query":{"match_all":{}}}`)
	res, err := es.Search(
		es.Search.WithContext(ctx),
		es.Search.WithIndex(src),
		es.Search.WithBody(body),
		es.Search.WithScroll(parseDuration(scrollKeepAlive)),
		es.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return st, fmt.Errorf("initial search: %w", err)
	}

	scrollID, hits, err := readScroll(res)
	if err != nil {
		return st, fmt.Errorf("decode initial: %w", err)
	}
	defer func() { _ = clearScroll(context.Background(), es, scrollID) }()

	for len(hits) > 0 {
		st.read += len(hits)

		batch := make([]bulkOp, 0, len(hits))
		for _, h := range hits {
			doc, ok := h.Source.(map[string]interface{})
			if !ok || doc == nil {
				st.skipped++
				continue
			}
			enriched := searchdoc.Apply(doc)
			batch = append(batch, bulkOp{ID: h.ID, Doc: enriched})
		}

		if !dryRun && len(batch) > 0 {
			written, errCount, err := bulkIndex(ctx, es, dst, batch)
			st.written += written
			st.errors += errCount
			if err != nil {
				return st, fmt.Errorf("bulk: %w", err)
			}
		} else {
			st.written += len(batch)
		}

		log.Printf("reindex: progress %s", st)

		// Next page.
		nextRes, err := es.Scroll(
			es.Scroll.WithContext(ctx),
			es.Scroll.WithScrollID(scrollID),
			es.Scroll.WithScroll(parseDuration(scrollKeepAlive)),
		)
		if err != nil {
			return st, fmt.Errorf("scroll: %w", err)
		}
		scrollID, hits, err = readScroll(nextRes)
		if err != nil {
			return st, fmt.Errorf("decode scroll page: %w", err)
		}
	}

	return st, nil
}

type hit struct {
	ID     string      `json:"_id"`
	Source interface{} `json:"_source"`
}

type bulkOp struct {
	ID  string
	Doc map[string]interface{}
}

func readScroll(res *esapi.Response) (string, []hit, error) {
	defer func() { _ = res.Body.Close() }()
	if res.IsError() {
		b, _ := io.ReadAll(res.Body)
		return "", nil, fmt.Errorf("es error %s: %s", res.Status(), string(b))
	}
	var payload struct {
		ScrollID string `json:"_scroll_id"`
		Hits     struct {
			Hits []hit `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return "", nil, err
	}
	return payload.ScrollID, payload.Hits.Hits, nil
}

func clearScroll(ctx context.Context, es *elasticsearch.Client, id string) error {
	if id == "" {
		return nil
	}
	res, err := es.ClearScroll(
		es.ClearScroll.WithContext(ctx),
		es.ClearScroll.WithScrollID(id),
	)
	if err != nil {
		return err
	}
	return res.Body.Close()
}

// bulkIndex serialises a single ES bulk request. We hand-roll the NDJSON
// instead of using esutil.BulkIndexer because (a) we want explicit error
// counts per batch and (b) we don't need parallel workers — the bottleneck
// is ES itself, and one in-flight bulk keeps the destination shard
// merge-balanced.
func bulkIndex(ctx context.Context, es *elasticsearch.Client, index string, ops []bulkOp) (written, failed int, err error) {
	var buf bytes.Buffer
	buf.Grow(1 << 20) // 1 MiB initial; bulks rarely exceed 10 MiB.

	enc := json.NewEncoder(&buf)
	for _, op := range ops {
		meta := map[string]interface{}{
			"index": map[string]interface{}{
				"_index": index,
				"_id":    op.ID,
			},
		}
		if err := enc.Encode(meta); err != nil {
			return 0, 0, err
		}
		if err := enc.Encode(op.Doc); err != nil {
			return 0, 0, err
		}
	}

	res, err := es.Bulk(bytes.NewReader(buf.Bytes()),
		es.Bulk.WithContext(ctx),
		es.Bulk.WithIndex(index),
	)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.IsError() {
		b, _ := io.ReadAll(res.Body)
		return 0, len(ops), fmt.Errorf("bulk http %s: %s", res.Status(), string(b))
	}

	var resp struct {
		Errors bool `json:"errors"`
		Items  []map[string]struct {
			Status int             `json:"status"`
			Error  json.RawMessage `json:"error,omitempty"`
		} `json:"items"`
	}
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return 0, 0, err
	}
	for _, item := range resp.Items {
		for _, v := range item {
			if v.Status >= 200 && v.Status < 300 {
				written++
			} else {
				failed++
				log.Printf("reindex: bulk item failed status=%d err=%s", v.Status, string(v.Error))
			}
		}
	}
	return written, failed, nil
}

func assertIndexExists(ctx context.Context, es *elasticsearch.Client, name string) error {
	res, err := es.Indices.Exists([]string{name}, es.Indices.Exists.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("check exists: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode == 404 {
		return fmt.Errorf("destination index %q does not exist (create it from documents/reindex/mapping_v3.json first, or pass -require-dst=false)", name)
	}
	if res.IsError() {
		return fmt.Errorf("unexpected status checking %q: %s", name, res.Status())
	}
	return nil
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 2 * time.Minute
	}
	return d
}
