package consumer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/constants"
	"github.com/the-monkeys/the_monkeys/logger"
	"github.com/the-monkeys/the_monkeys/microservices/rabbitmq"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_storage/constant"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_storage/internal/models"
	"go.uber.org/zap"
)

func ConsumeFromQueue(mgr *rabbitmq.ConnManager, conf config.RabbitMQ, log *zap.SugaredLogger) {
	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.ZapSugar().Debug("Storage consumer: received termination signal, exiting")
		os.Exit(0)
	}()

	// Load config and init a single MinIO client for all handlers
	cfg, err := config.GetConfig()
	if err != nil {
		log.Errorf("Failed to load storage config: %v", err)
	}
	var mc *minio.Client
	if cfg != nil {
		mc, err = minio.New(cfg.Minio.Endpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(cfg.Minio.AccessKey, cfg.Minio.SecretKey, ""),
			Secure: cfg.Minio.UseSSL,
		})
		if err != nil {
			log.Errorf("Failed to initialize MinIO client: %v", err)
		}
	}

	go consumeQueue(mgr, conf.Queues[0], log, cfg, mc)
	go consumeQueue(mgr, conf.Queues[2], log, cfg, mc)

	if mc != nil && cfg != nil {
		if cfg.Minio.EnableFSToMinioSync {
			log.Info("FS→MinIO periodic sync: ENABLED")
			go startPeriodicSync(cfg, mc, log)
		} else {
			log.Info("FS→MinIO periodic sync: DISABLED (set MINIO_ENABLE_FS_TO_MINIO_SYNC=true to enable)")
		}
		if cfg.Minio.EnableMinioToFSSync {
			log.Info("MinIO→FS periodic sync: ENABLED")
			go startMinioToFileSystemSync(cfg, mc, log)
		} else {
			log.Info("MinIO→FS periodic sync: DISABLED (set MINIO_ENABLE_MINIO_TO_FS_SYNC=true to enable)")
		}
	}

	select {}
}

func consumeQueue(mgr *rabbitmq.ConnManager, queueName string, log *zap.SugaredLogger, cfg *config.Config, mc *minio.Client) {
	backoff := time.Second

	for {
		msgs, err := mgr.Channel().Consume(
			queueName,
			"",
			true,
			false,
			false,
			false,
			nil,
		)
		if err != nil {
			logger.ZapSugar().Errorf("Storage consumer: failed to register on queue '%s', reconnecting in %v: %v", queueName, backoff, err)
			time.Sleep(backoff)
			if backoff *= 2; backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			mgr.Reconnect()
			continue
		}

		backoff = time.Second
		logger.ZapSugar().Infof("Storage consumer: registered on queue '%s'", queueName)

		for d := range msgs {
			user := models.TheMonkeysMessage{}
			if err := json.Unmarshal(d.Body, &user); err != nil {
				logger.ZapSugar().Errorf("Failed to unmarshal user from RabbitMQ: %v", err)
				continue
			}
			log.Debugw("Storage consumer: message received",
				"queue", queueName,
				"action", user.Action,
				"username", user.Username,
				"account_id", user.AccountId,
				"blog_id", user.BlogId,
				"blog_ids_count", len(user.BlogIds),
			)
			handleUserAction(user, log, cfg, mc)
		}

		// Channel closed — reconnect
		logger.ZapSugar().Warn("Storage consumer: channel closed, reconnecting...")
		mgr.Reconnect()
	}
}

