# Search v2: Team Video Script and Q&A

> A ~12-minute walkthrough for the team. Plain English. Each section has a heading you can read on screen, a short script for the speaker, and the on-screen visual.
>
> Pair with the two docs:
> - [SEARCH_V2_DESIGN.md](SEARCH_V2_DESIGN.md)
> - [SEARCH_V2_IMPLEMENTATION_PLAN.md](SEARCH_V2_IMPLEMENTATION_PLAN.md)

---

## How to use this script

- Read the **Speaker** lines out loud.
- The **On screen** lines tell you what to show.
- Pause for 2 seconds between sections.
- Total target length: 10–12 minutes.

---

## 0. Title slide  (10 sec)

**On screen:** "Search v2 on Monkeys — Why, What, How." Monkeys logo. Date.

**Speaker:** "Hi team. This is a walkthrough of how we are rebuilding search on Monkeys. The goal is simple. When a user searches, they should find what they are looking for, fast. Today we do not always do that. Let me show you why, and what we are going to change."

---

## 1. What search looks like today  (1 min)

**On screen:** Quick screen recording — open the site, type a blog title in the search box, see weak or empty results. Then search a user with a small typo, see zero results.

**Speaker:** "First, the user experience. I am searching for a blog by its exact title. You can see the result is not on top, or sometimes not even on the page. Now I will misspell a user name by one letter. Zero results. This is not what people expect from search in 2026. Every other platform — Medium, GitHub, Twitter — would forgive that typo."

---

## 2. Why it happens — the blog side  (1 min 30 sec)

**On screen:** A simple diagram. Browser → Gateway → Blog Service → Elasticsearch.

**Speaker:** "Let me explain the path. The browser hits the gateway. The gateway calls the blog service over gRPC. The blog service builds an Elasticsearch query and asks the index. Three things are wrong here. One: the blog title is never stored as its own field. We only search the body text and the tags. So a perfect title match has nothing to match against. Two: we do not allow typos. The search engine looks for the exact tokens you typed. Three: we always sort by date, never by relevance. So a fresh, weak match beats a perfect older one."

---

## 3. Why it happens — the user side  (1 min)

**On screen:** Same diagram, but Gateway → User Service → Postgres.

**Speaker:** "For people search the story is different. We use Postgres with an `ILIKE` query and a wildcard on both sides, `percent term percent`. The leading percent sign means Postgres cannot use the index. Every search reads the whole user table. We also do not filter out inactive accounts, and we have no ranking. It is slow and it is loose."

---

## 4. What top platforms do  (1 min 30 sec)

**On screen:** Logos: Medium, GitHub, Twitter/X, Reddit, LinkedIn. Four bullet points appear one by one.

**Speaker:** "Let me borrow from the playbook of platforms that solved this years ago. Four ideas keep showing up. First: search many fields, not one, and give each field a different weight. Title matters more than body. Tags matter more than body. Second: be forgiving with typos, especially for short queries. Third: have a special small index for autocomplete, so the dropdown is instant. Fourth: rank by relevance first, then add a small boost for fresh content. Not the other way around. We are going to do all four."

---

## 5. The new architecture  (2 min)

**On screen:** The target diagram from §6 of the design doc.

**Speaker:** "Here is the new shape. One unified endpoint on the gateway, `slash api slash v2 slash search`. The gateway has three jobs. Validate the input. Try a short Redis cache. Then call the right service. We add a one-second timeout on every call. No bad query can hang the system."

**Speaker (continues):** "On the blog side we will create a new Elasticsearch index, version three. It has flat fields: title, summary, body, tags, author. We will use the English analyzer, so 'running' and 'run' map to the same token. We will use a multi-match query with field-level boosts, and we will turn on fuzziness. Sort goes by score first, then by date for tie-breaking."

**Speaker (continues):** "On the user side we will enable the Postgres trigram extension. We will build one combined text column called `search_doc` and put a GIN index on it. The query will rank exact username matches highest, prefix matches next, and trigram similarity after that. Inactive users are filtered out."

---

