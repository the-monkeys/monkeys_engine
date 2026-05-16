-- Reverse of 000007_user_search_index.up.sql. Drop indexes first so the
-- column drop does not waste work rebuilding them.
DROP INDEX IF EXISTS idx_user_account_username_trgm;

DROP INDEX IF EXISTS idx_user_account_search_doc_trgm;

ALTER TABLE user_account DROP COLUMN IF EXISTS search_doc;