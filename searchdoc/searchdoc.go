// Package searchdoc derives the flat search-v2 fields (title, summary,
// body, tags) from a legacy editorjs-style blog document.
//
// Why a separate package?
//   - Same logic must run in two places: the live write path
//     (SaveBlog / PublishBlog) and the one-shot reindex job in
//     scripts/reindex_blogs_v3. Centralising it guarantees both produce
//     byte-identical denormalised fields, so a doc indexed live looks
//     the same as one backfilled.
//   - Pure, dependency-free. No ES client, no Postgres, no logger.
//     Trivial to unit-test and impossible to misuse from the wrong
//     layer.
package searchdoc

import (
	"strings"
	"unicode/utf8"
)

// summaryMaxRunes bounds the auto-generated summary. We measure in
// runes not bytes so the cut-off never lands inside a multi-byte
// character (which would emit invalid UTF-8 into Elasticsearch and
// break highlight rendering on the frontend).
const summaryMaxRunes = 300

// bodyMaxRunes caps the total body length we index. Pathologically
// large blogs (10 MB+ of pasted log output) would otherwise blow the
// term-vector storage budget and turn highlight queries into a CPU
// hog. 200k runes ≈ 40 minutes of reading; anything beyond that is
// unlikely to be a real article.
const bodyMaxRunes = 200_000

// blockTextFields lists the editorjs `data.*` keys that carry user
// prose. Other keys (file.url, alignment, style, etc.) are skipped on
// purpose — they would only pollute the relevance score.
var blockTextFields = []string{"text", "content", "caption", "code"}

// Fields is the result of denormalising a blog document. Callers merge
// it onto the original map before sending it to Elasticsearch.
type Fields struct {
	Title   string
	Summary string
	Body    string
	Tags    []string
}

// Build extracts denormalised search fields from a legacy blog doc.
// The input is the same map shape the blog service writes today
// (top-level `blog.blocks[]`, `blog.tags`, etc.). Build never mutates
// the input.
//
// Algorithm:
//  1. First non-empty `header` block becomes the title.
//  2. Body = newline-joined text of every block that carries prose,
//     in document order.
//  3. Summary = first 300 runes of body, cut on a word boundary.
//  4. Tags = unique, trimmed, lowercase strings from doc["tags"] (or
//     doc["blog"]["tags"] for legacy v2 docs).
//
// Empty / missing values are silently tolerated — partial denormali-
// sation is better than a hard failure that drops the doc.
func Build(doc map[string]interface{}) Fields {
	blocks := extractBlocks(doc)

	var (
		title   string
		bodyBuf strings.Builder
	)
	// Pre-reserve a reasonable buffer: average blog ≈ 5 KB of prose.
	bodyBuf.Grow(8 << 10)

	for _, b := range blocks {
		btype, _ := b["type"].(string)
		data, _ := b["data"].(map[string]interface{})
		if data == nil {
			continue
		}

		text := blockText(data)
		if text == "" {
			continue
		}

		// Title = first non-empty header. Subsequent headers go into
		// the body so they remain searchable but don't override.
		if title == "" && strings.EqualFold(btype, "header") {
			title = text
			// Keep title in body too — users sometimes search for
			// title words expecting them to count toward body BM25.
		}

		if bodyBuf.Len() > 0 {
			bodyBuf.WriteByte('\n')
		}
		bodyBuf.WriteString(text)

		// Stop accumulating once we exceed the cap. We still walk
		// remaining blocks looking for a title if we don't have one,
		// but we no longer grow the body buffer.
		if bodyBuf.Len() > bodyMaxRunes*4 /* UTF-8 worst case */ {
			break
		}
	}

	body := truncateRunes(bodyBuf.String(), bodyMaxRunes)
	summary := makeSummary(body)
	tags := extractTags(doc)

	return Fields{
		Title:   title,
		Summary: summary,
		Body:    body,
		Tags:    tags,
	}
}

