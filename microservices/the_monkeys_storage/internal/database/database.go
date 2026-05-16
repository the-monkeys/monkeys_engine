package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_file_service/pb"
	"github.com/the-monkeys/the_monkeys/config"
	"go.uber.org/zap"
)

type StorageDB interface {
	CheckAsset(ctx context.Context, checksum string) (*pb.CheckAssetRes, error)
	RegisterAsset(ctx context.Context, req *pb.RegisterAssetReq) (*pb.RegisterAssetRes, error)
	UpdateNSFW(ctx context.Context, checksum string, isNSFW bool, score float32) error
	CreateAssetRef(ctx context.Context, req *pb.CreateAssetRefReq) (*pb.CreateAssetRefRes, error)
	DeleteAssetRef(ctx context.Context, req *pb.DeleteAssetRefReq) (*pb.DeleteAssetRefRes, error)
	ReplaceAssetRef(ctx context.Context, req *pb.ReplaceAssetRefReq) (*pb.ReplaceAssetRefRes, error)
}

type storageDB struct {
	db  *sql.DB
	log *zap.SugaredLogger
}

func NewStorageDB(cfg *config.Config, log *zap.SugaredLogger) (StorageDB, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.Postgresql.PrimaryDB.DBUsername,
		cfg.Postgresql.PrimaryDB.DBPassword,
		cfg.Postgresql.PrimaryDB.DBHost,
		cfg.Postgresql.PrimaryDB.DBPort,
		cfg.Postgresql.PrimaryDB.DBName,
	)
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %v", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(3)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping postgres: %v", err)
	}

	return &storageDB{db: db, log: log}, nil
}

func (s *storageDB) CheckAsset(ctx context.Context, checksum string) (*pb.CheckAssetRes, error) {
	var objectKey string
	var width, height sql.NullInt32
	var blurhash sql.NullString

	err := s.db.QueryRowContext(ctx,
		"SELECT object_key, width, height, blurhash FROM storage_assets WHERE checksum = $1",
		strings.TrimSpace(checksum)).Scan(&objectKey, &width, &height, &blurhash)

	if err != nil {
		if err == sql.ErrNoRows {
			return &pb.CheckAssetRes{Exists: false}, nil
		}
		return nil, err
	}

	res := &pb.CheckAssetRes{
		Exists:    true,
		ObjectKey: objectKey,
	}
	if width.Valid {
		res.Width = width.Int32
	}
	if height.Valid {
		res.Height = height.Int32
	}
	if blurhash.Valid {
		res.Blurhash = blurhash.String
	}
	return res, nil
}

func (s *storageDB) RegisterAsset(ctx context.Context, req *pb.RegisterAssetReq) (*pb.RegisterAssetRes, error) {
	checksum := strings.TrimSpace(req.Checksum)
	objectKey := strings.TrimSpace(req.ObjectKey)
	if checksum == "" {
		return nil, fmt.Errorf("checksum is required")
	}
	if objectKey == "" {
		return nil, fmt.Errorf("object_key is required")
	}

	var canonicalChecksum, canonicalObjectKey string
	err := s.db.QueryRowContext(ctx,
		`WITH inserted AS (
			INSERT INTO storage_assets (checksum, object_key, content_type, size, width, height, blurhash)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (checksum) DO NOTHING
			RETURNING checksum, object_key
		)
		SELECT checksum, object_key FROM inserted
		UNION ALL
		SELECT checksum, object_key FROM storage_assets WHERE checksum = $1
		LIMIT 1`,
		checksum, objectKey, req.ContentType, nullableInt64(req.Size), nullableInt32(req.Width), nullableInt32(req.Height), nullableString(req.Blurhash),
	).Scan(&canonicalChecksum, &canonicalObjectKey)
	if err != nil {
		return nil, err
	}

	return &pb.RegisterAssetRes{
		Success:   true,
		Checksum:  canonicalChecksum,
		ObjectKey: canonicalObjectKey,
	}, nil
}

func (s *storageDB) UpdateNSFW(ctx context.Context, checksum string, isNSFW bool, score float32) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE storage_assets SET is_nsfw = $1, nsfw_score = $2, updated_at = CURRENT_TIMESTAMP WHERE checksum = $3",
		isNSFW, score, strings.TrimSpace(checksum))
	return err
}

func (s *storageDB) CreateAssetRef(ctx context.Context, req *pb.CreateAssetRefReq) (*pb.CreateAssetRefRes, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	res, err := createAssetRefTx(ctx, tx, req)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return res, nil
}

func (s *storageDB) DeleteAssetRef(ctx context.Context, req *pb.DeleteAssetRefReq) (*pb.DeleteAssetRefRes, error) {
	refID := strings.TrimSpace(req.RefId)
	ownerType := strings.TrimSpace(req.OwnerType)
	ownerID := strings.TrimSpace(req.OwnerId)
	purpose := strings.TrimSpace(req.Purpose)
	fileName := strings.TrimSpace(req.FileName)

	var result sql.Result
	var err error
	switch {
	case refID != "":
		result, err = s.db.ExecContext(ctx,
			`UPDATE storage_asset_refs
			 SET deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
			 WHERE id = $1 AND deleted_at IS NULL`,
			refID,
		)
	case ownerType != "" && ownerID != "" && purpose != "":
		result, err = s.db.ExecContext(ctx,
			`UPDATE storage_asset_refs
			 SET deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
			 WHERE owner_type = $1
			   AND owner_id = $2
			   AND purpose = $3
			   AND COALESCE(file_name, '') = $4
			   AND deleted_at IS NULL`,
			ownerType, ownerID, purpose, fileName,
		)
	case ownerType != "" && ownerID != "":
		result, err = s.db.ExecContext(ctx,
			`UPDATE storage_asset_refs
			 SET deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
			 WHERE owner_type = $1
			   AND owner_id = $2
			   AND deleted_at IS NULL`,
			ownerType, ownerID,
		)
	default:
		return nil, fmt.Errorf("ref_id or owner_type and owner_id are required")
	}
	if err != nil {
		return nil, err
	}

	count, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	return &pb.DeleteAssetRefRes{Success: true, DeletedCount: int32(count)}, nil
}

