-- Creating user status table
CREATE TABLE IF NOT EXISTS user_status (
    id SERIAL PRIMARY KEY,
    status VARCHAR(100) NOT NULL
);

-- Creating user role table
CREATE TABLE IF NOT EXISTS user_role (
    id SERIAL PRIMARY KEY,
    role_desc VARCHAR(50) NOT NULL
);

-- Creating user account table
CREATE TABLE IF NOT EXISTS user_account (
    id BIGSERIAL PRIMARY KEY,
    account_id VARCHAR(64) NOT NULL,
    username VARCHAR(32) NOT NULL,
    first_name VARCHAR(32),
    last_name VARCHAR(32),
    email VARCHAR(128) NOT NULL UNIQUE, -- Ensuring email uniqueness
    date_of_birth DATE,
    role_id INTEGER,
    bio TEXT,
    avatar_url TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    address VARCHAR(255),
    contact_number VARCHAR(20), -- Changed data type
    user_status INTEGER NOT NULL,
    view_permission VARCHAR(50) DEFAULT 'public', -- 'public', 'private', 'friends', etc.
    FOREIGN KEY (user_status) REFERENCES user_status(id)
);

-- Adding unique constraint on user_id in user_account table
ALTER TABLE user_account
ADD CONSTRAINT user_id_unique UNIQUE (id);


-- Creating email validation status table
CREATE TABLE IF NOT EXISTS email_validation_status (
    id SERIAL PRIMARY KEY,
    status VARCHAR(100) UNIQUE NOT NULL
);

-- Creating auth provider table
CREATE TABLE IF NOT EXISTS auth_provider (
    id SERIAL PRIMARY KEY,
    provider_name VARCHAR(100) NOT NULL UNIQUE
);

-- Creating user authentication information table
CREATE TABLE IF NOT EXISTS user_auth_info (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    password_recovery_token VARCHAR(100),
    password_recovery_timeout TIMESTAMP,
    password_updated_at TIMESTAMP,
    email_validation_token VARCHAR(100),
    email_verification_timeout TIMESTAMP,
    email_validation_status INTEGER NOT NULL,
    email_validation_time TIMESTAMP,
    auth_provider_id INTEGER NOT NULL,
    FOREIGN KEY (user_id) REFERENCES user_account(id) ON DELETE CASCADE ON UPDATE NO ACTION,
    FOREIGN KEY (email_validation_status) REFERENCES email_validation_status(id) ON DELETE NO ACTION ON UPDATE NO ACTION,
    FOREIGN KEY (auth_provider_id) REFERENCES auth_provider(id) ON DELETE NO ACTION ON UPDATE NO ACTION
);

-- Creating permissions table
CREATE TABLE IF NOT EXISTS permissions (
    permission_id SERIAL PRIMARY KEY,
    permission_desc VARCHAR(50) NOT NULL
);

-- Creating permissions granted table
CREATE TABLE IF NOT EXISTS permissions_granted (
    role_id BIGINT NOT NULL,
    permission_id INTEGER NOT NULL,
    FOREIGN KEY (role_id) REFERENCES user_role(id) ON DELETE CASCADE ON UPDATE NO ACTION,
    FOREIGN KEY (permission_id) REFERENCES permissions(permission_id) ON DELETE CASCADE ON UPDATE NO ACTION
);

-- Creating user account status table workflow
CREATE TABLE IF NOT EXISTS user_account_status (
    id SERIAL PRIMARY KEY,
    user_id BIGSERIAL NOT NULL,
    account_status VARCHAR(20) NOT NULL,
    last_login TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, -- Added default value
    creation_date TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, -- Added default value
    modified_date TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, -- Added default value
    reason VARCHAR(100),
    FOREIGN KEY (user_id) REFERENCES user_account(id) ON DELETE CASCADE ON UPDATE NO ACTION
);

-- Creating user external login table
CREATE TABLE IF NOT EXISTS user_external_login (
    user_id BIGINT NOT NULL,
    auth_provider_id INTEGER NOT NULL,
    auth_token VARCHAR(100) NOT NULL,
    FOREIGN KEY (user_id) REFERENCES user_account(id) ON DELETE CASCADE ON UPDATE NO ACTION,
    FOREIGN KEY (auth_provider_id) REFERENCES auth_provider(id) ON DELETE CASCADE ON UPDATE NO ACTION
);

-- Creating payment info table
CREATE TABLE IF NOT EXISTS payment_info (
    subscription_info VARCHAR(50) NOT NULL,
    payment_info VARCHAR(50) NOT NULL,
    user_id BIGINT NOT NULL,
    PRIMARY KEY (user_id), -- Removed profile_id from primary key
    FOREIGN KEY (user_id) REFERENCES user_account(id) ON DELETE CASCADE ON UPDATE NO ACTION
);

-- Creating topics table
CREATE TABLE IF NOT EXISTS topics (
    id SERIAL PRIMARY KEY,
    description VARCHAR(100) NOT NULL,
    category VARCHAR(100) NOT NULL,
    user_id BIGINT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES user_account(id) ON DELETE CASCADE ON UPDATE NO ACTION
);

-- Creating user interestd in topics table
CREATE TABLE IF NOT EXISTS user_interest (
    user_id BIGINT NOT NULL,
    topics_id INTEGER NOT NULL,
    PRIMARY KEY (user_id, topics_id),
    FOREIGN KEY (user_id) REFERENCES user_account(id) ON DELETE CASCADE ON UPDATE NO ACTION,
    FOREIGN KEY (topics_id) REFERENCES topics(id) ON DELETE CASCADE ON UPDATE NO ACTION
);

-- Creating clients table
CREATE TABLE IF NOT EXISTS clients (
    id SERIAL PRIMARY KEY,
    c_name VARCHAR(32)
);

