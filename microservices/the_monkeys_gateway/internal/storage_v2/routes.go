package storage_v2

import (
	"bytes"
	"context"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bbrks/go-blurhash"
	"go.uber.org/zap"
	"golang.org/x/image/webp"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/constants"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/internal/auth"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/utils"
)

// Router groups and handlers for /api/v2/storage
// This is a thin HTTP layer; implementation wiring to MinIO and image processing will be added next.

type Service struct {
	mc           *minio.Client
	bucket       string
	cdnURL       string
	publicBase   string // optional public base (scheme+host) to generate presigned URLs for
	publicSigner *minio.Client
	log          *zap.SugaredLogger
}

func newService(cfg *config.Config, log *zap.SugaredLogger) (*Service, error) {
	cli, err := minio.New(cfg.Minio.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.Minio.AccessKey, cfg.Minio.SecretKey, ""),
		Secure: cfg.Minio.UseSSL,
	})
	if err != nil {
		return nil, err
	}

	svc := &Service{mc: cli, bucket: cfg.Minio.Bucket, cdnURL: cfg.Minio.CDNURL, log: log}
	// If a public base is provided (e.g., http://localhost:9000), create a signer bound to that host
	if v := strings.TrimSpace(cfg.Minio.PublicBaseURL); v != "" {
		svc.publicBase = strings.TrimRight(v, "/")
		if pu, err := url.Parse(svc.publicBase); err == nil {
			endpoint := pu.Host
			secure := pu.Scheme == "https"
			ps, psErr := minio.New(endpoint, &minio.Options{
				Creds:  credentials.NewStaticV4(cfg.Minio.AccessKey, cfg.Minio.SecretKey, ""),
				Secure: secure,
				Region: "us-east-1",
			})
			if psErr != nil {
				log.Warnf("failed to init public signer for presigned URLs: %v", psErr)
			} else {
				svc.publicSigner = ps
			}
		}
	}

	// Ensure bucket exists
	ctx := context.Background()
	exists, err := svc.mc.BucketExists(ctx, svc.bucket)
	if err != nil {
		return nil, err
	}
	if !exists {
		if err := svc.mc.MakeBucket(ctx, svc.bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, err
		}
		log.Infof("Created MinIO bucket: %s", svc.bucket)
	}

	return svc, nil
}