func (s *storageDB) ReplaceAssetRef(ctx context.Context, req *pb.ReplaceAssetRefReq) (*pb.ReplaceAssetRefRes, error) {
	checksum := strings.TrimSpace(req.Checksum)
	ownerType := strings.TrimSpace(req.OwnerType)
	ownerID := strings.TrimSpace(req.OwnerId)
	purpose := strings.TrimSpace(req.Purpose)
	fileName := strings.TrimSpace(req.FileName)
	if checksum == "" || ownerType == "" || ownerID == "" || purpose == "" {
		return nil, fmt.Errorf("checksum, owner_type, owner_id, and purpose are required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	objectKey, err := assetObjectKeyTx(ctx, tx, checksum)
	if err != nil {
		return nil, err
	}

	result, err := tx.ExecContext(ctx,
		`UPDATE storage_asset_refs
		 SET deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		 WHERE owner_type = $1
		   AND owner_id = $2
		   AND purpose = $3
		   AND COALESCE(file_name, '') = $4
		   AND deleted_at IS NULL`,
		ownerType, ownerID, purpose, fileName,
	)
	if err != nil {
		return nil, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}

	refID := uuid.NewString()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO storage_asset_refs (id, checksum, owner_type, owner_id, purpose, file_name)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		refID, checksum, ownerType, ownerID, purpose, fileName,
	); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &pb.ReplaceAssetRefRes{
		Success:      true,
		RefId:        refID,
		Checksum:     checksum,
		ObjectKey:    objectKey,
		DeletedCount: int32(count),
	}, nil
}

func createAssetRefTx(ctx context.Context, tx *sql.Tx, req *pb.CreateAssetRefReq) (*pb.CreateAssetRefRes, error) {
	checksum := strings.TrimSpace(req.Checksum)
	ownerType := strings.TrimSpace(req.OwnerType)
	ownerID := strings.TrimSpace(req.OwnerId)
	purpose := strings.TrimSpace(req.Purpose)
	fileName := strings.TrimSpace(req.FileName)
	if checksum == "" || ownerType == "" || ownerID == "" || purpose == "" {
		return nil, fmt.Errorf("checksum, owner_type, owner_id, and purpose are required")
	}

	objectKey, err := assetObjectKeyTx(ctx, tx, checksum)
	if err != nil {
		return nil, err
	}

	var existingID, existingChecksum string
	err = tx.QueryRowContext(ctx,
		`SELECT id, checksum
		 FROM storage_asset_refs
		 WHERE owner_type = $1
		   AND owner_id = $2
		   AND purpose = $3
		   AND COALESCE(file_name, '') = $4
		   AND deleted_at IS NULL`,
		ownerType, ownerID, purpose, fileName,
	).Scan(&existingID, &existingChecksum)
	if err == nil {
		if existingChecksum != checksum {
			return nil, fmt.Errorf("active asset reference already exists for owner")
		}
		return &pb.CreateAssetRefRes{
			Success:   true,
			RefId:     existingID,
			Checksum:  checksum,
			ObjectKey: objectKey,
		}, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	var restoredID string
	err = tx.QueryRowContext(ctx,
		`UPDATE storage_asset_refs
		 SET deleted_at = NULL, updated_at = CURRENT_TIMESTAMP
		 WHERE id = (
			 SELECT id
			 FROM storage_asset_refs
			 WHERE checksum = $1
			   AND owner_type = $2
			   AND owner_id = $3
			   AND purpose = $4
			   AND COALESCE(file_name, '') = $5
			   AND deleted_at IS NOT NULL
			 ORDER BY updated_at DESC
			 LIMIT 1
		 )
		 RETURNING id`,
		checksum, ownerType, ownerID, purpose, fileName,
	).Scan(&restoredID)
	if err == nil {
		return &pb.CreateAssetRefRes{
			Success:   true,
			RefId:     restoredID,
			Checksum:  checksum,
			ObjectKey: objectKey,
		}, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	refID := uuid.NewString()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO storage_asset_refs (id, checksum, owner_type, owner_id, purpose, file_name)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		refID, checksum, ownerType, ownerID, purpose, fileName,
	); err != nil {
		return nil, err
	}

	return &pb.CreateAssetRefRes{
		Success:   true,
		RefId:     refID,
		Checksum:  checksum,
		ObjectKey: objectKey,
	}, nil
}

func assetObjectKeyTx(ctx context.Context, tx *sql.Tx, checksum string) (string, error) {
	var objectKey string
	err := tx.QueryRowContext(ctx,
		"SELECT object_key FROM storage_assets WHERE checksum = $1",
		checksum,
	).Scan(&objectKey)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("asset not found for checksum %s", checksum)
	}
	return objectKey, err
}

func nullableString(v string) sql.NullString {
	v = strings.TrimSpace(v)
	return sql.NullString{String: v, Valid: v != ""}
}

func nullableInt32(v int32) sql.NullInt32 {
	return sql.NullInt32{Int32: v, Valid: v > 0}
}

func nullableInt64(v int64) sql.NullInt64 {
	return sql.NullInt64{Int64: v, Valid: v > 0}
}
