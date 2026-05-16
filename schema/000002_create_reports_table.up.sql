CREATE TABLE IF NOT EXISTS reports (
	report_id BIGSERIAL PRIMARY KEY,
	reason_type VARCHAR(255) NOT NULL,
	flag_count INTEGER NOT NULL DEFAULT 0,
	reporter_id INTEGER NOT NULL,
	reported_type VARCHAR(255),
	reported_blog_id BIGINT,
	reported_user_id BIGINT,
	status VARCHAR(255) DEFAULT 'PENDING',
	reporter_notes TEXT,
	moderator_id BIGINT,
	moderator_notes TEXT,
	verdict VARCHAR(255),
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

	FOREIGN KEY (reporter_id) REFERENCES user_account(id) ON DELETE CASCADE,
	FOREIGN KEY (reported_user_id) REFERENCES user_account(id) ON DELETE CASCADE,
	FOREIGN KEY (reported_blog_id) REFERENCES blog(id) ON DELETE CASCADE,
	FOREIGN KEY (moderator_id) REFERENCES user_account(id) ON DELETE CASCADE,
	
	CONSTRAINT reported_type_values CHECK (reported_type IN ('BLOG', 'USER')),
	CONSTRAINT report_reason_types CHECK (reason_type IN ('SPAM', 'ABUSE', 'NSFW', 'MISINFORMATION', 'OTHER')),
	CONSTRAINT report_status_types CHECK (status IN ('PENDING', 'IN_PROGRESS', 'RESOLVED')),
	CONSTRAINT verdict_types CHECK (verdict IN ('BANNED', 'DELETED', 'IGNORE')),
	CONSTRAINT exactly_one_fk CHECK ((reported_blog_id IS NOT NULL) OR (reported_user_id IS NOT NULL)),
	CONSTRAINT no_report_self CHECK (reporter_id != reported_user_id)
);

-- This index makes sure user can't create more more reports that are pending and in progress 
CREATE UNIQUE INDEX idx_active_user_reports
ON reports (reporter_id, reported_blog_id, reported_user_id)
WHERE status IN ('PENDING', 'IN_PROGRESS');

CREATE TABLE IF NOT EXISTS report_flags (
	report_id INTEGER NOT NULL,
	user_id INTEGER NOT NULL,
	flagged_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

	PRIMARY KEY (report_id, user_id),

	FOREIGN KEY (report_id) REFERENCES reports(report_id) ON DELETE CASCADE,
	FOREIGN KEY (user_id) REFERENCES user_account(id) ON DELETE CASCADE
);
