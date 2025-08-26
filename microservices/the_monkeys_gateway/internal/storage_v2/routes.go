package storage_v2

import (
	"bytes"
	"context"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bbrks/go-blurhash"
	"golang.org/x/image/webp"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/internal/auth"
)

// Router groups and handlers for /api/v2/storage
// This is a thin HTTP layer; implementation wiring to MinIO and image processing will be added next.

type Service struct {
	mc     *minio.Client
	bucket string
	cdnURL string
}

func newService(cfg *config.Config) (*Service, error) {
	cli, err := minio.New(cfg.Minio.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.Minio.AccessKey, cfg.Minio.SecretKey, ""),
		Secure: cfg.Minio.UseSSL,
	})
	if err != nil {
		return nil, err
	}

	svc := &Service{mc: cli, bucket: cfg.Minio.Bucket, cdnURL: cfg.Minio.CDNURL}

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
		logrus.Infof("Created MinIO bucket: %s", svc.bucket)
	}

	return svc, nil
}

func RegisterRoutes(router *gin.Engine, cfg *config.Config, authClient *auth.ServiceClient) *Service {
	mw := auth.InitAuthMiddleware(authClient)

	svc, err := newService(cfg)
	if err != nil {
		logrus.Fatalf("failed to initialize MinIO client: %v", err)
	}

	v2 := router.Group("/api/v2/storage")

	// Public reads (CDN-style). We may later gate or sign URLs as needed.
	v2.GET("/posts/:id/:fileName", svc.GetPostFile)
	v2.GET("/profiles/:user_id/profile", svc.GetProfileImage)
	// Fast-load helpers (public): metadata + presigned/CDN URL
	v2.GET("/posts/:id/:fileName/meta", svc.GetPostFileMeta)
	v2.GET("/profiles/:user_id/profile/meta", svc.GetProfileMeta)
	v2.GET("/posts/:id/:fileName/url", svc.GetPostFileURL)
	v2.GET("/profiles/:user_id/profile/url", svc.GetProfileURL)

	// Auth-required writes and deletes (and management)
	v2.Use(mw.AuthRequired)
	// Blog content CRUD
	v2.POST("/posts/:id", svc.UploadPostFile)
	v2.GET("/posts/:id", svc.ListPostFiles)
	v2.HEAD("/posts/:id/:fileName", svc.HeadPostFile)
	v2.PUT("/posts/:id/:fileName", svc.UpdatePostFile)
	v2.DELETE("/posts/:id/:fileName", svc.DeletePostFile)

	// Profile image CRUD (single resource)
	v2.POST("/profiles/:user_id/profile", svc.UploadProfileImage)
	v2.HEAD("/profiles/:user_id/profile", svc.HeadProfileImage)
	v2.PUT("/profiles/:user_id/profile", svc.UpdateProfileImage)
	v2.DELETE("/profiles/:user_id/profile", svc.DeleteProfileImage)

	return svc
}

// Helpers
func uniqueName(original string) string {
	ext := filepath.Ext(original)
	if ext == "" {
		return uuid.NewString()
	}
	return uuid.NewString() + ext
}

// Additional helpers for image optimization and URLs
func metaValue(m map[string]string, key string) string {
	for k, v := range m {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}

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

func (s *Service) presignedOrCDNURL(ctx context.Context, objectName string, expiry time.Duration) (string, error) {
	if s.cdnURL != "" {
		// Treat as public CDN origin; return deterministic URL
		return strings.TrimRight(s.cdnURL, "/") + "/" + objectName, nil
	}
	u, err := s.mc.PresignedGetObject(ctx, s.bucket, objectName, expiry, nil)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// Handlers (Create/Read/Update/Delete)

// Blog content
func (s *Service) UploadPostFile(ctx *gin.Context) {
	blogID := ctx.Param("id")

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

	// Read into memory once so we can compute image metadata (BlurHash) and upload
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
		logrus.Errorf("minio PutObject error: %v", err)
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

func (s *Service) ListPostFiles(ctx *gin.Context) {
	blogID := ctx.Param("id")
	prefix := "posts/" + blogID + "/"

	ch := s.mc.ListObjects(ctx.Request.Context(), s.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true})
	files := make([]gin.H, 0, 8)
	for obj := range ch {
		if obj.Err != nil {
			logrus.Errorf("list object error: %v", obj.Err)
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

func (s *Service) UpdatePostFile(ctx *gin.Context) {
	blogID := ctx.Param("id")
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
		logrus.Errorf("minio PutObject (update) error: %v", err)
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

func (s *Service) GetPostFile(ctx *gin.Context) {
	blogID := ctx.Param("id")
	fileName := ctx.Param("fileName")
	objectName := "posts/" + blogID + "/" + fileName

	obj, err := s.mc.GetObject(ctx.Request.Context(), s.bucket, objectName, minio.GetObjectOptions{})
	if err != nil {
		logrus.Errorf("minio GetObject error: %v", err)
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

	// Stream body
	if _, err := io.Copy(ctx.Writer, obj); err != nil {
		logrus.Errorf("stream write error: %v", err)
	}
}

func (s *Service) DeletePostFile(ctx *gin.Context) {
	blogID := ctx.Param("id")
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
		logrus.Errorf("minio RemoveObject error: %v", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "delete failed"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "deleted", "object": objectName})
}

// Profile image (single resource)
func (s *Service) UploadProfileImage(ctx *gin.Context) {
	userID := ctx.Param("user_id")

	file, fileHeader, err := ctx.Request.FormFile("profile_pic")
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "missing profile_pic"})
		return
	}
	defer file.Close()

	objectName := "profiles/" + userID + "/profile" // single canonical key for profile image
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
		logrus.Errorf("minio PutObject error: %v", err)
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

func (s *Service) UpdateProfileImage(ctx *gin.Context) {
	userID := ctx.Param("user_id")

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
		logrus.Errorf("minio PutObject (update) error: %v", err)
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

func (s *Service) GetProfileImage(ctx *gin.Context) {
	userID := ctx.Param("user_id")
	objectName := "profiles/" + userID + "/profile"

	obj, err := s.mc.GetObject(ctx.Request.Context(), s.bucket, objectName, minio.GetObjectOptions{})
	if err != nil {
		logrus.Errorf("minio GetObject error: %v", err)
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
		logrus.Errorf("stream write error: %v", err)
	}
}

func (s *Service) DeleteProfileImage(ctx *gin.Context) {
	userID := ctx.Param("user_id")
	objectName := "profiles/" + userID + "/profile"

	_, statErr := s.mc.StatObject(ctx.Request.Context(), s.bucket, objectName, minio.StatObjectOptions{})
	if statErr != nil {
		ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "profile not found"})
		return
	}

	if err := s.mc.RemoveObject(ctx.Request.Context(), s.bucket, objectName, minio.RemoveObjectOptions{}); err != nil {
		logrus.Errorf("minio RemoveObject error: %v", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "delete failed"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "deleted", "object": objectName})
}

// Fast-load metadata endpoints
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

// Presigned/CDN URL endpoints
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
