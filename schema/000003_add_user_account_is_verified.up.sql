-- Adding is_verified field to user_account table
ALTER TABLE user_account ADD COLUMN is_verified BOOLEAN DEFAULT FALSE;