func RegisterRoutes(router *gin.Engine, cfg *config.Config, authClient *auth.ServiceClient, log *zap.SugaredLogger) *Service {
	mw := auth.InitAuthMiddleware(authClient, log)

	svc, err := newService(cfg, log)
	if err != nil {
		log.Fatalf("failed to initialize MinIO client: %v", err)
	}

	v2 := router.Group("/api/v2/storage")

	// Public reads (CDN-style). We may later gate or sign URLs as needed.
	{
		// Stream the user's profile image (public). Canonical key.
		v2.GET("/profiles/:user_id/profile", svc.GetProfileImage)
		// JSON metadata for profile image.
		v2.GET("/profiles/:user_id/profile/meta", svc.GetProfileMeta)
		// Presigned/CDN URL for profile image.
		v2.GET("/profiles/:user_id/profile/url", svc.GetProfileURL)
	}
	{
		// Stream a blog file (public). Uses object key posts/{id}/{fileName}.
		v2.GET("/posts/:id/:fileName", svc.GetPostFile)
		// Fast-load helpers (public): metadata + presigned/CDN URL
		// JSON with etag, size, contentType, lastModified, cacheControl, blurhash, width, height, url.
		v2.GET("/posts/:id/:fileName/meta", svc.GetPostFileMeta)
		// Returns presigned or CDN URL for direct delivery. Optional ?expires=seconds.
		v2.GET("/posts/:id/:fileName/url", svc.GetPostFileURL)
	}

	// Auth-required writes and deletes (and management)
	v2.Use(mw.AuthRequired)

	// Blog content CRUD
	{
		// Upload multipart form field `file`. Stores under posts/{id}/<uuid+ext>. Returns JSON with object info.
		v2.POST("/posts/:id", mw.AuthorizationByID, svc.UploadPostFile)
		// List all objects under posts/{id}/ (auth required).
		v2.GET("/posts/:id", svc.ListPostFiles)
		// Return metadata in headers (ETag, Last-Modified, Cache-Control, X-Blurhash, X-Image-Width, X-Image-Height).
		v2.HEAD("/posts/:id/:fileName", svc.HeadPostFile)
		// Replace an existing file with multipart field `file`. Updates metadata for images.
		v2.PUT("/posts/:id/:fileName", mw.AuthorizationByID, svc.UpdatePostFile)
		// Delete an object.
		v2.DELETE("/posts/:id/:fileName", mw.AuthorizationByID, svc.DeletePostFile)
	}

	// Profile image CRUD (single resource)
	{
		// Upload multipart field `profile_pic`. Canonical key per user.
		v2.POST("/profiles/:user_id/profile", svc.UploadProfileImage)
		// Metadata in headers (ETag, Last-Modified, Cache-Control, X-Blurhash, X-Image-Width, X-Image-Height).
		v2.HEAD("/profiles/:user_id/profile", svc.HeadProfileImage)
		// Replace profile image (multipart `profile_pic`).
		v2.PUT("/profiles/:user_id/profile", svc.UpdateProfileImage)
		// Delete profile image.
		v2.DELETE("/profiles/:user_id/profile", svc.DeleteProfileImage)

	}
	return svc
}

// uniqueName generates a UUID-based filename preserving the original extension.
func uniqueName(original string) string {
	ext := filepath.Ext(original)
	if ext == "" {
		return uuid.NewString()
	}
	return uuid.NewString() + ext
}

// metaValue fetches a header-like key from a case-insensitive map provided by MinIO/S3 user metadata.
func metaValue(m map[string]string, key string) string {
	for k, v := range m {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}

// computeImageMetadata decodes the image and computes:
// - BlurHash placeholder (used for LQIP rendering on clients)
// - Intrinsic width and height
// Returns ok=false if data is not a supported image.
func (s *Service) computeImageMetadata(contentType string, data []byte) (hash string, w, h int, ok bool) {
	var img image.Image
	if strings.Contains(contentType, "image/webp") {
		m, err := webp.Decode(bytes.NewReader(data))
		if err != nil {
			return "", 0, 0, false
		}
		img = m
	} else {
		m, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return "", 0, 0, false
		}
		img = m
	}
	hash, err := blurhash.Encode(4, 3, img)
	if err != nil {
		return "", 0, 0, false
	}
	b := img.Bounds()
	return hash, b.Dx(), b.Dy(), true
}