func handleUserAction(user models.TheMonkeysMessage, log *zap.SugaredLogger, cfg *config.Config, mc *minio.Client) {
	switch user.Action {
	case constants.USER_REGISTER:
		log.Debugw("Handling USER_REGISTER", "username", user.Username)

		// Fetch default profile image: try remote URL, fall back to bundled local copy
		imageBytes, err := readImageFromURL(constant.DefaultProfilePhoto)
		if err != nil {
			log.Warnw("Remote profile photo fetch failed, using bundled fallback",
				"username", user.Username, "remote_err", err)
			imageBytes, err = os.ReadFile(constant.DefaultProfilePhotoLocal)
			if err != nil {
				log.Errorw("Failed to read bundled fallback profile photo",
					"username", user.Username, "path", constant.DefaultProfilePhotoLocal, "err", err)
				break
			}
		}

		// Upload to MinIO
		if mc != nil && cfg != nil {
			objectKey := "profiles/" + user.Username + "/profile.svg"
			log.Debugw("Uploading default profile photo (MinIO)", "username", user.Username, "object_key", objectKey)
			reader := bytes.NewReader(imageBytes)
			_, putErr := mc.PutObject(context.Background(), cfg.Minio.Bucket, objectKey, reader, int64(len(imageBytes)), minio.PutObjectOptions{
				ContentType: "image/svg+xml",
			})
			if putErr != nil {
				log.Errorw("Failed to upload profile photo to MinIO", "username", user.Username, "err", putErr)
			} else {
				log.Debugw("Uploaded default profile photo (MinIO)", "username", user.Username, "object_key", objectKey)
			}
		}

		log.Debugw("USER_REGISTER done", "username", user.Username)

	case constants.USERNAME_UPDATE:
		log.Debugw("Handling USERNAME_UPDATE",
			"old_username", user.Username,
			"new_username", user.NewUsername,
		)

		if mc != nil && cfg != nil {
			log.Debugw("Renaming user profile folder (MinIO)",
				"from_prefix", "profiles/"+user.Username+"/",
				"to_prefix", "profiles/"+user.NewUsername+"/",
			)
			if err := UpdateMinioProfileFolder(context.Background(), mc, cfg.Minio.Bucket, user.Username, user.NewUsername); err != nil {
				log.Errorw("Failed to rename MinIO profile folder", "from", user.Username, "to", user.NewUsername, "err", err)
			} else {
				log.Debugw("Renamed user profile folder (MinIO)", "from", user.Username, "to", user.NewUsername)
			}
		}

		log.Debugw("USERNAME_UPDATE done", "old_username", user.Username, "new_username", user.NewUsername)

	case constants.USER_ACCOUNT_DELETE:
		log.Debugw("Handling USER_ACCOUNT_DELETE",
			"username", user.Username,
			"account_id", user.AccountId,
			"blog_ids_count", len(user.BlogIds),
			"blog_ids", user.BlogIds,
		)

		// 1. Delete profile folder from MinIO
		if mc != nil && cfg != nil {
			log.Debugw("Deleting user profile folder (MinIO)", "username", user.Username, "prefix", "profiles/"+user.Username+"/")
			if err := DeleteMinioProfileFolder(context.Background(), mc, cfg.Minio.Bucket, user.Username); err != nil {
				log.Errorw("Failed to delete MinIO profile folder", "username", user.Username, "err", err)
			} else {
				log.Debugw("Deleted user profile folder (MinIO)", "username", user.Username)
			}
		}

		// 2. Delete blog files for each owned blog from FS + MinIO
		for i, blogId := range user.BlogIds {
			log.Debugw("Deleting blog files", "blog_index", i+1, "total", len(user.BlogIds), "blog_id", blogId)

			// FS: blogs/{blogId}/
			if err := DeleteBlogFolder(blogId); err != nil {
				log.Errorw("Failed to delete blog folder (FS)", "blog_id", blogId, "err", err)
			} else {
				log.Debugw("Deleted blog folder (FS)", "blog_id", blogId)
			}

			// MinIO: posts/{blogId}/
			if mc != nil && cfg != nil {
				if err := DeleteMinioBlogFolder(context.Background(), mc, cfg.Minio.Bucket, blogId); err != nil {
					log.Errorw("Failed to delete blog folder (MinIO)", "blog_id", blogId, "err", err)
				} else {
					log.Debugw("Deleted blog folder (MinIO)", "blog_id", blogId, "prefix", "posts/"+blogId+"/")
				}
			}
		}

		log.Debugw("USER_ACCOUNT_DELETE done",
			"username", user.Username,
			"blogs_processed", len(user.BlogIds),
		)
	case constants.BLOG_DELETE:
		log.Debugw("Handling BLOG_DELETE", "blog_id", user.BlogId)

		log.Debugw("Deleting blog folder (FS)", "blog_id", user.BlogId)
		if err := DeleteBlogFolder(user.BlogId); err != nil {
			log.Errorw("Failed to delete blog folder (FS)", "blog_id", user.BlogId, "err", err)
		} else {
			log.Debugw("Deleted blog folder (FS)", "blog_id", user.BlogId)
		}

		if mc != nil && cfg != nil {
			log.Debugw("Deleting blog folder (MinIO)", "blog_id", user.BlogId, "prefix", "posts/"+user.BlogId+"/")
			if err := DeleteMinioBlogFolder(context.Background(), mc, cfg.Minio.Bucket, user.BlogId); err != nil {
				log.Errorw("Failed to delete MinIO blog folder", "blog_id", user.BlogId, "err", err)
			} else {
				log.Debugw("Deleted blog folder (MinIO)", "blog_id", user.BlogId)
			}
		}

		log.Debugw("BLOG_DELETE done", "blog_id", user.BlogId)
	default:
		log.Errorw("Unknown action", "action", user.Action)
	}
}

