package storage

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"

	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_file_service/pb"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/constants"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/internal/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type FileServiceClient struct {
	Client pb.UploadBlogFileClient
	log    *zap.SugaredLogger
	// MinIO fallback: serve files from object storage when the gRPC
	// file-store service is unreachable or the file was uploaded via v2.
	mc     *minio.Client
	bucket string
}

func NewFileServiceClient(cfg *config.Config, log *zap.SugaredLogger) pb.UploadBlogFileClient {
	storageService := fmt.Sprintf("%s:%d", cfg.Microservices.TheMonkeysFileStore, cfg.Microservices.StoragePort)
	cc, err := grpc.NewClient(storageService,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(constants.MaxMsgSize),
			grpc.MaxCallSendMsgSize(constants.MaxMsgSize),
		),
	)

	if err != nil {
		log.Errorf("cannot dial to grpc file server: %v", err)
	}

	log.Infof("✅ the monkeys gateway is dialing to the file rpc server at: %v", storageService)
	return pb.NewUploadBlogFileClient(cc)
}

func RegisterFileStorageRouter(router *gin.Engine, cfg *config.Config, authClient *auth.ServiceClient, log *zap.SugaredLogger) *FileServiceClient {
	mware := auth.InitAuthMiddleware(authClient, log)

	usc := &FileServiceClient{
		Client: NewFileServiceClient(cfg, log),
		log:    log,
	}

	// Initialise MinIO client for fallback reads.
	// If MinIO is unreachable the v1 handler degrades to gRPC-only.
	if cfg.Minio.Endpoint != "" {
		mc, err := minio.New(cfg.Minio.Endpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(cfg.Minio.AccessKey, cfg.Minio.SecretKey, ""),
			Secure: cfg.Minio.UseSSL,
		})
		if err != nil {
			log.Warnf("v1 storage: MinIO fallback unavailable: %v", err)
		} else {
			usc.mc = mc
			usc.bucket = cfg.Minio.Bucket
			log.Info("v1 storage: MinIO fallback enabled")
		}
	}

	routes := router.Group("/api/v1/files")

	// routes.GET("/post/:id/:fileName", usc.GetBlogFile)

	// // route defined to get profile pic
	// routes.GET("/profile/:user_id/profile", usc.GetProfilePic)

	routes.Use(mware.AuthRequired)
	// routes.POST("/post/:id", usc.UploadBlogFile)
	// routes.DELETE("/post/:id/:fileName", usc.DeleteBlogFile)

	// // route defined to access profile
	// routes.POST("/profile/:user_id/profile", usc.UploadProfilePic)
	// routes.DELETE("/profile/:user_id/profile", usc.DeleteProfilePic)

	// rotuesV11 := router.Group("/api/v1.1/files")
	// rotuesV11.GET("/profile/:user_id/profile", usc.GetProfilePicStream)

	return usc
}

func (asc *FileServiceClient) UploadBlogFile(ctx *gin.Context) {
	// get Id of the blog from the URL
	blogId := ctx.Param("id")

	// Get file from the form file section
	file, fileHeader, err := ctx.Request.FormFile("file")
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "error while getting the file"})
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			asc.log.Errorf("Error closing file: %v", err)
		}
	}()

	// Read the file and make it slice of bytes
	imageData, err := io.ReadAll(file)
	if err != nil {
		fmt.Println("Error reading image data:", err)
	}

	stream, err := asc.Client.UploadBlogFile(context.Background())
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot stream file to the storage server"})
		return
	}

	chunk := &pb.UploadBlogFileReq{
		BlogId:   blogId,
		Data:     imageData,
		FileName: fileHeader.Filename,
	}
	err = stream.Send(chunk)
	if err != nil {
		log.Fatal("cannot send file info to server: ", err, stream.RecvMsg(nil))
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Something went wrong while closing the stream"})
		return
	}

	// log.Printf("%+v\n", response)
	ctx.JSON(http.StatusAccepted, resp)
}

func (asc *FileServiceClient) GetBlogFile(ctx *gin.Context) {
	blogId := ctx.Param("id")
	fileName := ctx.Param("fileName")

	// --- MinIO fast path ---
	// If MinIO is configured, try to serve the object directly.
	// This covers files uploaded via v2 and files already synced to MinIO.
	if asc.mc != nil {
		objectKey := "posts/" + blogId + "/" + fileName
		obj, err := asc.mc.GetObject(context.Background(), asc.bucket, objectKey, minio.GetObjectOptions{})
		if err == nil {
			info, statErr := obj.Stat()
			if statErr == nil {
				ct := info.ContentType
				if ct == "" {
					ct = contentTypeFromExt(fileName)
				}
				ctx.Header("Content-Type", ct)
				ctx.Header("Cache-Control", "public, max-age=31536000, immutable")
				ctx.Status(http.StatusOK)
				if _, err := io.Copy(ctx.Writer, obj); err != nil {
					asc.log.Errorf("MinIO stream error for %s: %v", objectKey, err)
				}
				_ = obj.Close()
				return
			}
			_ = obj.Close()
			// Object not found in MinIO — fall through to gRPC.
		}
	}

	// --- gRPC fallback ---
	stream, err := asc.Client.GetBlogFile(context.Background(), &pb.GetBlogFileReq{
		BlogId:   blogId,
		FileName: fileName,
	})
	if err != nil {
		asc.log.Errorf("cannot connect to user rpc server, error: %v", err)
		_ = ctx.AbortWithError(http.StatusBadGateway, err)
		return
	}

	resp, err := stream.Recv()
	if err == io.EOF {
		asc.log.Info("received the complete stream")
	}
	if err != nil {
		// Check for gRPC error code
		if status, ok := status.FromError(err); ok {
			if status.Code() == codes.NotFound {
				// Handle "profile picture not found" error
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "Profile picture for user is not found"})
				return
			}
		}
		// Fallback for other errors
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
		return
	}

	// TODO: Remove the comment lines
	// ctx.Header("Content-Disposition", "attachment; filename=file-name.txt")
	// ctx.Data(http.StatusOK, "application/octet-stream", resp.Data)

	// ctx.JSON(http.StatusAccepted, "uploaded")
	if _, err := ctx.Writer.Write(resp.Data); err != nil {
		asc.log.Errorf("error writing response data: %v", err)
	}
}