// presignedOrCDNURL returns a CDN URL if MINIO_CDN_URL is set; otherwise a presigned GET URL
// with the provided expiry. Note: we DO NOT rewrite the host of the presigned URL because
// AWS SigV4 includes the host in the signature. Rewriting would break the signature.
func (s *Service) presignedOrCDNURL(ctx context.Context, objectName string, expiry time.Duration) (string, error) {
	if s.cdnURL != "" {
		// Treat as public CDN origin; return deterministic URL
		return strings.TrimRight(s.cdnURL, "/") + "/" + objectName, nil
	}
	// Prefer a signer bound to a public base URL if provided, so the signature uses the public host
	if s.publicSigner != nil {
		u, err := s.publicSigner.PresignedGetObject(ctx, s.bucket, objectName, expiry, nil)
		if err == nil {
			return u.String(), nil
		}
		// fall back to default client on error
		s.log.Warnf("public presign failed, falling back to internal presign: %v", err)
	}
	u, err := s.mc.PresignedGetObject(ctx, s.bucket, objectName, expiry, nil)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// Handlers (Create/Read/Update/Delete)

// UploadPostFile
// Method: POST /api/v2/storage/posts/:id (auth required)
// Input: multipart form field `file`
// Behavior: stores under posts/{id}/<uuid+ext>, computes BlurHash/dimensions for images, sets Cache-Control immutable
// Response: 201 JSON { bucket, object, fileName, etag, size, contentType }
func (s *Service) UploadPostFile(ctx *gin.Context) {
	blogID := ctx.Param("id")

	if !utils.CheckUserAccessInContext(ctx, constants.PermissionEdit) {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "You are not allowed to perform this action"})
		return
	}

	file, fileHeader, err := ctx.Request.FormFile("file")
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "missing file"})
		return
	}
	defer file.Close()

	// Generate unique name within posts/{id}/ prefix (S3-style folder)
	fname := uniqueName(fileHeader.Filename)
	objectName := "posts/" + blogID + "/" + fname
	contentType := fileHeader.Header.Get("Content-Type")

	opts := minio.PutObjectOptions{
		ContentType:  contentType,
		CacheControl: "public, max-age=31536000, immutable",
	}

	// Read a small prefix for image metadata if it's an image
	// 5MB limit for metadata processing to keep memory low
	const metadataLimit = 5 * 1024 * 1024
	var finalReader io.Reader = file
	var objectSize int64 = fileHeader.Size

	if strings.HasPrefix(strings.ToLower(contentType), "image/") && fileHeader.Size <= metadataLimit {
		// Read into memory ONLY if small enough for metadata
		data, err := io.ReadAll(file)
		if err == nil {
			if hash, w, h, ok := s.computeImageMetadata(contentType, data); ok {
				opts.UserMetadata = map[string]string{
					"x-blurhash": hash,
					"x-width":    strconv.Itoa(w),
					"x-height":   strconv.Itoa(h),
				}
			}
			finalReader = bytes.NewReader(data)
			objectSize = int64(len(data))
		} else {
			// Fallback to streaming if read fails
			_, _ = file.Seek(0, io.SeekStart)
		}
	}

	info, err := s.mc.PutObject(ctx.Request.Context(), s.bucket, objectName, finalReader, objectSize, opts)
	if err != nil {
		s.log.Errorf("minio PutObject error: %v", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "upload failed"})
		return
	}

	// Respond with stored file info
	resp := gin.H{
		"bucket":      s.bucket,
		"object":      objectName,
		"fileName":    fname,
		"etag":        info.ETag,
		"size":        info.Size,
		"contentType": contentType,
	}
	ctx.JSON(http.StatusCreated, resp)
}

// ListPostFiles
// Method: GET /api/v2/storage/posts/:id (auth required)
// Behavior: lists objects under posts/{id}/
// Response: 200 JSON { files: [{ object, fileName, size, etag, lastModified }] }
func (s *Service) ListPostFiles(ctx *gin.Context) {
	blogID := ctx.Param("id")
	prefix := "posts/" + blogID + "/"

	ch := s.mc.ListObjects(ctx.Request.Context(), s.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true})
	files := make([]gin.H, 0, 8)
	for obj := range ch {
		if obj.Err != nil {
			s.log.Errorf("list object error: %v", obj.Err)
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "list failed"})
			return
		}
		// Skip the folder key itself if present
		if obj.Key == prefix {
			continue
		}
		files = append(files, gin.H{
			"object":       obj.Key,
			"fileName":     path.Base(obj.Key),
			"size":         obj.Size,
			"etag":         obj.ETag,
			"lastModified": obj.LastModified,
		})
	}

	ctx.JSON(http.StatusOK, gin.H{"files": files})
}