// func CreateUserFolder(userName string) error {
// 	dirPath, filePath := ConstructPath(constant.ProfileDir, userName, "profile.png")
// 	if err := os.MkdirAll(dirPath, 0755); err != nil {
// 		logger.ZapSugar().Errorf("Cannot create directory structure for user: %s, error: %v", userName, err)
// 		return err
// 	}
// 	imageByte, err := readImageFromURL(constant.DefaultProfilePhoto)
// 	if err != nil {
// 		logger.ZapSugar().Errorf("Error fetching image for user: %s, error: %v", userName, err)
// 		return fmt.Errorf("error fetching image: %v", err)
// 	}
// 	if err = os.WriteFile(filePath, imageByte, 0644); err != nil {
// 		logger.ZapSugar().Errorf("Cannot write profile image file for user: %s, error: %v", userName, err)
// 		return err
// 	}
// 	logger.ZapSugar().Debugf("Done uploading profile pic: %s", filePath)
// 	return nil
// }

func readImageFromURL(url string) ([]byte, error) {
	client := http.Client{}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Error closing response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status code %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	return data, nil
}

func ConstructPath(baseDir, userName, fileName string) (string, string) {
	dirPath := filepath.Join(baseDir, userName)
	filePath := filepath.Join(dirPath, fileName)
	return dirPath, filePath
}

// func UpdateUserFolder(currentName, newName string) error {
// 	currentPath := filepath.Join(constant.ProfileDir, currentName)
// 	newPath := filepath.Join(constant.ProfileDir, newName)

// 	log.Printf("updating user folder %s to %s", currentName, newName)

// 	from, err := os.Stat(currentPath)
// 	if err != nil {
// 		return errors.New("could not stat current directory: " + err.Error())
// 	}

// 	if !from.IsDir() {
// 		return errors.New(currentPath + " is not a directory")
// 	}

// 	to := currentPath + "_temp"

// 	err = os.Rename(currentPath, to)
// 	if err != nil {
// 		return errors.New("failed to rename directory: " + err.Error())
// 	}

// 	err = os.Rename(to, newPath)
// 	if err != nil {
// 		return errors.New("failed to rename directory to new name: " + err.Error())
// 	}

// 	return nil
// }

// UpdateMinioProfileFolder renames the object prefix in MinIO from
// profiles/{oldName}/ -> profiles/{newName}/ by copying each object then deleting the old one.
func UpdateMinioProfileFolder(ctx context.Context, mc *minio.Client, bucket, oldName, newName string) error {
	oldPrefix := "profiles/" + strings.Trim(oldName, "/") + "/"
	newPrefix := "profiles/" + strings.Trim(newName, "/") + "/"
	if oldPrefix == newPrefix {
		return nil
	}
	// List all objects under the old prefix
	for obj := range mc.ListObjects(ctx, bucket, minio.ListObjectsOptions{Prefix: oldPrefix, Recursive: true}) {
		if obj.Err != nil {
			return fmt.Errorf("list objects failed: %w", obj.Err)
		}
		srcKey := obj.Key
		dstKey := strings.Replace(srcKey, oldPrefix, newPrefix, 1)
		// Copy to destination key
		_, err := mc.CopyObject(ctx,
			minio.CopyDestOptions{Bucket: bucket, Object: dstKey},
			minio.CopySrcOptions{Bucket: bucket, Object: srcKey},
		)
		if err != nil {
			return fmt.Errorf("copy %s -> %s failed: %w", srcKey, dstKey, err)
		}
		// Remove the old object
		if err := mc.RemoveObject(ctx, bucket, srcKey, minio.RemoveObjectOptions{}); err != nil {
			return fmt.Errorf("remove old object %s failed: %w", srcKey, err)
		}
	}
	return nil
}

