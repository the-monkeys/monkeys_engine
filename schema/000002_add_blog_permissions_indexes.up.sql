-- Adding indexes to blog_permissions table for better query performance
CREATE INDEX idx_blog_permissions_blog_id ON blog_permissions(blog_id);
CREATE INDEX idx_blog_permissions_user_id ON blog_permissions(user_id);