// HeadPostFile
// Method: HEAD /api/v2/storage/posts/:id/:fileName (auth required)
// Behavior: returns object metadata in headers: Content-Type, ETag, Last-Modified, Cache-Control
//
//	and if image: X-Blurhash, X-Image-Width, X-Image-Height
func (s *Service) HeadPostFile(ctx *gin.Context) {
	blogID := ctx.Param("id")
	fileName := ctx.Param("fileName")
	objectName := "posts/" + blogID + "/" + fileName

	info, err := s.mc.StatObject(ctx.Request.Context(), s.bucket, objectName, minio.StatObjectOptions{})
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "file not found"})
		return
	}

	if info.ContentType != "" {
		ctx.Header("Content-Type", info.ContentType)
	}
	if info.ETag != "" {
		ctx.Header("ETag", info.ETag)
	}
	if !info.LastModified.IsZero() {
		ctx.Header("Last-Modified", info.LastModified.UTC().Format(http.TimeFormat))
	}
	if cc := info.Metadata.Get("Cache-Control"); cc != "" {
		ctx.Header("Cache-Control", cc)
	}
	if info.UserMetadata != nil {
		if bh := metaValue(info.UserMetadata, "x-blurhash"); bh != "" {
			ctx.Header("X-Blurhash", bh)
		}
		if w := metaValue(info.UserMetadata, "x-width"); w != "" {
			ctx.Header("X-Image-Width", w)
		}
		if h := metaValue(info.UserMetadata, "x-height"); h != "" {
			ctx.Header("X-Image-Height", h)
		}
	}

	ctx.Status(http.StatusOK)
}

// UpdatePostFile
// Method: PUT /api/v2/storage/posts/:id/:fileName (auth required)
// Input: multipart form field `file`
// Behavior: replaces object and recomputes image metadata
// Response: 200 JSON { bucket, object, etag, size, contentType }
func (s *Service) UpdatePostFile(ctx *gin.Context) {
	blogID := ctx.Param("id")

	if !utils.CheckUserAccessInContext(ctx, constants.PermissionEdit) {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "You are not allowed to perform this action"})
		return
	}

	fileName := ctx.Param("fileName")
	objectName := "posts/" + blogID + "/" + fileName

	file, fileHeader, err := ctx.Request.FormFile("file")
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "missing file"})
		return
	}
	defer file.Close()

	contentType := fileHeader.Header.Get("Content-Type")

	data, err := io.ReadAll(file)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "read failed"})
		return
	}
	reader := bytes.NewReader(data)

	opts := minio.PutObjectOptions{
		ContentType:  contentType,
		CacheControl: "public, max-age=31536000, immutable",
	}
	if strings.HasPrefix(strings.ToLower(contentType), "image/") {
		if hash, w, h, ok := s.computeImageMetadata(contentType, data); ok {
			opts.UserMetadata = map[string]string{
				"x-blurhash": hash,
				"x-width":    strconv.Itoa(w),
				"x-height":   strconv.Itoa(h),
			}
		}
	}

	info, err := s.mc.PutObject(ctx.Request.Context(), s.bucket, objectName, reader, int64(len(data)), opts)
	if err != nil {
		s.log.Errorf("minio PutObject (update) error: %v", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "update failed"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"bucket":      s.bucket,
		"object":      objectName,
		"etag":        info.ETag,
		"size":        info.Size,
		"contentType": contentType,
	})
}