// DeleteMinioPrefix removes all objects under the given prefix.
func DeleteMinioPrefix(ctx context.Context, mc *minio.Client, bucket, prefix string) error {
	for obj := range mc.ListObjects(ctx, bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if obj.Err != nil {
			return fmt.Errorf("list objects failed: %w", obj.Err)
		}
		if err := mc.RemoveObject(ctx, bucket, obj.Key, minio.RemoveObjectOptions{}); err != nil {
			return fmt.Errorf("remove object %s failed: %w", obj.Key, err)
		}
	}
	return nil
}

func DeleteMinioProfileFolder(ctx context.Context, mc *minio.Client, bucket, username string) error {
	prefix := "profiles/" + strings.Trim(username, "/") + "/"
	return DeleteMinioPrefix(ctx, mc, bucket, prefix)
}

func DeleteMinioBlogFolder(ctx context.Context, mc *minio.Client, bucket, blogId string) error {
	prefix := "posts/" + strings.Trim(blogId, "/") + "/"
	return DeleteMinioPrefix(ctx, mc, bucket, prefix)
}

func DeleteUserFolder(userName string) error {
	dirPath := filepath.Join(constant.ProfileDir, userName)

	err := os.RemoveAll(dirPath)
	if err != nil {
		return errors.New("failed to remove directory: " + err.Error())
	}

	return nil
}

func DeleteBlogFolder(blogId string) error {
	dirPath := filepath.Join(constant.BlogDir, blogId)

	err := os.RemoveAll(dirPath)
	if err != nil {
		return errors.New("failed to remove directory: " + err.Error())
	}

	return nil
}

// startPeriodicSync runs filesystem to MinIO sync every 24 hours
func startPeriodicSync(cfg *config.Config, mc *minio.Client, log *zap.SugaredLogger) {
	ticker := time.NewTicker(3 * time.Hour)
	defer ticker.Stop()

	// Run initial sync after 1 minute
	time.Sleep(1 * time.Minute)
	syncFilesystemToMinio(cfg, mc, log)

	for range ticker.C {
		syncFilesystemToMinio(cfg, mc, log)
	}
}

// syncFilesystemToMinio syncs blog files and profile files from filesystem to MinIO if they don't exist
func syncFilesystemToMinio(cfg *config.Config, mc *minio.Client, log *zap.SugaredLogger) {
	ctx := context.Background()

	log.Debug("Starting periodic sync of filesystem files to MinIO")

	// Sync blog files
	syncBlogFiles(ctx, cfg, mc, log)

	// Sync profile files
	syncProfileFiles(ctx, cfg, mc, log)

	log.Debug("Completed periodic sync of filesystem files to MinIO")
}

// syncBlogFiles syncs blog files from blogs/ directory to posts/ prefix in MinIO
func syncBlogFiles(ctx context.Context, cfg *config.Config, mc *minio.Client, log *zap.SugaredLogger) {
	blogsDir := constant.BlogDir

	if _, err := os.Stat(blogsDir); os.IsNotExist(err) {
		log.Warn("Blogs directory does not exist, skipping blog sync")
		return
	}

	log.Debug("Syncing blog files...")

	err := filepath.Walk(blogsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Errorf("Error accessing path %s: %v", path, err)
			return nil // Continue with other files
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Extract blog ID and filename from path
		relPath, err := filepath.Rel(blogsDir, path)
		if err != nil {
			log.Errorf("Error getting relative path for %s: %v", path, err)
			return nil
		}

		pathParts := strings.Split(filepath.ToSlash(relPath), "/")
		if len(pathParts) < 2 {
			log.Warnf("Unexpected blog file structure: %s", relPath)
			return nil
		}

		blogID := pathParts[0]
		fileName := strings.Join(pathParts[1:], "/")
		objectKey := fmt.Sprintf("posts/%s/%s", blogID, fileName)

		return syncFileToMinio(ctx, cfg, mc, log, path, objectKey, info)
	})

	if err != nil {
		log.Errorf("Error during blog files walk: %v", err)
	}
}

