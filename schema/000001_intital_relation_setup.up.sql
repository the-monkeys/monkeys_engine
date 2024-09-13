-- Creating user status table
CREATE TABLE IF NOT EXISTS user_status (
    id SERIAL PRIMARY KEY,
    status VARCHAR(100) NOT NULL UNIQUE
);

-- Creating user role table
CREATE TABLE IF NOT EXISTS user_role (
    id SERIAL PRIMARY KEY,
    role_desc VARCHAR(50) NOT NULL UNIQUE
);

-- Creating user account table
CREATE TABLE IF NOT EXISTS user_account (
    id BIGSERIAL PRIMARY KEY,
    account_id VARCHAR(64) NOT NULL UNIQUE,
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
    linkedin VARCHAR(255),
    github VARCHAR(255),
    twitter VARCHAR(255),
    instagram VARCHAR(255),
    view_permission VARCHAR(50) DEFAULT 'public', -- 'public', 'private', 'friends', etc.
    FOREIGN KEY (user_status) REFERENCES user_status(id)
);

-- Adding indexes to user_account table
CREATE INDEX idx_user_account_email ON user_account(email);
CREATE INDEX idx_user_account_username ON user_account(username);

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
    permission_desc VARCHAR(50) NOT NULL UNIQUE
);

-- Creating permissions granted table
CREATE TABLE IF NOT EXISTS permissions_granted (
    role_id BIGINT NOT NULL,
    permission_id INTEGER NOT NULL,
    FOREIGN KEY (role_id) REFERENCES user_role(id) ON DELETE CASCADE ON UPDATE NO ACTION,
    FOREIGN KEY (permission_id) REFERENCES permissions(permission_id) ON DELETE CASCADE ON UPDATE NO ACTION,
    PRIMARY KEY (role_id, permission_id)
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
    FOREIGN KEY (auth_provider_id) REFERENCES auth_provider(id) ON DELETE CASCADE ON UPDATE NO ACTION,
    PRIMARY KEY (user_id, auth_provider_id)
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

-- Creating user interested in topics table
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
    c_name VARCHAR(32) UNIQUE
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

-- Creating blog table
CREATE TABLE IF NOT EXISTS blog (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    blog_id VARCHAR(255) NOT NULL UNIQUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(50), -- e.g., 'draft', 'published', 'archived'
    FOREIGN KEY (user_id) REFERENCES user_account(id) ON DELETE SET NULL ON UPDATE NO ACTION
);

-- Creating blog permissions table
CREATE TABLE IF NOT EXISTS blog_permissions (
    id BIGSERIAL PRIMARY KEY,
    blog_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    permission_type VARCHAR(50) NOT NULL, -- 'owner', 'editor', 'viewer'
    FOREIGN KEY (blog_id) REFERENCES blog(id) ON DELETE CASCADE ON UPDATE NO ACTION,
    FOREIGN KEY (user_id) REFERENCES user_account(id) ON DELETE CASCADE ON UPDATE NO ACTION
);

-- Table to store co-author invites
CREATE TABLE IF NOT EXISTS co_author_invites (
    id SERIAL PRIMARY KEY,
    blog_id BIGINT NOT NULL, -- Reference to the blog
    inviter_id BIGINT NOT NULL, -- Reference to the user (owner or admin) sending the invite
    invitee_id BIGINT NOT NULL, -- Reference to the user being invited as a co-author
    invite_status VARCHAR(50) NOT NULL DEFAULT 'pending', -- Can be 'pending', 'accepted', or 'rejected'
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    responded_at TIMESTAMP,
    FOREIGN KEY (blog_id) REFERENCES blog(id) ON DELETE CASCADE ON UPDATE NO ACTION,
    FOREIGN KEY (inviter_id) REFERENCES user_account(id) ON DELETE CASCADE ON UPDATE NO ACTION,
    FOREIGN KEY (invitee_id) REFERENCES user_account(id) ON DELETE CASCADE ON UPDATE NO ACTION
);

-- Table to store accepted co-author permissions
CREATE TABLE IF NOT EXISTS co_author_permissions (
    id SERIAL PRIMARY KEY,
    blog_id BIGINT NOT NULL, -- Reference to the blog
    co_author_id BIGINT NOT NULL, -- The invited user who accepted the invitation
    role_id BIGINT NOT NULL, -- Reference to the user role ('Editor', 'Viewer', etc.)
    granted_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (blog_id) REFERENCES blog(id) ON DELETE CASCADE ON UPDATE NO ACTION,
    FOREIGN KEY (co_author_id) REFERENCES user_account(id) ON DELETE CASCADE ON UPDATE NO ACTION,
    FOREIGN KEY (role_id) REFERENCES user_role(id) ON DELETE CASCADE ON UPDATE NO ACTION
);


-- Table to track actions related to co-author invitations and permissions
CREATE TABLE IF NOT EXISTS co_author_activity_log (
    id SERIAL PRIMARY KEY,
    blog_id BIGINT NOT NULL, -- Reference to the blog
    co_author_id BIGINT, -- Reference to the invited user (nullable in case of deletion logs)
    action VARCHAR(50) NOT NULL, -- 'invited', 'accepted', 'rejected', 'removed'
    performed_by BIGINT NOT NULL, -- Reference to the user who performed the action (inviter or owner/admin)
    action_timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (blog_id) REFERENCES blog(id) ON DELETE CASCADE ON UPDATE NO ACTION,
    FOREIGN KEY (co_author_id) REFERENCES user_account(id) ON DELETE SET NULL ON UPDATE NO ACTION,
    FOREIGN KEY (performed_by) REFERENCES user_account(id) ON DELETE CASCADE ON UPDATE NO ACTION
);

-- Creating blog bookmarks table
CREATE TABLE IF NOT EXISTS blog_bookmarks (
    id SERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    blog_id BIGINT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES user_account(id) ON DELETE CASCADE ON UPDATE NO ACTION,
    FOREIGN KEY (blog_id) REFERENCES blog(id) ON DELETE CASCADE ON UPDATE NO ACTION
);

-- Table to store notification types
CREATE TABLE IF NOT EXISTS notification_type (
    id SERIAL PRIMARY KEY,
    notification_name VARCHAR(100) NOT NULL UNIQUE,  -- E.g., 'Co-author invite', 'Blog liked', 'Comment on blog'
    description TEXT -- Optional description of the notification type
);

-- Table to store notifications for users
CREATE TABLE IF NOT EXISTS notifications (
    id SERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL, -- The user receiving the notification
    notification_type_id INTEGER NOT NULL, -- Type of notification (e.g., co-author invite, blog liked, etc.)
    message TEXT NOT NULL, -- Customizable message for the notification
    related_blog_id BIGINT, -- Optional reference to a related blog, if applicable
    related_user_id BIGINT, -- Optional reference to a related user (e.g., the one who liked or commented)
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, -- When the notification was created
    seen BOOLEAN DEFAULT FALSE, -- Whether the user has seen the notification
    FOREIGN KEY (user_id) REFERENCES user_account(id) ON DELETE CASCADE ON UPDATE NO ACTION,
    FOREIGN KEY (notification_type_id) REFERENCES notification_type(id) ON DELETE CASCADE ON UPDATE NO ACTION,
    FOREIGN KEY (related_blog_id) REFERENCES blog(id) ON DELETE SET NULL ON UPDATE NO ACTION,
    FOREIGN KEY (related_user_id) REFERENCES user_account(id) ON DELETE SET NULL ON UPDATE NO ACTION
);

-- Creating credentials table (note: in a production environment, sensitive data should be stored securely using encryption)
CREATE TABLE IF NOT EXISTS user_credentials (
    id SERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    credential_name VARCHAR(100) NOT NULL,
    credential_value TEXT NOT NULL, -- Ensure this data is encrypted in a real production setup
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES user_account(id) ON DELETE CASCADE ON UPDATE NO ACTION
);

-- Inserting predefined roles
INSERT INTO user_role (role_desc) VALUES ('Admin'), ('Owner'), ('Editor'), ('Viewer'), ('Support')
ON CONFLICT DO NOTHING;

-- Inserting predefined permissions
INSERT INTO permissions (permission_desc) VALUES ('Read'), ('Edit'), ('Delete'), ('Archive'), ('Transfer-Ownership'), ('Publish'), ('Draft')
ON CONFLICT DO NOTHING;

-- Inserting predefined clients
INSERT INTO clients (c_name) VALUES ('Chrome'), ('Firefox'), ('Safari'), ('Edge'), ('Opera'), ('Android'), ('iOS'), ('Brave'), ('Others')
ON CONFLICT DO NOTHING;

-- Inserting predefined email validation statuses
INSERT INTO email_validation_status (status) VALUES ('Unverified'), ('Verification link sent'), ('verified')
ON CONFLICT DO NOTHING;

-- Inserting predefined auth providers
INSERT INTO auth_provider (provider_name) VALUES ('The Monkeys'), ('Google Oauth2'), ('Instagram Oauth2')
ON CONFLICT DO NOTHING;

-- Inserting predefined user statuses
INSERT INTO user_status (status) VALUES ('Active'), ('Inactive'), ('Hidden')
ON CONFLICT DO NOTHING;

-- Inserting data into permissions granted for all roles with varying levels of permission
INSERT INTO permissions_granted (role_id, permission_id)
SELECT r.id, p.permission_id
FROM user_role r
JOIN permissions p ON 
    CASE 
        WHEN r.role_desc = 'Admin' THEN p.permission_desc IN ('Read', 'Edit', 'Delete', 'Archive', 'Transfer-Ownership', 'Publish', 'Draft')
        WHEN r.role_desc = 'Owner' THEN p.permission_desc IN ('Read', 'Edit', 'Delete', 'Archive', 'Transfer-Ownership', 'Publish', 'Draft')
        WHEN r.role_desc = 'Support' THEN p.permission_desc IN ('Read', 'Edit', 'Delete', 'Archive', 'Transfer-Ownership', 'Publish', 'Draft')
        WHEN r.role_desc = 'Editor' THEN p.permission_desc IN ('Read', 'Edit', 'Publish', 'Draft')
        WHEN r.role_desc = 'Viewer' THEN p.permission_desc IN ('Read')
    END
ON CONFLICT DO NOTHING;

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
('Television', 'Entertainment'),
('Design', 'Arts'),
('Startups', 'Business'),
('Mobiles', 'Tech'),
('Love and Romance', 'Relationships'),
('Emotions', 'Wellness'),
('Adoption','Family'),
('Children','Family'),
('Elder Care','Family'),
('Fatherhood','Family'),
('Motherhood','Family'),
('Parenting','Family'),
('Pregnancy','Family'),
('Seniors','Family'),
('Anxiety','Mental Health'),
('Counseling','Mental Health'),
('Grief','Mental Health'),
('Life Lessons','Mental Health'),
('Self-awareness','Mental Health'),
('Stress','Mental Health'),
('Therapy','Mental Health'),
('Trauma','Mental Health'),
('Entrepreneurship','Business'),
('Freelancing','Business'),
('Small Business','Business'),
('Startups','Business'),
('Venture Capital','Business'),
('Aging','Health'),
('Coronavirus','Health'),
('Covid-19','Health'),
('Death And Dying','Health'),
('Disease','Health'),
('Fitness','Health'),
('Mens Health','Health'),
('Nutrition','Health'),
('Sleep','Health'),
('Trans Healthcare','Health'),
('Vaccines','Health'),
('Weight Loss','Health'),
('Womens Health','Health'),
('Career Advice','Productivity'),
('Coaching','Productivity'),
('Goal Setting','Productivity'),
('Morning Routines','Productivity'),
('Pomodoro Technique','Productivity'),
('Time Mangement','Productivity'),
('Work Life Balance','Productivity'),
('Advertising','Marketing'),
('Branding','Marketing'),
('Content Marketing','Marketing'),
('Content Strategy','Marketing'),
('Digital Marketing','Marketing'),
('SEO','Marketing'),
('Social Media Marketing','Marketing'),
('Storytelling For Business','Marketing'),
('Dating','Relationships'),
('Divorce','Relationships'),
('Friendship','Relationships'),
('Love','Relationships'),
('Marriage','Relationships'),
('Polyamory','Relationships'),
('Guided Meditation','Mindfulness'),
('Journaling','Mindfulness'),
('Meditation','Mindfulness'),
('Transcendental Meditation','Mindfulness'),
('Yoga','Mindfulness'),
('Employee Engagement','Leadership'),
('Leadership Coaching','Leadership'),
('Leadership Development','Leadership'),
('Management','Leadership'),
('Meetings','Leadership'),
('Org Charts','Leadership'),
('Thought Leadership','Leadership'),
('Company Retreats','Remote Work'),
('Digital Nomads','Remote Work'),
('Distributed Teams','Remote Work'),
('Future Of Work','Remote Work'),
('Work From Home','Remote Work'),
('Erotica','Sexuality'),
('Sex','Sexuality'),
('Sexual Health','Sexuality'),
('Architecture','Home'),
('Home Improvement','Home'),
('Homeownership','Home'),
('Interior Design','Home'),
('Rental Property','Home'),
('Vacation Rental','Home'),
('Baking','Food'),
('Coffee','Food'),
('Cooking','Food'),
('Foodies','Food'),
('Restaurants','Food'),
('Tea','Food'),
('Cats','Pets'),
('Dog Training','Pets'),
('Dogs','Pets'),
('Hamster','Pets'),
('Horses','Pets'),
('Pet Care','Pets'),
('ChatGPT','Artificial Intelligence'),
('Conversational AI','Artificial Intelligence'),
('Deep Learning','Artificial Intelligence'),
('Large Language Models','Artificial Intelligence'),
('Machine Learning','Artificial Intelligence'),
('NLP','Artificial Intelligence'),
('Voice Assistant','Artificial Intelligence'),
('Android Development','Programming'),
('Coding','Programming'),
('Flutter','Programming'),
('Frontend Engineering','Programming'),
('IOS Development','Programming'),
('Mobile Development','Programming'),
('Software Engineering','Programming'),
('web Development','Programming'),
('Bitcoin','Blockchain'),
('Cryptocurrency', 'Blockchain'),
('Decentralized Finance', 'Blockchain'),
('Ethereum', 'Blockchain'),
('Nft', 'Blockchain'),
('Web3', 'Blockchain'),
('Analytics', 'Data Science'),
('Data Engineering', 'Data Science'),
('Data Visualization', 'Data Science'),
('Database Design', 'Data Science'),
('Sql', 'Data Science'),
('eBook', 'Gadgets'),
('Internet of Things', 'Gadgets'),
('iPad', 'Gadgets'),
('Smart Home', 'Gadgets'),
('Smartphones', 'Gadgets'),
('Wearables', 'Gadgets'),
('3D Printing', 'Makers'),
('Arduino', 'Makers'),
('DIY', 'Makers'),
('Raspberry Pi', 'Makers'),
('Robotics', 'Makers'),
('Cybersecurity', 'Security'),
('Data Security', 'Security'),
('Encryption', 'Security'),
('Infosec', 'Security'),
('Passwords', 'Security'),
('Privacy', 'Security'),
('Amazon', 'Tech Companies'),
('Apple', 'Tech Companies'),
('Google', 'Tech Companies'),
('Mastodon', 'Tech Companies'),
('Medium', 'Tech Companies'),
('Meta', 'Tech Companies'),
('Microsoft', 'Tech Companies'),
('Tiktok', 'Tech Companies'),
('Twitter', 'Tech Companies'),
('Accessibility', 'Design'),
('Design Systems', 'Design'),
('Design Thinking', 'Design'),
('Graphic Design', 'Design'),
('Icon Design', 'Design'),
('Inclusive Design', 'Design'),
('Product Design', 'Design'),
('Typography', 'Design'),
('UX Design', 'Design'),
('UX Research', 'Design'),
('Agile', 'Product Management'),
('Innovation', 'Product Management'),
('Kanban', 'Product Management'),
('Lean Startup', 'Product Management'),
('MVP', 'Product Management'),
('Product', 'Product Management'),
('Strategy', 'Product Management'),
('Angular', 'Programming Languages'),
('CSS', 'Programming Languages'),
('HTML', 'Programming Languages'),
('Java', 'Programming Languages'),
('JavaScript', 'Programming Languages'),
('Nodejs', 'Programming Languages'),
('Python', 'Programming Languages'),
('React', 'Programming Languages'),
('Ruby', 'Programming Languages'),
('Typescript', 'Programming Languages'),
('AWS', 'DevOps'),
('Databricks', 'DevOps'),
('Docker', 'DevOps'),
('Kubernetes', 'DevOps'),
('Terraform', 'DevOps'),
('Android', 'Operating Systems'),
('iOS', 'Operating Systems'),
('Linux', 'Operating Systems'),
('Macos', 'Operating Systems'),
('Windows', 'Operating Systems'),
('Writing', 'Media'),
('30 Day Challenge', 'Media'),
('Book Reviews', 'Media'),
('Books', 'Media'),
('Creative Nonfiction', 'Media'),
('Diary', 'Media'),
('Fiction', 'Media'),
('Haiku', 'Media'),
('Hello World', 'Media'),
('Memoir', 'Media'),
('Nonfiction', 'Media'),
('Personal Essay', 'Media'),
('Poetry', 'Media'),
('Screenwriting', 'Media'),
('Short Stories', 'Media'),
('This Happened To Me', 'Media'),
('Writing Prompts', 'Media'),
('Writing Tips', 'Media'),
('Comics', 'Art'),
('Contemporary Art', 'Art'),
('Drawing', 'Art'),
('Fine Art', 'Art'),
('Generative Art', 'Art'),
('Illustration', 'Art'),
('Painting', 'Art'),
('Portraits', 'Art'),
('Street Art', 'Art'),
('Game Design', 'Gaming'),
('Game Development', 'Gaming'),
('Indie Game', 'Gaming'),
('Metaverse', 'Gaming'),
('Nintendo', 'Gaming'),
('PlayStation', 'Gaming'),
('Videogames', 'Gaming'),
('Virtual Reality', 'Gaming'),
('Xbox', 'Gaming'),
('ComedyJokes', 'Humor'),
('Parody', 'Humor'),
('Satire', 'Humor'),
('Stand Up Comedy', 'Humor'),
('Cinema', 'Movies'),
('Film', 'Movies'),
('Filmmaking', 'Movies'),
('Movie Reviews', 'Movies'),
('Oscars', 'Movies'),
('Sundance', 'Movies'),
('Hip Hop', 'Music'),
('Indie', 'Music'),
('Metal', 'Music'),
('Pop', 'Music'),
('Rap', 'Music'),
('Rock', 'Music'),
('Data Journalism', 'News'),
('Fake News', 'News'),
('Journalism', 'News'),
('Misinformation', 'News'),
('True Crime', 'News'),
('Cameras', 'Photography'),
('Photography Tips', 'Photography'),
('Photojournalism', 'Photography'),
('Photos', 'Photography'),
('Street Photography', 'Photography'),
('Podcast Equipment', 'Podcasts'),
('Podcast Recommendations', 'Podcasts'),
('Podcasting', 'Podcasts'),
('Podcasting Tips', 'Podcasts'),
('Radio', 'Podcasts'),
('Hbo Max', 'Television'),
('Hulu', 'Television'),
('Netflix', 'Television'),
('Reality TV', 'Television'),
('Tv Reviews', 'Television'),
('Basic Income', 'Economics'),
('Debt', 'Economics'),
('Economy', 'Economics'),
('Inflation', 'Economics'),
('Stock Market', 'Economics'),
('Charter Schools', 'Education'),
('Education Reform', 'Education'),
('Higher Education', 'Education'),
('PhD', 'Education'),
('Public Schools', 'Education'),
('Student Loans', 'Education'),
('Study Abroad', 'Education'),
('Teaching', 'Education'),
('Disability', 'Equality'),
('Discrimination', 'Equality'),
('Diversity In Tech', 'Equality'),
('Feminism', 'Equality'),
('Inclusion', 'Equality'),
('LGBTQ', 'Equality'),
('Racism', 'Equality'),
('Transgender', 'Equality'),
('Womens Rights', 'Equality'),
('401k', 'Finance'),
('Investing', 'Finance'),
('Money', 'Finance'),
('Philanthropy', 'Finance'),
('Real Estate', 'Finance'),
('Retirement', 'Finance'),
('Criminal Justice', 'Law'),
('Law School', 'Law'),
('Legaltech', 'Law'),
('Social Justice', 'Law'),
('Supreme Court', 'Law'),
('Logistics', 'Transportation'),
('Public Transit', 'Transportation'),
('Self Driving Cars', 'Transportation'),
('Trucking', 'Transportation'),
('Urban Planning', 'Transportation'),
('Elections', 'Politics'),
('Government', 'Politics'),
('Gun Control', 'Politics'),
('Immigration', 'Politics'),
('Political Parties', 'Politics'),
('American Indian', 'Races'),
('Anti Racism', 'Races'),
('Asian American', 'Races'),
('Black Lives Matter', 'Races'),
('Indigenous People', 'Races'),
('Multiracial', 'Races'),
('Pacific Islander', 'Races'),
('White Privilege', 'Races'),
('White Supremacy', 'Races'),
('Archaeology', 'Science'),
('Astronomy', 'Science'),
('Astrophysics', 'Science'),
('Biotechnology', 'Science'),
('Chemistry', 'Science'),
('Ecology', 'Science'),
('Genetics', 'Science'),
('Geology', 'Science'),
('Medicine', 'Science'),
('Neuroscience', 'Science'),
('Physics', 'Science'),
('Psychology', 'Science'),
('Space', 'Science'),
('Algebra', 'Mathematics'),
('Calculus', 'Mathematics'),
('Geometry', 'Mathematics'),
('Probability', 'Mathematics'),
('Statistics', 'Mathematics'),
('Addiction', 'Drugs'),
('Cannabis', 'Drugs'),
('Opioids', 'Drugs'),
('Pharmaceuticals', 'Drugs'),
('Psychedelics', 'Drugs'),
('Atheism', 'Philosophy'),
('Epistemology', 'Philosophy'),
('Ethics', 'Philosophy'),
('Existentialism', 'Philosophy'),
('Metaphysics', 'Philosophy'),
('Morality', 'Philosophy'),
('Philosophy Of Mind', 'Philosophy'),
('Stoicism', 'Philosophy'),
('Buddhism', 'Religion'),
('Christianity', 'Religion'),
('Hinduism', 'Religion'),
('Judaism', 'Religion'),
('Zen', 'Religion'),
('Astrology', 'Spirituality'),
('Energy Healing', 'Spirituality'),
('Horoscopes', 'Spirituality'),
('Mysticism', 'Spirituality'),
('Reiki', 'Spirituality'),
('Ancient History', 'Cultural Studies'),
('Anthropology', 'Cultural Studies'),
('Cultural Heritage', 'Cultural Studies'),
('Digital Life', 'Cultural Studies'),
('History', 'Cultural Studies'),
('Museums', 'Cultural Studies'),
('Sociology', 'Cultural Studies'),
('Tradition', 'Cultural Studies'),
('Clothing', 'Fashion'),
('Fashion Design', 'Fashion'),
('Fashion Trends', 'Fashion'),
('Shoes', 'Fashion'),
('Sneakers', 'Fashion'),
('Style', 'Fashion'),
('Beauty Tips', 'Beauty'),
('Body Image', 'Beauty'),
('Hair', 'Beauty'),
('Makeup', 'Beauty'),
('Skincare', 'Beauty'),
('Arabic', 'Language'),
('English Language', 'Language'),
('English Learning', 'Language'),
('French', 'Language'),
('German', 'Language'),
('Hindi', 'Language'),
('Language Learning', 'Language'),
('Linguistics', 'Language'),
('Mandarin', 'Language'),
('Portuguese', 'Language'),
('Spanish', 'Language'),
('Baseball', 'Sports'),
('Basketball', 'Sports'),
('Football', 'Sports'),
('NBA', 'Sports'),
('NFL', 'Sports'),
('Premier League', 'Sports'),
('Soccer', 'Sports'),
('World Cup', 'Sports'),
('Abu Dhabi', 'Cities'),
('Amsterdam', 'Cities'),
('Athens', 'Cities'),
('Bangkok', 'Cities'),
('Barcelona', 'Cities'),
('Berlin', 'Cities'),
('Boston', 'Cities'),
('Buenos Aires', 'Cities'),
('Chicago', 'Cities'),
('Copenhagen', 'Cities'),
('Delhi', 'Cities'),
('Dubai', 'Cities'),
('Dublin', 'Cities'),
('Edinburgh', 'Cities'),
('Glasgow', 'Cities'),
('Hong Kong', 'Cities'),
('Istanbul', 'Cities'),
('Lisbon', 'Cities'),
('London', 'Cities'),
('Los Angeles', 'Cities'),
('Madrid', 'Cities'),
('Melbourne', 'Cities'),
('Mexico City', 'Cities'),
('Miami', 'Cities'),
('Montreal', 'Cities'),
('New York City', 'Cities'),
('Paris', 'Cities'),
('Prague', 'Cities'),
('Rio De Janeiro', 'Cities'),
('Rome', 'Cities'),
('San Francisco', 'Cities'),
('Sydney', 'Cities'),
('Taipei', 'Cities'),
('Tel Aviv', 'Cities'),
('Tokyo', 'Cities'),
('Toronto', 'Cities'),
('Vancouver', 'Cities'),
('Vienna', 'Cities'),
('Birding', 'Nature'),
('Camping', 'Nature'),
('Climate Change', 'Nature'),
('Conservation', 'Nature'),
('Hiking', 'Nature'),
('Sustainability', 'Nature'),
('Wildlife', 'Nature'),
('Tourism', 'Travel'),
('Travel Tips', 'Travel'),
('Travel Writing', 'Travel'),
('Vacation', 'Travel'),
('Vanlife', 'Travel'),
('Personal Development', 'Self-Improvement'),
('Nature', 'Outdoors');
