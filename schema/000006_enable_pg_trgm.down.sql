-- Reverse of 000006_enable_pg_trgm.up.sql.
--
-- We use IF EXISTS so the down migration is idempotent. We deliberately do
-- NOT cascade — if any index still depends on pg_trgm (e.g. a Phase 1
-- migration ran), the operator wants to be told rather than silently lose
-- those indexes. The proper rollback order is: down the trgm-dependent
-- migration first, then this one.
DROP EXTENSION IF EXISTS pg_trgm;