-- Search v2 — Phase 1. Add a typo-tolerant, index-backed search column to
-- user_account.
--
-- Why a generated column?
--   The current /api/v1/user/search uses `ILIKE '%term%'` against username,
--   first_name, last_name. The leading % forbids B-tree usage so every search
--   becomes a sequential scan. We can fix that with a trigram GIN index, but
--   GIN indexes work best against a single text column rather than three. A
--   STORED generated column lets Postgres maintain the concatenation
--   transparently on every UPDATE/INSERT — no triggers, no application code
--   to forget.
--
-- Why include `bio` here when the current query does not?
--   v2 explicitly broadens the searched surface (matches user expectation:
--   they search "platform engineer" and find people whose bio says exactly
--   that). Keeping bio in the same column means one GIN index instead of two.
--
-- Indexing strategy:
--   1. search_doc  GIN(gin_trgm_ops) — typo-tolerant similarity matches
--      across username/first/last/bio in a single index seek.
--   2. username    GIN(gin_trgm_ops) — kept separately because exact &
--      prefix matches on username dominate user intent ("@dave") and we
--      want a tight, smaller index for that hot path.
--
-- Both indexes are CREATEd without IF NOT EXISTS gating via DO blocks would
-- be over-engineering; the migration runner gives us a single, ordered apply.
-- IF NOT EXISTS is still used so re-runs against a partially-applied env
-- (interrupted CI, etc.) are safe.

ALTER TABLE user_account
ADD COLUMN IF NOT EXISTS search_doc TEXT GENERATED ALWAYS AS (
    coalesce(username, '') || ' ' || coalesce(first_name, '') || ' ' || coalesce(last_name, '') || ' ' || coalesce(bio, '')
) STORED;

-- GIN trigram index on the combined doc.
-- Note: pg_trgm must already be enabled (migration 000006).
CREATE INDEX IF NOT EXISTS idx_user_account_search_doc_trgm ON user_account USING GIN (search_doc gin_trgm_ops);

-- Dedicated index on username for prefix / equality, also trigram-backed so
-- partial-username typeahead ("dav" → "dave") stays sub-millisecond.
CREATE INDEX IF NOT EXISTS idx_user_account_username_trgm ON user_account USING GIN (username gin_trgm_ops);