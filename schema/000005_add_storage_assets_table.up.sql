CREATE TABLE IF NOT EXISTS storage_assets (
    checksum      VARCHAR(64) PRIMARY KEY, -- SHA-256 fingerprint
    object_key    TEXT NOT NULL UNIQUE,    -- Canonical checksum-based MinIO key
    content_type  VARCHAR(100),
    size          BIGINT,
    width         INT,
    height        INT,
    blurhash      TEXT,
    is_nsfw       BOOLEAN DEFAULT FALSE,
    nsfw_score    FLOAT DEFAULT 0,
    created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_storage_assets_checksum_len CHECK (char_length(checksum) = 64),
    CONSTRAINT chk_storage_assets_size_nonnegative CHECK (size IS NULL OR size >= 0),
    CONSTRAINT chk_storage_assets_dimensions_nonnegative CHECK (
        (width IS NULL OR width >= 0)
        AND (height IS NULL OR height >= 0)
    ),
    CONSTRAINT chk_storage_assets_nsfw_score_range CHECK (
        nsfw_score IS NULL OR (nsfw_score >= 0 AND nsfw_score <= 1)
    )
);

CREATE TABLE IF NOT EXISTS storage_asset_refs (
    id            UUID PRIMARY KEY,
    checksum      VARCHAR(64) NOT NULL,
    owner_type    VARCHAR(32) NOT NULL,  -- blog, profile, draft, etc.
    owner_id      TEXT NOT NULL,         -- blog id, username, draft id, etc.
    purpose       VARCHAR(64) NOT NULL,  -- editor_image, profile_image, attachment, etc.
    file_name     TEXT,
    created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at    TIMESTAMP,
    FOREIGN KEY (checksum) REFERENCES storage_assets(checksum) ON DELETE RESTRICT,
    CONSTRAINT chk_storage_asset_refs_owner_type_nonempty CHECK (char_length(trim(owner_type)) > 0),
    CONSTRAINT chk_storage_asset_refs_owner_id_nonempty CHECK (char_length(trim(owner_id)) > 0),
    CONSTRAINT chk_storage_asset_refs_purpose_nonempty CHECK (char_length(trim(purpose)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_storage_asset_refs_checksum
    ON storage_asset_refs(checksum);

CREATE INDEX IF NOT EXISTS idx_storage_asset_refs_active_checksum
    ON storage_asset_refs(checksum)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_storage_asset_refs_owner
    ON storage_asset_refs(owner_type, owner_id, deleted_at);

CREATE UNIQUE INDEX IF NOT EXISTS uq_storage_asset_refs_active_logical_file
    ON storage_asset_refs(owner_type, owner_id, purpose, COALESCE(file_name, ''))
    WHERE deleted_at IS NULL;