// GetPostFile
// Method: GET /api/v2/storage/posts/:id/:fileName (public)
// Behavior: streams the object, sets Content-Type/ETag/Last-Modified/Cache-Control
//
//	and if image: X-Blurhash, X-Image-Width, X-Image-Height
func (s *Service) GetPostFile(ctx *gin.Context) {
	blogID := ctx.Param("id")
	fileName := ctx.Param("fileName")
	objectName := "posts/" + blogID + "/" + fileName

	obj, err := s.mc.GetObject(ctx.Request.Context(), s.bucket, objectName, minio.GetObjectOptions{})
	if err != nil {
		s.log.Errorf("minio GetObject error: %v", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "read failed"})
		return
	}
	defer obj.Close()

	stat, err := obj.Stat()
	if err != nil {
		// Not found or access error
		ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "file not found"})
		return
	}

	if stat.ContentType != "" {
		ctx.Header("Content-Type", stat.ContentType)
	}
	if stat.ETag != "" {
		ctx.Header("ETag", stat.ETag)
	}
	if !stat.LastModified.IsZero() {
		ctx.Header("Last-Modified", stat.LastModified.UTC().Format(http.TimeFormat))
	}
	if cc := stat.Metadata.Get("Cache-Control"); cc != "" {
		ctx.Header("Cache-Control", cc)
	}
	if stat.UserMetadata != nil {
		if bh := metaValue(stat.UserMetadata, "x-blurhash"); bh != "" {
			ctx.Header("X-Blurhash", bh)
		}
		if w := metaValue(stat.UserMetadata, "x-width"); w != "" {
			ctx.Header("X-Image-Width", w)
		}
		if h := metaValue(stat.UserMetadata, "x-height"); h != "" {
			ctx.Header("X-Image-Height", h)
		}
	}

	// For PDFs, ensure inline display in iframes
	if strings.Contains(strings.ToLower(stat.ContentType), "application/pdf") {
		ctx.Header("Content-Disposition", "inline")
	}

	// Stream body
	if _, err := io.Copy(ctx.Writer, obj); err != nil {
		s.log.Errorf("stream write error: %v", err)
	}
}

// DeletePostFile
// Method: DELETE /api/v2/storage/posts/:id/:fileName (auth required)
// Behavior: deletes the object if it exists
// Response: 200 JSON { message: "deleted", object }
func (s *Service) DeletePostFile(ctx *gin.Context) {
	blogID := ctx.Param("id")

	if !utils.CheckUserAccessInContext(ctx, constants.PermissionEdit) {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "You are not allowed to perform this action"})
		return
	}

	fileName := ctx.Param("fileName")
	objectName := "posts/" + blogID + "/" + fileName

	// Optionally check existence
	_, statErr := s.mc.StatObject(ctx.Request.Context(), s.bucket, objectName, minio.StatObjectOptions{})
	if statErr != nil {
		ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "file not found"})
		return
	}

	err := s.mc.RemoveObject(ctx.Request.Context(), s.bucket, objectName, minio.RemoveObjectOptions{})
	if err != nil {
		s.log.Errorf("minio RemoveObject error: %v", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "delete failed"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "deleted", "object": objectName})
}

// UploadProfileImage
// Method: POST /api/v2/storage/profiles/:user_id/profile (auth required)
// Input: multipart form field `profile_pic`
// Behavior: stores to canonical key profiles/{user_id}/profile, computes BlurHash/dimensions for images
// Response: 201 JSON { bucket, object, etag, size, contentType }
func (s *Service) UploadProfileImage(ctx *gin.Context) {
	userID := ctx.Param("user_id")
	loggedInUser := ctx.GetString("userName")

	if userID != loggedInUser {
		ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"message": "cannot upload profile for another user"})
		return
	}

	file, fileHeader, err := ctx.Request.FormFile("profile_pic")
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "missing profile_pic"})
		return
	}
	defer file.Close()

	objectName := "profiles/" + userID + "/profile" // single canonical key for profile image
	contentType := fileHeader.Header.Get("Content-Type")

	// Streaming upload
	const metadataLimit = 5 * 1024 * 1024
	var finalReader io.Reader = file
	var objectSize int64 = fileHeader.Size

	opts := minio.PutObjectOptions{
		ContentType:  contentType,
		CacheControl: "public, max-age=31536000, immutable",
	}

	if strings.HasPrefix(strings.ToLower(contentType), "image/") && fileHeader.Size <= metadataLimit {
		data, err := io.ReadAll(file)
		if err == nil {
			if hash, w, h, ok := s.computeImageMetadata(contentType, data); ok {
				opts.UserMetadata = map[string]string{
					"x-blurhash": hash,
					"x-width":    strconv.Itoa(w),
					"x-height":   strconv.Itoa(h),
				}
			}
			finalReader = bytes.NewReader(data)
			objectSize = int64(len(data))
		} else {
			_, _ = file.Seek(0, io.SeekStart)
		}
	}

	info, err := s.mc.PutObject(ctx.Request.Context(), s.bucket, objectName, finalReader, objectSize, opts)
	if err != nil {
		s.log.Errorf("minio PutObject error: %v", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "upload failed"})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"bucket":      s.bucket,
		"object":      objectName,
		"etag":        info.ETag,
		"size":        info.Size,
		"contentType": contentType,
	})
}

