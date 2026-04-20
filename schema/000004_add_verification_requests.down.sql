-- Remove verification_requests table
DROP INDEX IF EXISTS idx_verification_requests_status;

DROP INDEX IF EXISTS idx_verification_requests_username;

DROP TABLE IF EXISTS verification_requests;

-- Remove UNIQUE constraint on user_account.username
ALTER TABLE user_account
DROP CONSTRAINT IF EXISTS uq_user_account_username;