func (asc *FileServiceClient) DeleteBlogFile(ctx *gin.Context) {
	blogId := ctx.Param("id")
	fileName := ctx.Param("fileName")

	res, err := asc.Client.DeleteBlogFile(context.Background(), &pb.DeleteBlogFileReq{
		BlogId:   blogId,
		FileName: fileName,
	})

	if err != nil {
		asc.log.Errorf("cannot connect to user rpc server, error: %v", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "error while deleting the file"})
		return
	}

	ctx.JSON(http.StatusAccepted, res)
}

func (asc *FileServiceClient) UploadProfilePic(ctx *gin.Context) {
	// get Id of the blog from the URL
	userId := ctx.Param("user_id")

	// Get file from the form file section
	file, fileHeader, err := ctx.Request.FormFile("profile_pic")
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "error while getting the profile pic"})
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			asc.log.Errorf("Error closing file: %v", err)
		}
	}()

	// Read the file and make it slice of bytes
	imageData, err := io.ReadAll(file)
	if err != nil {
		fmt.Println("Error reading image data:", err)
	}

	stream, err := asc.Client.UploadProfilePic(context.Background())
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot stream profile pic to the storage server"})
		return
	}

	chunk := &pb.UploadProfilePicReq{
		UserId:   userId,
		Data:     imageData,
		FileType: fileHeader.Filename,
	}
	err = stream.Send(chunk)
	if err != nil {
		log.Fatal("cannot send file info to server: ", err, stream.RecvMsg(nil))
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Something went wrong while closing the profile pic stream"})
		return
	}

	// log.Printf("%+v\n", response)
	ctx.JSON(http.StatusAccepted, resp)
}
func (asc *FileServiceClient) GetProfilePic(ctx *gin.Context) {
	userID := ctx.Param("user_id")

	// --- MinIO fast path ---
	if asc.mc != nil {
		objectKey := "profiles/" + userID + "/profile"
		obj, err := asc.mc.GetObject(context.Background(), asc.bucket, objectKey, minio.GetObjectOptions{})
		if err == nil {
			info, statErr := obj.Stat()
			if statErr == nil {
				ct := info.ContentType
				if ct == "" {
					ct = "image/png"
				}
				ctx.Header("Content-Type", ct)
				ctx.Header("Cache-Control", "public, max-age=3600")
				ctx.Status(http.StatusOK)
				if _, err := io.Copy(ctx.Writer, obj); err != nil {
					asc.log.Errorf("MinIO stream error for profile %s: %v", objectKey, err)
				}
				_ = obj.Close()
				return
			}
			_ = obj.Close()
		}
	}

	// --- gRPC fallback ---
	stream, err := asc.Client.GetProfilePic(context.Background(), &pb.GetProfilePicReq{
		UserId:   userID,
		FileName: "profile.png",
	})
	if err != nil {
		asc.log.Errorf("cannot connect to user rpc server, error: %v", err)
		ctx.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"message": "cannot connect to user rpc server"})
		return
	}

	resp, err := stream.Recv()
	if err != nil {
		// Check for gRPC error code
		if status, ok := status.FromError(err); ok {
			if status.Code() == codes.NotFound {
				// Handle "profile picture not found" error
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "Profile picture for user is not found"})
				return
			}
		}
		// Fallback for other errors
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
		return
	}

	if _, err := ctx.Writer.Write(resp.Data); err != nil {
		asc.log.Errorf("error writing response data: %v", err)
	}
}

func (asc *FileServiceClient) GetProfilePicStream(ctx *gin.Context) {
	userID := ctx.Param("user_id")

	stream, err := asc.Client.GetProfilePic(context.Background(), &pb.GetProfilePicReq{
		UserId:   userID,
		FileName: "profile.png",
	})
	if err != nil {
		asc.log.Errorf("cannot connect to user rpc server, error: %v", err)
		ctx.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"message": "cannot connect to user rpc server"})
		return
	}

	//ctx.Header("Content-Disposition", "attachment; filename=img.png")
	ctx.Header("Content-Type", "image/png")

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}

		if err != nil {
			// Check for gRPC error code
			if status, ok := status.FromError(err); ok {
				if status.Code() == codes.NotFound {
					// Handle "profile picture not found" error
					ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "Profile picture for user is not found"})
					return
				}
			}
			// Fallback for other errors
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
			return
		}

		_, err = ctx.Writer.Write(resp.Data)

		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
		}

		ctx.Writer.Flush()
	}
}

func (asc *FileServiceClient) DeleteProfilePic(ctx *gin.Context) {
	userId := ctx.Param("user_id")

	res, err := asc.Client.DeleteProfilePic(context.Background(), &pb.DeleteProfilePicReq{
		UserId:   userId,
		FileName: "profile.png",
	})

	if err != nil {
		asc.log.Errorf("cannot connect to user rpc server, error: %v", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "error while deleting the profile pic"})
		return
	}

	ctx.JSON(http.StatusAccepted, res)
}

// contentTypeFromExt returns a MIME type for common image/file extensions.
// Used when MinIO object metadata lacks Content-Type.
func contentTypeFromExt(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
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
	default:
		return "application/octet-stream"
	}
}