// HeadProfileImage
// Method: HEAD /api/v2/storage/profiles/:user_id/profile (auth required)
// Behavior: returns metadata headers; includes X-Blurhash and dimensions if image
func (s *Service) HeadProfileImage(ctx *gin.Context) {
	userID := ctx.Param("user_id")
	objectName := "profiles/" + userID + "/profile"

	info, err := s.mc.StatObject(ctx.Request.Context(), s.bucket, objectName, minio.StatObjectOptions{})
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "profile not found"})
		return
	}

	if info.ContentType != "" {
		ctx.Header("Content-Type", info.ContentType)
	}
	if info.ETag != "" {
		ctx.Header("ETag", info.ETag)
	}
	if !info.LastModified.IsZero() {
		ctx.Header("Last-Modified", info.LastModified.UTC().Format(http.TimeFormat))
	}
	if cc := info.Metadata.Get("Cache-Control"); cc != "" {
		ctx.Header("Cache-Control", cc)
	}
	if info.UserMetadata != nil {
		if bh := metaValue(info.UserMetadata, "x-blurhash"); bh != "" {
			ctx.Header("X-Blurhash", bh)
		}
		if w := metaValue(info.UserMetadata, "x-width"); w != "" {
			ctx.Header("X-Image-Width", w)
		}
		if h := metaValue(info.UserMetadata, "x-height"); h != "" {
			ctx.Header("X-Image-Height", h)
		}
	}

	ctx.Status(http.StatusOK)
}

// UpdateProfileImage
// Method: PUT /api/v2/storage/profiles/:user_id/profile (auth required)
// Input: multipart form field `profile_pic`
// Behavior: replaces profile image and recomputes image metadata
// Response: 200 JSON { bucket, object, etag, size, contentType }
func (s *Service) UpdateProfileImage(ctx *gin.Context) {
	userID := ctx.Param("user_id")
	loggedInUser := ctx.GetString("userName")

	if userID != loggedInUser {
		ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"message": "cannot update profile for another user"})
		return
	}

	file, fileHeader, err := ctx.Request.FormFile("profile_pic")
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "missing profile_pic"})
		return
	}
	defer file.Close()

	objectName := "profiles/" + userID + "/profile"
	contentType := fileHeader.Header.Get("Content-Type")

	data, err := io.ReadAll(file)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "read failed"})
		return
	}
	reader := bytes.NewReader(data)

	opts := minio.PutObjectOptions{
		ContentType:  contentType,
		CacheControl: "public, max-age=31536000, immutable",
	}
	if strings.HasPrefix(strings.ToLower(contentType), "image/") {
		if hash, w, h, ok := s.computeImageMetadata(contentType, data); ok {
			opts.UserMetadata = map[string]string{
				"x-blurhash": hash,
				"x-width":    strconv.Itoa(w),
				"x-height":   strconv.Itoa(h),
			}
		}
	}

	info, err := s.mc.PutObject(ctx.Request.Context(), s.bucket, objectName, reader, int64(len(data)), opts)
	if err != nil {
		s.log.Errorf("minio PutObject (update) error: %v", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "update failed"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"bucket":      s.bucket,
		"object":      objectName,
		"etag":        info.ETag,
		"size":        info.Size,
		"contentType": contentType,
	})
}

