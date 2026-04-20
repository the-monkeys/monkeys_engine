-- Add UNIQUE constraint on user_account.username (required for FK reference)
ALTER TABLE user_account
ADD CONSTRAINT uq_user_account_username UNIQUE (username);

-- Create verification_requests table for user verification checkmark system
CREATE TABLE IF NOT EXISTS verification_requests (
    id VARCHAR(64) PRIMARY KEY,
    username VARCHAR(32) NOT NULL,
    verification_type VARCHAR(50) NOT NULL, -- 'social_proof', 'id_document', 'professional'
    proof_urls TEXT NOT NULL, -- Comma-separated URLs
    additional_info TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- 'pending', 'approved', 'rejected'
    reviewer_username VARCHAR(32),
    rejection_reason TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    reviewed_at TIMESTAMP,
    FOREIGN KEY (username) REFERENCES user_account (username) ON DELETE CASCADE
);

CREATE INDEX idx_verification_requests_username ON verification_requests (username);

CREATE INDEX idx_verification_requests_status ON verification_requests (status);