-- Creating logged in devices table
CREATE TABLE IF NOT EXISTS logged_in_devices (
    id SERIAL PRIMARY KEY,
    device_name VARCHAR(32),
    ip_address VARCHAR(64),
    operating_sys VARCHAR(32),
    login_time TIMESTAMP,
    user_id BIGINT NOT NULL,
    client_id INTEGER NOT NULL,
    FOREIGN KEY (user_id) REFERENCES user_account(id) ON DELETE CASCADE ON UPDATE NO ACTION,
    FOREIGN KEY (client_id) REFERENCES clients(id) ON DELETE CASCADE ON UPDATE NO ACTION
);

-- Creating user account log table
CREATE TABLE IF NOT EXISTS user_account_log (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    ip_address VARCHAR(20),
    description TEXT,
    client_id INTEGER NOT NULL,
    FOREIGN KEY (user_id) REFERENCES user_account(id) ON DELETE CASCADE ON UPDATE NO ACTION,
    FOREIGN KEY (client_id) REFERENCES clients(id) ON DELETE CASCADE ON UPDATE NO ACTION
);

CREATE TABLE IF NOT EXISTS blog (
    id BIGSERIAL PRIMARY KEY,
    owner_id BIGINT NOT NULL,
    title VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(50), -- e.g., 'draft', 'published', 'archived'
    FOREIGN KEY (owner_id) REFERENCES user_account(id) ON DELETE SET NULL ON UPDATE NO ACTION
);

CREATE TABLE IF NOT EXISTS blog_permissions (
    blog_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    permission_type VARCHAR(50) NOT NULL, -- 'owner', 'editor', 'viewer'
    PRIMARY KEY (blog_id, user_id),
    FOREIGN KEY (blog_id) REFERENCES blog(id) ON DELETE CASCADE ON UPDATE NO ACTION,
    FOREIGN KEY (user_id) REFERENCES user_account(id) ON DELETE CASCADE ON UPDATE NO ACTION
);


-- Inserting predefined roles
INSERT INTO user_role (role_desc) VALUES ('Admin'), ('Owner'), ('Editor'), ('Viewer');

-- Inserting predefined permissions
INSERT INTO permissions (permission_desc) VALUES ('Read'), ('Edit'), ('Delete'), ('Archive');

-- Inserting predefined clients
INSERT INTO clients (c_name) VALUES ('Chrome'), ('Firefox'), ('Safari'), ('Edge'), ('Opera'), ('Android'), ('iOS'), ('Brave'), ('Others');

-- Inserting predefined email validation statuses
INSERT INTO email_validation_status (status) VALUES ('Unverified'), ('Verification link sent'), ('verified');

-- Inserting predefined auth providers
INSERT INTO auth_provider (provider_name) VALUES ('The Monkeys'), ('Google Oauth2'), ('Instagram Oauth2');

-- Inserting predefined user statuses
INSERT INTO user_status (status) VALUES ('Active'), ('Inactive'), ('Hidden');

-- Inserting data into permissions granted for all roles
INSERT INTO permissions_granted (role_id, permission_id)
SELECT r.id, p.permission_id
FROM user_role r
JOIN permissions p ON r.role_desc IN ('Admin', 'Owner', 'Editor', 'Viewer')
AND p.permission_desc IN ('Read', 'Write', 'Edit', 'Delete', 'Archive');

-- Insert some default topics
INSERT INTO topics (description, category) VALUES
('Reading', 'Hobbies'),
('Writing', 'Hobbies'),
('Coding', 'Tech'),
('Hiking', 'Outdoors'),
('Photography', 'Hobbies'),
('Music', 'Entertainment'),
('Traveling', 'Lifestyle'),
('Painting', 'Arts'),
('Gardening', 'Hobbies'),
('Cooking', 'Food'),
('Dancing', 'Arts'),
('Sports', 'Fitness'),
('Gaming', 'Entertainment'),
('Blogging', 'Writing'),
('Volunteering', 'Social'),
('Fishing', 'Outdoors'),
('Crafting', 'Hobbies'),
('Collecting', 'Hobbies'),
('Food and Cuisine', 'Food'),
('Technology', 'Tech'),
('Business and Finance', 'Business'),
('Infrastructure', 'Business'),
('Agriculture', 'Science'),
('Healthcare', 'Science'),
('Science', 'Science'),
('Education', 'Learning'),
('Space', 'Science'),
('Movies', 'Entertainment'),
('Psychology', 'Science'),
('Mental Health', 'Wellness'),
('Research', 'Science'),
('Geography', 'Science'),
('Software', 'Tech'),
('Maths', 'Science'),
('Social Media', 'Communication'),
('The Internet', 'Communication'),
('Blockchain', 'Tech'),
('Language', 'Learning'),
('Spirituality', 'Wellness'),
('Hardware and IOTs', 'Tech'),
('Humour', 'Entertainment'),
('Opinion', 'Writing'),
('Books', 'Reading'),
('Trains', 'Transportation'),
('Aviation', 'Transportation'),
('Rock n Roll', 'Music'),
('Night Life', 'Entertainment'),
('Restaurants', 'Food'),
('Motivation', 'Self-Improvement'),
('Vibe', 'Lifestyle'),
('Scandinavia', 'Travel'),
('Economics', 'Business'),
('Brands', 'Business'),
('Careers', 'Business'),
('Automobiles', 'Transportation'),
('Fashion', 'Lifestyle'),
('Family', 'Lifestyle'),
('Television', 'Entertainment'),
('Design', 'Arts'),
('Startups', 'Business'),
('Mobiles', 'Tech'),
('Love and Romance', 'Relationships'),
('Emotions', 'Wellness'),
('Personal Development', 'Self-Improvement'),
('Nature', 'Outdoors');