// GetPostFileMeta
// Method: GET /api/v2/storage/posts/:id/:fileName/meta (public)
// Behavior: returns JSON metadata for the object including BlurHash, dimensions, and a direct URL (CDN or presigned)
// Response: 200 JSON FileMetaResponse
func (s *Service) GetPostFileMeta(ctx *gin.Context) {
	blogID := ctx.Param("id")
	fileName := ctx.Param("fileName")
	objectName := "posts/" + blogID + "/" + fileName

	info, err := s.mc.StatObject(ctx.Request.Context(), s.bucket, objectName, minio.StatObjectOptions{})
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "file not found"})
		return
	}

	bh := ""
	w := 0
	h := 0
	if info.UserMetadata != nil {
		bh = metaValue(info.UserMetadata, "x-blurhash")
		if ws := metaValue(info.UserMetadata, "x-width"); ws != "" {
			if vi, err := strconv.Atoi(ws); err == nil {
				w = vi
			}
		}
		if hs := metaValue(info.UserMetadata, "x-height"); hs != "" {
			if vi, err := strconv.Atoi(hs); err == nil {
				h = vi
			}
		}
	}

	urlStr, _ := s.presignedOrCDNURL(ctx.Request.Context(), objectName, 10*time.Minute)

	ctx.JSON(http.StatusOK, gin.H{
		"object":       objectName,
		"etag":         info.ETag,
		"size":         info.Size,
		"contentType":  info.ContentType,
		"lastModified": info.LastModified,
		"cacheControl": info.Metadata.Get("Cache-Control"),
		"blurhash":     bh,
		"width":        w,
		"height":       h,
		"url":          urlStr,
	})
}

// GetProfileMeta
// Method: GET /api/v2/storage/profiles/:user_id/profile/meta (public)
// Behavior: JSON metadata for profile image including BlurHash, dimensions, and direct URL
func (s *Service) GetProfileMeta(ctx *gin.Context) {
	userID := ctx.Param("user_id")
	objectName := "profiles/" + userID + "/profile"

	info, err := s.mc.StatObject(ctx.Request.Context(), s.bucket, objectName, minio.StatObjectOptions{})
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "profile not found"})
		return
	}

	bh := ""
	w := 0
	h := 0
	if info.UserMetadata != nil {
		bh = metaValue(info.UserMetadata, "x-blurhash")
		if ws := metaValue(info.UserMetadata, "x-width"); ws != "" {
			if vi, err := strconv.Atoi(ws); err == nil {
				w = vi
			}
		}
		if hs := metaValue(info.UserMetadata, "x-height"); hs != "" {
			if vi, err := strconv.Atoi(hs); err == nil {
				h = vi
			}
		}
	}

	urlStr, _ := s.presignedOrCDNURL(ctx.Request.Context(), objectName, 10*time.Minute)

	ctx.JSON(http.StatusOK, gin.H{
		"object":       objectName,
		"etag":         info.ETag,
		"size":         info.Size,
		"contentType":  info.ContentType,
		"lastModified": info.LastModified,
		"cacheControl": info.Metadata.Get("Cache-Control"),
		"blurhash":     bh,
		"width":        w,
		"height":       h,
		"url":          urlStr,
	})
}

// GetPostFileURL
// Method: GET /api/v2/storage/posts/:id/:fileName/url (public)
// Query: expires (seconds, default 600, max 604800)
// Behavior: returns a direct URL to the object (CDN if configured, otherwise presigned S3 URL)
func (s *Service) GetPostFileURL(ctx *gin.Context) {
	blogID := ctx.Param("id")
	fileName := ctx.Param("fileName")
	objectName := "posts/" + blogID + "/" + fileName

	expires := 600
	if qs := ctx.Query("expires"); qs != "" {
		if vi, err := strconv.Atoi(qs); err == nil {
			if vi > 0 {
				// cap at 7 days
				if vi > int((7*24*time.Hour)/time.Second) {
					vi = int((7 * 24 * time.Hour) / time.Second)
				}
				expires = vi
			}
		}
	}

	urlStr, err := s.presignedOrCDNURL(ctx.Request.Context(), objectName, time.Duration(expires)*time.Second)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "could not generate url"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"url": urlStr, "expiresIn": expires})
}