// syncProfileFiles syncs profile files from profiles/ directory to profiles/ prefix in MinIO
func syncProfileFiles(ctx context.Context, cfg *config.Config, mc *minio.Client, log *zap.SugaredLogger) {
	profilesDir := constant.ProfileDir

	if _, err := os.Stat(profilesDir); os.IsNotExist(err) {
		log.Warn("Profiles directory does not exist, skipping profile sync")
		return
	}

	log.Debug("Syncing profile files...")

	err := filepath.Walk(profilesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Errorf("Error accessing path %s: %v", path, err)
			return nil // Continue with other files
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Extract username and filename from path
		relPath, err := filepath.Rel(profilesDir, path)
		if err != nil {
			log.Errorf("Error getting relative path for %s: %v", path, err)
			return nil
		}

		pathParts := strings.Split(filepath.ToSlash(relPath), "/")
		if len(pathParts) < 2 {
			log.Warnf("Unexpected profile file structure: %s", relPath)
			return nil
		}

		username := pathParts[0]
		fileName := strings.Join(pathParts[1:], "/")
		objectKey := fmt.Sprintf("profiles/%s/%s", username, fileName)

		return syncFileToMinio(ctx, cfg, mc, log, path, objectKey, info)
	})

	if err != nil {
		log.Errorf("Error during profile files walk: %v", err)
	}
}

// syncFileToMinio uploads a single file to MinIO if it doesn't already exist
func syncFileToMinio(ctx context.Context, cfg *config.Config, mc *minio.Client, log *zap.SugaredLogger, filePath, objectKey string, info os.FileInfo) error {
	// Check if object already exists in MinIO
	_, err := mc.StatObject(ctx, cfg.Minio.Bucket, objectKey, minio.StatObjectOptions{})
	if err == nil {
		// Object exists, skip
		return nil
	}

	// Object doesn't exist, upload it
	log.Debugf("Syncing file: %s -> %s", filePath, objectKey)

	file, err := os.Open(filePath)
	if err != nil {
		log.Errorf("Error opening file %s: %v", filePath, err)
		return nil
	}
	defer file.Close()

	contentType := getContentType(filepath.Base(objectKey))

	_, err = mc.PutObject(ctx, cfg.Minio.Bucket, objectKey, file, info.Size(), minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		log.Errorf("Error uploading file %s to MinIO: %v", objectKey, err)
		return nil
	}

	log.Debugf("Successfully synced: %s", objectKey)
	return nil
}

// getContentType determines content type based on file extension
func getContentType(fileName string) string {
	ext := strings.ToLower(filepath.Ext(fileName))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".avif":
		return "image/avif"
	case ".svg":
		return "image/svg+xml"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".pdf":
		return "application/pdf"
	case ".txt":
		return "text/plain"
	case ".json":
		return "application/json"
	default:
		return "application/octet-stream"
	}
}

// startMinioToFileSystemSync runs MinIO to filesystem sync every 24 hours
func startMinioToFileSystemSync(cfg *config.Config, mc *minio.Client, log *zap.SugaredLogger) {
	ticker := time.NewTicker(4 * time.Hour)
	defer ticker.Stop()

	// Run initial sync after 2 minutes (offset from filesystem->minio sync)
	time.Sleep(2 * time.Minute)
	syncMinioToFileSystem(cfg, mc, log)

	for range ticker.C {
		syncMinioToFileSystem(cfg, mc, log)
	}
}

// syncMinioToFileSystem syncs MinIO objects to filesystem (local or remote via SSH)
func syncMinioToFileSystem(cfg *config.Config, mc *minio.Client, log *zap.SugaredLogger) {
	ctx := context.Background()

	log.Debug("Starting MinIO to filesystem sync")

	// Sync posts from MinIO to filesystem
	syncMinioPostsToFileSystem(ctx, cfg, mc, log)

	// Sync profiles from MinIO to filesystem
	syncMinioProfilesToFileSystem(ctx, cfg, mc, log)

	log.Debug("Completed MinIO to filesystem sync")
}