## 6. The front-end changes  (1 min)

**On screen:** Side-by-side mock: current search box vs. new search box with recent searches and highlighted matches.

**Speaker:** "On the front end, we will finally turn on the Zustand store that already exists. Recent searches will show when the box is empty. Live suggestions show while typing, with a 250 millisecond debounce and request cancellation. Result cards will highlight the matched words in bold. And the dropdown gets a clear 'see all results' link at the bottom."

---

## 7. How we ship it without breaking prod  (1 min 30 sec)

**On screen:** A row of five boxes, P0 through P5, with green ticks appearing.

**Speaker:** "Five phases. Phase zero is plumbing — Redis cache wrapper, logging, the trigram extension. No user impact. Phase one is people search v2 — small surface, big win. Phase two is the new blog index built side by side with the old one, behind an alias. Phase three is the new query path. Phase four is the front end. Phase five is cleanup and removing the old endpoints. Every phase is reversible. If something looks wrong, we point the alias back and disable the new route."

---

## 8. How we know it worked  (45 sec)

**On screen:** Four KPI tiles: zero-result rate, p95 latency, click-through rate, cache hit rate.

**Speaker:** "We will track four numbers from day one. Zero-result rate — what percentage of searches show nothing. Target under five percent. P95 latency — target two hundred milliseconds for blog, one hundred for users. Click-through rate — over forty percent. Cache hit rate during peak — over fifty percent. If a number does not move, we did not actually fix anything."

---

## 9. Closing  (15 sec)

**On screen:** Links to the design doc, the plan, and the issue tracker.

**Speaker:** "All the details, including the SQL, the Elasticsearch payloads, and the rollback plan, are in the design doc and the implementation plan. Read those when you start work on your phase. Questions, ping the platform channel. Thank you."

---

# Q&A — answers in plain English

Every question is one a teammate might ask. Each answer is short and concrete.

### 1. Why not just upgrade to a paid managed search service?
Cost and control. Elasticsearch is already part of our stack. We use it for activity logs as well. Adding a vendor would mean another bill, another credential to rotate, and another network hop. The fixes we need do not require a new tool, just a better mapping and a better query.

### 2. Why are we still using Postgres for people search and not Elasticsearch too?
Two reasons. First, we already keep all user data in Postgres and we update it often. Duplicating it into Elasticsearch means we have to keep two stores in sync, which is a class of bug we do not need. Second, with the trigram GIN index, Postgres can comfortably handle people search up to a million users at the latency we want.

### 3. What is a "trigram"?
A trigram is a three-character window into a word. The word "monkey" becomes the trigrams `mon`, `onk`, `nke`, `key`. When you search for `monky`, its trigrams are `mon`, `onk`, `nky`. Two trigrams overlap, so Postgres can score the match even though the spelling is wrong. That is how we get typo tolerance.

### 4. What is a GIN index?
GIN means "Generalized Inverted iNdex". Think of the index at the back of a book — for every word, it tells you which pages contain that word. A GIN index does the same for every trigram in our text column. Postgres can then jump straight to candidate rows instead of scanning the whole table.

### 5. What is "fuzziness" in Elasticsearch?
Fuzziness lets a query match words that are close to the search term by a small edit distance. `monky` is one letter off from `monkey`, so with `fuzziness: AUTO` Elasticsearch will treat them as a match. We use `AUTO` because it scales with word length — short words get less leeway, long words get more. That avoids matching everything to everything.

### 6. What is `multi_match` doing?
`multi_match` runs the same query against several fields at once. Each field has a boost. We give `title` a boost of six, `tags` a boost of three, `body` a boost of one. The result is that a hit on the title counts six times more than a hit in the body. That is how we make title matches float to the top.

### 7. Why is sorting by `_score` better than sorting by date?
Sort by date is binary: a result is either before or after another result. Sort by `_score` is a number that measures how good the match is. If we sort by date we throw away the relevance information entirely. We still want fresh results, so we add a small recency boost inside the score, not a hard sort.

