DROP TABLE IF EXISTS blog_bookmarks;

DROP TABLE IF EXISTS blog_permissions;

DROP TABLE IF EXISTS blog;

DROP TABLE IF EXISTS user_account_log;

DROP TABLE IF EXISTS logged_in_devices;

DROP TABLE IF EXISTS clients;

DROP TABLE IF EXISTS user_interest;

DROP TABLE IF EXISTS topics;

DROP TABLE IF EXISTS payment_info;

DROP TABLE IF EXISTS user_external_login;

DROP TABLE IF EXISTS user_account_status;

DROP TABLE IF EXISTS permissions_granted;

DROP TABLE IF EXISTS permissions;

DROP TABLE IF EXISTS user_auth_info;

DROP TABLE IF EXISTS auth_provider;

DROP TABLE IF EXISTS email_validation_status;

DROP TABLE IF EXISTS user_account;

DROP TABLE IF EXISTS user_role;

DROP TABLE IF EXISTS user_status;

DROP INDEX IF EXISTS idx_user_account_email;
DROP INDEX IF EXISTS idx_user_account_username;

DROP TABLE IF EXISTS user_credentials;