// GetProfileURL
// Method: GET /api/v2/storage/profiles/:user_id/profile/url (public)
// Query: expires (seconds)
// Behavior: returns CDN/presigned URL for profile image
func (s *Service) GetProfileURL(ctx *gin.Context) {
	userID := ctx.Param("user_id")
	objectName := "profiles/" + userID + "/profile"

	expires := 600
	if qs := ctx.Query("expires"); qs != "" {
		if vi, err := strconv.Atoi(qs); err == nil {
			if vi > 0 {
				if vi > int((7*24*time.Hour)/time.Second) {
					vi = int((7 * 24 * time.Hour) / time.Second)
				}
				expires = vi
			}
		}
	}

	urlStr, err := s.presignedOrCDNURL(ctx.Request.Context(), objectName, time.Duration(expires)*time.Second)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "could not generate url"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"url": urlStr, "expiresIn": expires})
}

func (s *Service) GetProfileImage(ctx *gin.Context) {
	// Streams the user's profile image (public)
	userID := ctx.Param("user_id")
	objectName := "profiles/" + userID + "/profile"

	obj, err := s.mc.GetObject(ctx.Request.Context(), s.bucket, objectName, minio.GetObjectOptions{})
	if err != nil {
		s.log.Errorf("minio GetObject (profile) error: %v", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "read failed"})
		return
	}
	defer obj.Close()

	stat, err := obj.Stat()
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "profile not found"})
		return
	}

	if stat.ContentType != "" {
		ctx.Header("Content-Type", stat.ContentType)
	}
	if stat.ETag != "" {
		ctx.Header("ETag", stat.ETag)
	}
	if !stat.LastModified.IsZero() {
		ctx.Header("Last-Modified", stat.LastModified.UTC().Format(http.TimeFormat))
	}
	if cc := stat.Metadata.Get("Cache-Control"); cc != "" {
		ctx.Header("Cache-Control", cc)
	}
	if stat.UserMetadata != nil {
		if bh := metaValue(stat.UserMetadata, "x-blurhash"); bh != "" {
			ctx.Header("X-Blurhash", bh)
		}
		if w := metaValue(stat.UserMetadata, "x-width"); w != "" {
			ctx.Header("X-Image-Width", w)
		}
		if h := metaValue(stat.UserMetadata, "x-height"); h != "" {
			ctx.Header("X-Image-Height", h)
		}
	}

	if _, err := io.Copy(ctx.Writer, obj); err != nil {
		s.log.Errorf("stream write error (profile): %v", err)
	}
}

func (s *Service) DeleteProfileImage(ctx *gin.Context) {
	// Deletes the user's profile image (auth required)
	userID := ctx.Param("user_id")
	loggedInUser := ctx.GetString("userName")

	if userID != loggedInUser {
		ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"message": "cannot delete profile for another user"})
		return
	}

	objectName := "profiles/" + userID + "/profile"

	// Check existence first for clearer 404
	_, statErr := s.mc.StatObject(ctx.Request.Context(), s.bucket, objectName, minio.StatObjectOptions{})
	if statErr != nil {
		ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "profile not found"})
		return
	}

	if err := s.mc.RemoveObject(ctx.Request.Context(), s.bucket, objectName, minio.RemoveObjectOptions{}); err != nil {
		s.log.Errorf("minio RemoveObject (profile) error: %v", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "delete failed"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "deleted", "object": objectName})
}