### 8. What is `search_after` and why is it faster than `from` + `size`?
With `from: 1000, size: 20`, Elasticsearch must internally collect 1020 results, then throw the first 1000 away. With `search_after`, the client passes back the sort values of the last item on the previous page. Elasticsearch jumps straight past them. Constant cost, no matter how deep you go.

### 9. Will the new index be bigger?
Roughly double. We are storing the body text as its own field now. At our current volume — a few thousand published blogs — that is still tiny. If we hit millions we will revisit.

### 10. How do we re-index without downtime?
We use an alias. The name `the_monkeys_blogs` is currently an alias pointing at `..._v2`. We build `..._v3` next to it, copy data over with a reindex script, then atomically swap the alias to point at `..._v3`. Search reads continue the whole time.

### 11. What happens to drafts?
Drafts are still indexed, because we want autosave to keep working. But the search query has a filter `is_draft: false`. Drafts never appear in results. If a user publishes a blog, the update sets `is_draft` to false and the same document instantly becomes searchable.

### 12. Is this safe from XSS through highlighting?
Yes if we are careful. The highlight feature wraps matched words in `<mark>` tags. We only enable highlighting on fields we control (`title`, `summary`). We do not enable it on raw body HTML, and we sanitise the highlight HTML server-side before returning it. The front end then renders it with `dangerouslySetInnerHTML` only for those whitelisted fields.

### 13. Can someone hammer the search endpoint and bring us down?
Less than today. We add three protections. A hard `limit` cap of 50 on the server. A request timeout of one second on the gateway. A short Redis cache that absorbs repeated identical queries. Rate limiting is the existing gateway middleware, which we will keep.

### 14. What about Hindi, French, or other languages?
Out of scope for v2. The English analyzer will still tokenize them on whitespace, so basic matching works. Proper multi-language support — stemming, accents, transliteration — is a v3 topic.

### 15. What about semantic search, like ChatGPT-style "you mean…"?
Also v3. That needs a vector store and an embedding model. We can layer it on top of the same gateway endpoint later without breaking clients. The v2 design leaves room for it.

### 16. Why a unified `/api/v2/search` endpoint instead of two?
Consistency. Today the front end juggles two URLs and two API versions. A unified endpoint with `?type=` makes it easier to add a third entity later (topics, comments, whatever) without changing the client.

### 17. Why not use Redis as the primary search store?
Redis is brilliant for cache and short-lived data, but it is not a search engine. It has no relevance scoring, no field-level boosts, no analyzers. We use it the right way — as a cache layer in front of Elasticsearch.

### 18. How do we measure if search "works" after launch?
Four numbers, watched weekly:
1. Zero-result rate. Lower is better.
2. p95 latency. Lower is better.
3. Click-through rate from a search session. Higher is better.
4. Cache hit rate. Higher is better.

We will also keep a leaderboard of "top searches with zero results" — that is our backlog for synonyms and content gaps.

### 19. What is the rollback story if v2 misbehaves in prod?
Each phase has its own rollback. The big one — the new blog query — is just a flag flip at the gateway. Point the alias back to the old index, disable the new route, and we are back to today's behaviour in under a minute.

### 20. When does the old endpoint go away?
After one release where the new endpoint serves all production traffic without alerts. Until then, both endpoints live in parallel. The cleanup step in Phase 5 is the only non-reversible change in the whole plan, and we do it only after the success metrics are green for a full week.

### 21. Do I need to know Elasticsearch to work on this?
Only if you are picking up Phase 2 or Phase 3. For Phase 1 (Postgres / user search) and Phase 4 (front end), the design doc has everything you need. Pair with someone who has touched the blog service before for your first Elasticsearch PR.

### 22. Where do I start?
1. Read [SEARCH_V2_DESIGN.md](SEARCH_V2_DESIGN.md).
2. Read [SEARCH_V2_IMPLEMENTATION_PLAN.md](SEARCH_V2_IMPLEMENTATION_PLAN.md).
3. Pick a phase from the plan. Comment in the team channel that you are taking it.
4. Open a draft PR early so we can review the approach before you finish the code.