// Apply writes the derived fields onto a copy of the blog doc and
// returns it. The original map is left untouched so callers can keep
// referring to it after enrichment.
func Apply(doc map[string]interface{}) map[string]interface{} {
	if doc == nil {
		return nil
	}
	out := make(map[string]interface{}, len(doc)+4)
	for k, v := range doc {
		out[k] = v
	}
	f := Build(doc)
	// We only overwrite when we have a non-zero value; a partially
	// pre-populated doc (e.g. from a migration backfill) should not
	// be clobbered by an empty derivation.
	if f.Title != "" {
		out["title"] = f.Title
	}
	if f.Summary != "" {
		out["summary"] = f.Summary
	}
	if f.Body != "" {
		out["body"] = f.Body
	}
	if len(f.Tags) > 0 {
		out["tags"] = f.Tags
	}
	return out
}

func extractBlocks(doc map[string]interface{}) []map[string]interface{} {
	// Two legacy shapes: top-level `blocks` or nested `blog.blocks`.
	if raw, ok := doc["blocks"].([]interface{}); ok {
		return castBlockSlice(raw)
	}
	blog, _ := doc["blog"].(map[string]interface{})
	if blog == nil {
		return nil
	}
	raw, _ := blog["blocks"].([]interface{})
	return castBlockSlice(raw)
}

func castBlockSlice(raw []interface{}) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(raw))
	for _, b := range raw {
		if m, ok := b.(map[string]interface{}); ok {
			out = append(out, m)
		}
	}
	return out
}

// blockText pulls the first non-empty prose field from a block's data
// object. We check fields in priority order so a paragraph with both
// `text` and `caption` (unusual but possible) yields `text`.
func blockText(data map[string]interface{}) string {
	for _, key := range blockTextFields {
		if s, ok := data[key].(string); ok {
			s = stripHTMLLite(strings.TrimSpace(s))
			if s != "" {
				return s
			}
		}
	}
	// `items` (list/checklist) is a []string or []map; flatten if so.
	if items, ok := data["items"].([]interface{}); ok && len(items) > 0 {
		var sb strings.Builder
		for i, it := range items {
			if i > 0 {
				sb.WriteByte(' ')
			}
			switch v := it.(type) {
			case string:
				sb.WriteString(stripHTMLLite(v))
			case map[string]interface{}:
				if s, ok := v["text"].(string); ok {
					sb.WriteString(stripHTMLLite(s))
				} else if s, ok := v["content"].(string); ok {
					sb.WriteString(stripHTMLLite(s))
				}
			}
		}
		return strings.TrimSpace(sb.String())
	}
	return ""
}

// stripHTMLLite removes the small subset of inline HTML editorjs
// emits (<b>, <i>, <a>, <br>, etc.). A full HTML parser would be
// overkill — and a security trap, since we'd be re-emitting attacker
// input. We just drop everything between '<' and '>'.
func stripHTMLLite(s string) string {
	if !strings.ContainsRune(s, '<') {
		return s
	}
	var sb strings.Builder
	sb.Grow(len(s))
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func extractTags(doc map[string]interface{}) []string {
	raw, ok := doc["tags"].([]interface{})
	if !ok {
		if blog, _ := doc["blog"].(map[string]interface{}); blog != nil {
			raw, _ = blog["tags"].([]interface{})
		}
	}
	if len(raw) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, t := range raw {
		s, ok := t.(string)
		if !ok {
			continue
		}
		s = strings.TrimSpace(strings.ToLower(s))
		if s == "" {
			continue
		}
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func makeSummary(body string) string {
	if body == "" {
		return ""
	}
	if utf8.RuneCountInString(body) <= summaryMaxRunes {
		return body
	}
	clipped := truncateRunes(body, summaryMaxRunes)
	// Back off to the last whitespace so we don't slice mid-word.
	if idx := strings.LastIndexAny(clipped, " \n\t"); idx > summaryMaxRunes/2 {
		clipped = clipped[:idx]
	}
	return strings.TrimSpace(clipped) + "\u2026" // ellipsis
}

func truncateRunes(s string, n int) string {
	if n <= 0 || s == "" {
		return ""
	}
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	i := 0
	for pos := range s {
		if i == n {
			return s[:pos]
		}
		i++
	}
	return s
}
