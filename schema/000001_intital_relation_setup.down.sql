-- ================================
-- Drop Predefined Data Inserts
-- ================================
-- Deleting predefined permissions and roles
DELETE FROM permissions_granted;
DELETE FROM user_status;
DELETE FROM auth_provider;
DELETE FROM email_validation_status;
DELETE FROM clients;
DELETE FROM permissions;
DELETE FROM user_role;

-- ================================
-- Drop SMS and OTP-related Tables
-- ================================
DROP TABLE IF EXISTS otp_logs;
DROP TABLE IF EXISTS sms_notifications;

-- ================================
-- Drop WhatsApp Notification-related Tables
-- ================================
DROP TABLE IF EXISTS whatsapp_notifications;

-- ================================
-- Drop Email Notification-related Tables
-- ================================
DROP TABLE IF EXISTS email_templates;

-- ================================
-- Drop Browser Notification-related Tables
-- ================================
DROP TABLE IF EXISTS web_push_tokens;

-- ================================
-- Drop Notification-related Tables
-- ================================
DROP TABLE IF EXISTS user_notification_preferences;
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS notification_type;
DROP TABLE IF EXISTS notification_channel;

-- ================================
-- Drop Co-Author Activity Log Tables
-- ================================
DROP TABLE IF EXISTS co_author_activity_log;

-- ================================
-- Drop Blog Bookmarks Tables
-- ================================
DROP TABLE IF EXISTS blog_bookmarks;

-- ================================
-- Drop Topics-related Tables
-- ================================
DROP TABLE IF EXISTS user_interest;
DROP TABLE IF EXISTS topics;

-- ================================
-- Drop User Activity-related Tables
-- ================================
DROP TABLE IF EXISTS logged_in_devices;
DROP TABLE IF EXISTS user_account_log;
DROP TABLE IF EXISTS clients;

-- ================================
-- Drop Blog-related Tables
-- ================================
DROP TABLE IF EXISTS co_author_permissions;
DROP TABLE IF EXISTS co_author_invites;
DROP TABLE IF EXISTS blog_permissions;
DROP TABLE IF EXISTS blog;

-- ================================
-- Drop Permission-related Tables
-- ================================
DROP TABLE IF EXISTS permissions_granted;
DROP TABLE IF EXISTS permissions;

-- ================================
-- Drop User-related Tables
-- ================================
DROP TABLE IF EXISTS user_auth_info;
DROP TABLE IF EXISTS auth_provider;
DROP TABLE IF EXISTS email_validation_status;
DROP TABLE IF EXISTS user_account;
DROP INDEX IF EXISTS idx_user_account_email;
DROP INDEX IF EXISTS idx_user_account_username;
DROP TABLE IF EXISTS user_role;
DROP TABLE IF EXISTS user_status;