// syncMinioPostsToFileSystem syncs posts/{blogID}/ objects to blogs/{blogID}/ filesystem
func syncMinioPostsToFileSystem(ctx context.Context, cfg *config.Config, mc *minio.Client, log *zap.SugaredLogger) {
	log.Debug("Syncing MinIO posts to filesystem...")

	// List all objects with posts/ prefix
	opts := minio.ListObjectsOptions{Prefix: "posts/", Recursive: true}
	for obj := range mc.ListObjects(ctx, cfg.Minio.Bucket, opts) {
		if obj.Err != nil {
			log.Errorf("Error listing MinIO objects: %v", obj.Err)
			continue
		}

		// Skip empty folders
		if strings.HasSuffix(obj.Key, "/") {
			continue
		}

		// Extract blog ID and filename from object key: posts/{blogID}/{fileName}
		parts := strings.SplitN(strings.TrimPrefix(obj.Key, "posts/"), "/", 2)
		if len(parts) != 2 {
			log.Warnf("Unexpected object key format: %s", obj.Key)
			continue
		}

		blogID := parts[0]
		fileName := parts[1]
		localPath := filepath.Join(constant.LocalBlogsDir, blogID, fileName)

		if err := syncMinioObjectToFile(ctx, cfg, mc, log, obj.Key, localPath); err != nil {
			log.Errorf("Failed to sync %s: %v", obj.Key, err)
		}
	}
}

// syncMinioProfilesToFileSystem syncs profiles/{username}/ objects to filesystem
func syncMinioProfilesToFileSystem(ctx context.Context, cfg *config.Config, mc *minio.Client, log *zap.SugaredLogger) {
	log.Debug("Syncing MinIO profiles to filesystem...")

	// List all objects with profiles/ prefix
	opts := minio.ListObjectsOptions{Prefix: "profiles/", Recursive: true}
	for obj := range mc.ListObjects(ctx, cfg.Minio.Bucket, opts) {
		if obj.Err != nil {
			log.Errorf("Error listing MinIO objects: %v", obj.Err)
			continue
		}

		// Skip empty folders
		if strings.HasSuffix(obj.Key, "/") {
			continue
		}

		// Extract username and filename from object key: profiles/{username}/{fileName}
		parts := strings.SplitN(strings.TrimPrefix(obj.Key, "profiles/"), "/", 2)
		if len(parts) != 2 {
			log.Warnf("Unexpected object key format: %s", obj.Key)
			continue
		}

		username := parts[0]
		fileName := parts[1]
		localPath := filepath.Join(constant.LocalProfilesDir, username, fileName)

		if err := syncMinioObjectToFile(ctx, cfg, mc, log, obj.Key, localPath); err != nil {
			log.Errorf("Failed to sync %s: %v", obj.Key, err)
		}
	}
}

// syncMinioObjectToFile downloads a MinIO object to local filesystem if it doesn't exist or is different
func syncMinioObjectToFile(ctx context.Context, cfg *config.Config, mc *minio.Client, log *zap.SugaredLogger, objectKey, localPath string) error {
	// Local filesystem sync (remote sync is now handled by minio-sync container)
	// Check if local file exists and compare with MinIO object
	if info, err := os.Stat(localPath); err == nil {
		objInfo, err2 := mc.StatObject(ctx, cfg.Minio.Bucket, objectKey, minio.StatObjectOptions{})
		if err2 == nil && info.Size() == objInfo.Size && !objInfo.LastModified.After(info.ModTime()) {
			// File is up to date, skip
			return nil
		}
	}

	log.Debugf("Syncing from MinIO: %s -> %s", objectKey, localPath)

	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// Get object from MinIO
	obj, err := mc.GetObject(ctx, cfg.Minio.Bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to get object from MinIO: %v", err)
	}
	defer obj.Close()

	// Create local file
	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %v", err)
	}
	defer file.Close()

	// Copy data
	if _, err := io.Copy(file, obj); err != nil {
		return fmt.Errorf("failed to copy data: %v", err)
	}

	log.Debugf("Successfully synced: %s", localPath)
	return nil
}
