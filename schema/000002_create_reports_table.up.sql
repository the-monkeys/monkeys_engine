CREATE TABLE IF NOT EXISTS reports (
	report_id SERIAL PRIMARY KEY,
	reason_type VARCHAR(255) NOT NULL,
	flag_count INTEGER NOT NULL DEFAULT 0,
	reporter_id INTEGER NOT NULL,
	reported_blog_id INTEGER,
	reported_user_id INTEGER,
	status VARCHAR(255) DEFAULT 'PENDING',
	reporter_notes TEXT,
	moderator_id INTEGER,
	moderator_notes TEXT,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

	FOREIGN KEY (reporter_id) REFERENCES user_account(id) ON DELETE CASCADE,
	FOREIGN KEY (reported_user_id) REFERENCES user_account(id) ON DELETE CASCADE,
	FOREIGN KEY (reported_blog_id) REFERENCES blog(id) ON DELETE CASCADE,
	FOREIGN KEY (moderator_id) REFERENCES user_account(id) ON DELETE CASCADE,
)

CREATE TABLE IF NOT EXISTS report_flags (
	report_id INTEGER NOT NULL,
	user_id INTEGER NOT NULL,
	flagged_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

	PRIMARY KEY (report_id, user_id),

	FOREIGN KEY (report_id) REFERENCES reports(id) ON DELETE CASCADE,
	FOREIGN KEY (user_id) REFERENCES user_account(id) ON DELETE CASCADE,
)
