package file_server

import (
	"context"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_file_service/pb"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/internal/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type FileServiceClient struct {
	Client    pb.UploadBlogFileClient
	ChunkSize int64 // Configurable chunk size for streaming
}

const (
	// Default chunk size for streaming uploads (1MB)
	DefaultChunkSize = 1024 * 1024
	// Maximum chunk size (10MB)
	MaxChunkSize = 10 * 1024 * 1024
)

func NewFileServiceClient(cfg *config.Config) pb.UploadBlogFileClient {
	cc, err := grpc.NewClient(cfg.Microservices.TheMonkeysFileStore,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			// Remove size constraints for large files
			grpc.MaxCallRecvMsgSize(100*1024*1024), // 100MB
			grpc.MaxCallSendMsgSize(100*1024*1024), // 100MB
		),
	)

	if err != nil {
		logrus.Errorf("cannot dial to grpc file server: %v", err)
	}

	logrus.Infof("✅ the monkeys gateway is dialing to the file rpc server at: %v", cfg.Microservices.TheMonkeysFileStore)
	return pb.NewUploadBlogFileClient(cc)
}

func RegisterFileStorageRouter(router *gin.Engine, cfg *config.Config, authClient *auth.ServiceClient) *FileServiceClient {
	mware := auth.InitAuthMiddleware(authClient)

	usc := &FileServiceClient{
		Client:    NewFileServiceClient(cfg),
		ChunkSize: DefaultChunkSize,
	}

	// Configure router for large file uploads
	router.MaxMultipartMemory = 100 << 20 // 100MB

	routes := router.Group("/api/v1/files")

	// Public routes for file retrieval
	routes.GET("/post/:id/:fileName", usc.GetBlogFile)
	routes.GET("/profile/:user_id/profile", usc.GetProfilePic)

	// Protected routes requiring authentication
	routes.Use(mware.AuthRequired)
	routes.POST("/post/:id", usc.UploadBlogFileChunked)
	routes.POST("/post/:id/stream", usc.UploadBlogFileStream)
	routes.DELETE("/post/:id/:fileName", usc.DeleteBlogFile)

	// Profile routes
	routes.POST("/profile/:user_id/profile", usc.UploadProfilePicChunked)
	routes.POST("/profile/:user_id/profile/stream", usc.UploadProfilePicStream)
	routes.DELETE("/profile/:user_id/profile", usc.DeleteProfilePic)

	// Enhanced streaming routes
	routesV11 := router.Group("/api/v1.1/files")
	routesV11.GET("/profile/:user_id/profile", usc.GetProfilePicStream)
	routesV11.GET("/post/:id/:fileName/stream", usc.GetBlogFileStream)

	// Object storage routes (MinIO-like)
	objectRoutes := router.Group("/api/v2/objects")
	objectRoutes.Use(mware.AuthRequired)
	{
		objectRoutes.POST("/:bucket/:objectKey", usc.UploadObject)
		objectRoutes.GET("/:bucket/:objectKey", usc.GetObject)
		objectRoutes.DELETE("/:bucket/:objectKey", usc.DeleteObject)
		objectRoutes.HEAD("/:bucket/:objectKey", usc.GetObjectInfo)
	}

	return usc
}

// UploadBlogFileChunked handles large file uploads with chunking for better performance
func (asc *FileServiceClient) UploadBlogFileChunked(ctx *gin.Context) {
	blogId := ctx.Param("id")

	// Get file from the form file section
	file, fileHeader, err := ctx.Request.FormFile("file")
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "error while getting the file"})
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			logrus.Errorf("Error closing file: %v", err)
		}
	}()

	// Get file size
	fileSize := fileHeader.Size
	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	logrus.Infof("Starting chunked upload for file: %s, size: %d bytes, type: %s", fileHeader.Filename, fileSize, contentType)

	stream, err := asc.Client.UploadBlogFile(context.Background())
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot stream file to the storage server"})
		return
	}

	buffer := make([]byte, asc.ChunkSize)
	chunkIndex := 0

	for {
		n, err := file.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			logrus.Errorf("Error reading file chunk: %v", err)
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "error reading file"})
			return
		}

		chunk := &pb.UploadBlogFileReq{
			BlogId:   blogId,
			Data:     buffer[:n],
			FileName: fileHeader.Filename,
		}

		if err := stream.Send(chunk); err != nil {
			logrus.Errorf("Error sending chunk %d: %v", chunkIndex, err)
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "error uploading file chunk"})
			return
		}

		chunkIndex++
		if chunkIndex%100 == 0 { // Log progress every 100 chunks
			logrus.Debugf("Sent chunk %d (%.2f MB)", chunkIndex, float64(chunkIndex*int(asc.ChunkSize))/(1024*1024))
		}
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		logrus.Errorf("Error closing stream: %v", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "error finalizing file upload"})
		return
	}

	logrus.Infof("Successfully uploaded file: %s in %d chunks (%.2f MB)", fileHeader.Filename, chunkIndex, float64(fileSize)/(1024*1024))
	ctx.JSON(http.StatusOK, gin.H{
		"message":      "File uploaded successfully",
		"file_name":    fileHeader.Filename,
		"file_size":    fileSize,
		"content_type": contentType,
		"chunks":       chunkIndex,
		"result":       resp,
	})
}

// UploadBlogFileStream handles streaming uploads for real-time file processing
func (asc *FileServiceClient) UploadBlogFileStream(ctx *gin.Context) {
	blogId := ctx.Param("id")
	fileName := ctx.Query("filename")
	contentType := ctx.GetHeader("Content-Type")

	if fileName == "" {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "filename parameter is required"})
		return
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	logrus.Infof("Starting stream upload for file: %s, type: %s", fileName, contentType)

	stream, err := asc.Client.UploadBlogFile(context.Background())
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot stream file to the storage server"})
		return
	}

	buffer := make([]byte, asc.ChunkSize)
	chunkIndex := 0
	totalBytes := int64(0)

	for {
		n, err := ctx.Request.Body.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			logrus.Errorf("Error reading request body: %v", err)
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "error reading request body"})
			return
		}

		chunk := &pb.UploadBlogFileReq{
			BlogId:   blogId,
			Data:     buffer[:n],
			FileName: fileName,
		}

		if err := stream.Send(chunk); err != nil {
			logrus.Errorf("Error sending stream chunk %d: %v", chunkIndex, err)
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "error uploading stream chunk"})
			return
		}

		chunkIndex++
		totalBytes += int64(n)
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		logrus.Errorf("Error closing stream: %v", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "error finalizing stream upload"})
		return
	}

	logrus.Infof("Successfully streamed file: %s in %d chunks (%.2f MB)", fileName, chunkIndex, float64(totalBytes)/(1024*1024))
	ctx.JSON(http.StatusOK, gin.H{
		"message":      "File streamed successfully",
		"file_name":    fileName,
		"content_type": contentType,
		"chunks":       chunkIndex,
		"total_bytes":  totalBytes,
		"result":       resp,
	})
}

// GetBlogFileStream handles streaming downloads for large files
func (asc *FileServiceClient) GetBlogFileStream(ctx *gin.Context) {
	blogId := ctx.Param("id")
	fileName := ctx.Param("fileName")

	logrus.Infof("Starting stream download for file: %s in blog: %s", fileName, blogId)

	stream, err := asc.Client.GetBlogFile(context.Background(), &pb.GetBlogFileReq{
		BlogId:   blogId,
		FileName: fileName,
	})
	if err != nil {
		logrus.Errorf("cannot connect to file rpc server, error: %v", err)
		ctx.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"message": "cannot connect to file server"})
		return
	}

	// Set appropriate headers for streaming
	ctx.Header("Content-Type", "application/octet-stream")
	ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	ctx.Header("Transfer-Encoding", "chunked")

	totalBytes := int64(0)
	chunkCount := 0

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			logrus.Infof("Completed stream download for file: %s (%.2f MB, %d chunks)", fileName, float64(totalBytes)/(1024*1024), chunkCount)
			break
		}
		if err != nil {
			if status, ok := status.FromError(err); ok {
				if status.Code() == codes.NotFound {
					ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "File not found"})
					return
				}
			}
			logrus.Errorf("Error receiving file chunk: %v", err)
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Error downloading file"})
			return
		}

		if _, err := ctx.Writer.Write(resp.Data); err != nil {
			logrus.Errorf("Error writing response data: %v", err)
			return
		}

		totalBytes += int64(len(resp.Data))
		chunkCount++
		ctx.Writer.Flush()
	}
}

func (asc *FileServiceClient) GetBlogFile(ctx *gin.Context) {
	blogId := ctx.Param("id")
	fileName := ctx.Param("fileName")

	stream, err := asc.Client.GetBlogFile(context.Background(), &pb.GetBlogFileReq{
		BlogId:   blogId,
		FileName: fileName,
	})
	if err != nil {
		logrus.Errorf("cannot connect to file rpc server, error: %v", err)
		ctx.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"message": "cannot connect to file server"})
		return
	}

	// For single chunk files, handle directly
	resp, err := stream.Recv()
	if err == io.EOF {
		logrus.Info("received the complete stream")
	}
	if err != nil {
		if status, ok := status.FromError(err); ok {
			if status.Code() == codes.NotFound {
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "File not found"})
				return
			}
		}
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
		return
	}

	// Set content type based on file extension
	ext := strings.ToLower(fileName[strings.LastIndex(fileName, ".")+1:])
	contentType := getContentType(ext)
	ctx.Header("Content-Type", contentType)

	if _, err := ctx.Writer.Write(resp.Data); err != nil {
		logrus.Errorf("error writing response data: %v", err)
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
		logrus.Errorf("cannot connect to user rpc server, error: %v", err)
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
			logrus.Errorf("Error closing file: %v", err)
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

	stream, err := asc.Client.GetProfilePic(context.Background(), &pb.GetProfilePicReq{
		UserId:   userID,
		FileName: "profile.png",
	})
	if err != nil {
		logrus.Errorf("cannot connect to user rpc server, error: %v", err)
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
		logrus.Errorf("error writing response data: %v", err)
	}
}

func (asc *FileServiceClient) GetProfilePicStream(ctx *gin.Context) {
	userID := ctx.Param("user_id")

	stream, err := asc.Client.GetProfilePic(context.Background(), &pb.GetProfilePicReq{
		UserId:   userID,
		FileName: "profile.png",
	})
	if err != nil {
		logrus.Errorf("cannot connect to user rpc server, error: %v", err)
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
		logrus.Errorf("cannot connect to user rpc server, error: %v", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "error while deleting the profile pic"})
		return
	}

	ctx.JSON(http.StatusAccepted, res)
}

// getContentType returns the MIME type based on file extension
func getContentType(ext string) string {
	mimeType := mime.TypeByExtension("." + ext)
	if mimeType == "" {
		return "application/octet-stream"
	}
	return mimeType
}

// UploadProfilePicChunked handles chunked profile picture uploads for large files
func (asc *FileServiceClient) UploadProfilePicChunked(ctx *gin.Context) {
	userId := ctx.Param("user_id")

	file, err := ctx.FormFile("file")
	if err != nil {
		logrus.Errorf("Error getting file from form: %v", err)
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "Error reading file"})
		return
	}

	logrus.Infof("Starting chunked profile picture upload for user: %s, file: %s (%.2f MB)",
		userId, file.Filename, float64(file.Size)/(1024*1024))

	src, err := file.Open()
	if err != nil {
		logrus.Errorf("Error opening file: %v", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Error opening file"})
		return
	}
	defer src.Close()

	// Use blog file upload for profile pictures (reusing existing infrastructure)
	stream, err := asc.Client.UploadBlogFile(context.Background())
	if err != nil {
		logrus.Errorf("Error creating upload stream: %v", err)
		ctx.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"message": "Cannot connect to file server"})
		return
	}

	chunkSize := 1024 * 1024 // 1MB chunks
	buffer := make([]byte, chunkSize)
	totalBytes := int64(0)
	chunkCount := 0

	for {
		n, err := src.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			logrus.Errorf("Error reading file chunk: %v", err)
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Error reading file"})
			return
		}

		req := &pb.UploadBlogFileReq{
			BlogId:   "profile_" + userId, // Use profile_ prefix to distinguish from blog files
			FileName: file.Filename,
			Data:     buffer[:n],
		}

		if err := stream.Send(req); err != nil {
			logrus.Errorf("Error sending chunk: %v", err)
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Error uploading file"})
			return
		}

		totalBytes += int64(n)
		chunkCount++

		// Log progress every 10MB or 100 chunks
		if chunkCount%100 == 0 || totalBytes%(10*1024*1024) == 0 {
			logrus.Infof("Upload progress for %s: %.2f MB (%d chunks)",
				file.Filename, float64(totalBytes)/(1024*1024), chunkCount)
		}
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		logrus.Errorf("Error finalizing upload: %v", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Error finalizing upload"})
		return
	}

	logrus.Infof("Profile picture upload completed for user: %s, file: %s (%.2f MB, %d chunks)",
		userId, file.Filename, float64(totalBytes)/(1024*1024), chunkCount)

	ctx.JSON(http.StatusOK, gin.H{
		"status":        resp.Status,
		"new_file_name": resp.NewFileName,
		"size":          totalBytes,
		"chunks":        chunkCount,
	})
}

// UploadProfilePicStream handles streaming profile picture uploads
func (asc *FileServiceClient) UploadProfilePicStream(ctx *gin.Context) {
	userId := ctx.Param("user_id")

	file, err := ctx.FormFile("file")
	if err != nil {
		logrus.Errorf("Error getting file from form: %v", err)
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "Error reading file"})
		return
	}

	logrus.Infof("Starting stream profile picture upload for user: %s, file: %s", userId, file.Filename)

	src, err := file.Open()
	if err != nil {
		logrus.Errorf("Error opening file: %v", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Error opening file"})
		return
	}
	defer src.Close()

	// Use blog file upload for profile pictures (reusing existing infrastructure)
	stream, err := asc.Client.UploadBlogFile(context.Background())
	if err != nil {
		logrus.Errorf("Error creating upload stream: %v", err)
		ctx.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"message": "Cannot connect to file server"})
		return
	}

	// Stream the entire file content
	fileData, err := io.ReadAll(src)
	if err != nil {
		logrus.Errorf("Error reading file data: %v", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Error reading file"})
		return
	}

	req := &pb.UploadBlogFileReq{
		BlogId:   "profile_" + userId, // Use profile_ prefix to distinguish from blog files
		FileName: file.Filename,
		Data:     fileData,
	}

	if err := stream.Send(req); err != nil {
		logrus.Errorf("Error sending file data: %v", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Error uploading file"})
		return
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		logrus.Errorf("Error finalizing upload: %v", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Error finalizing upload"})
		return
	}

	logrus.Infof("Profile picture stream upload completed for user: %s, file: %s (%.2f MB)",
		userId, file.Filename, float64(len(fileData))/(1024*1024))

	ctx.JSON(http.StatusOK, gin.H{
		"status":        resp.Status,
		"new_file_name": resp.NewFileName,
		"size":          len(fileData),
	})
}

// Object storage APIs (MinIO-like functionality)

// UploadObject handles uploading objects to a specific bucket
func (asc *FileServiceClient) UploadObject(ctx *gin.Context) {
	bucket := ctx.Param("bucket")
	objectKey := ctx.Param("objectKey")

	file, err := ctx.FormFile("file")
	if err != nil {
		logrus.Errorf("Error getting file from form: %v", err)
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "Error reading file"})
		return
	}

	logrus.Infof("Starting object upload to bucket: %s, key: %s, file: %s (%.2f MB)",
		bucket, objectKey, file.Filename, float64(file.Size)/(1024*1024))

	src, err := file.Open()
	if err != nil {
		logrus.Errorf("Error opening file: %v", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Error opening file"})
		return
	}
	defer src.Close()

	// For now, use blog file upload as object upload (this would need a dedicated gRPC method)
	stream, err := asc.Client.UploadBlogFile(context.Background())
	if err != nil {
		logrus.Errorf("Error creating upload stream: %v", err)
		ctx.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"message": "Cannot connect to file server"})
		return
	}

	chunkSize := 1024 * 1024 // 1MB chunks
	buffer := make([]byte, chunkSize)
	totalBytes := int64(0)
	chunkCount := 0

	for {
		n, err := src.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			logrus.Errorf("Error reading file chunk: %v", err)
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Error reading file"})
			return
		}

		req := &pb.UploadBlogFileReq{
			BlogId:   bucket, // Use bucket as blog ID for now
			FileName: objectKey,
			Data:     buffer[:n],
		}

		if err := stream.Send(req); err != nil {
			logrus.Errorf("Error sending chunk: %v", err)
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Error uploading object"})
			return
		}

		totalBytes += int64(n)
		chunkCount++
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		logrus.Errorf("Error finalizing upload: %v", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Error finalizing upload"})
		return
	}

	logrus.Infof("Object upload completed to bucket: %s, key: %s (%.2f MB, %d chunks)",
		bucket, objectKey, float64(totalBytes)/(1024*1024), chunkCount)

	ctx.JSON(http.StatusOK, gin.H{
		"status":         resp.Status,
		"new_file_name":  resp.NewFileName,
		"bucket":         bucket,
		"key":            objectKey,
		"size":           totalBytes,
		"chunks":         chunkCount,
	})
}

// GetObject retrieves an object from a specific bucket
func (asc *FileServiceClient) GetObject(ctx *gin.Context) {
	bucket := ctx.Param("bucket")
	objectKey := ctx.Param("objectKey")

	logrus.Infof("Getting object from bucket: %s, key: %s", bucket, objectKey)

	stream, err := asc.Client.GetBlogFile(context.Background(), &pb.GetBlogFileReq{
		BlogId:   bucket, // Use bucket as blog ID for now
		FileName: objectKey,
	})
	if err != nil {
		logrus.Errorf("cannot connect to file rpc server, error: %v", err)
		ctx.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"message": "cannot connect to file server"})
		return
	}

	// Set appropriate headers for object download
	ctx.Header("Content-Type", "application/octet-stream")
	ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", objectKey))

	totalBytes := int64(0)
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			logrus.Infof("Completed object download from bucket: %s, key: %s (%.2f MB)",
				bucket, objectKey, float64(totalBytes)/(1024*1024))
			break
		}
		if err != nil {
			if status, ok := status.FromError(err); ok {
				if status.Code() == codes.NotFound {
					ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "Object not found"})
					return
				}
			}
			logrus.Errorf("Error receiving object chunk: %v", err)
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Error downloading object"})
			return
		}

		if _, err := ctx.Writer.Write(resp.Data); err != nil {
			logrus.Errorf("Error writing response data: %v", err)
			return
		}

		totalBytes += int64(len(resp.Data))
		ctx.Writer.Flush()
	}
}

// DeleteObject removes an object from a specific bucket
func (asc *FileServiceClient) DeleteObject(ctx *gin.Context) {
	bucket := ctx.Param("bucket")
	objectKey := ctx.Param("objectKey")

	logrus.Infof("Deleting object from bucket: %s, key: %s", bucket, objectKey)

	// For now, use blog file deletion (this would need a dedicated gRPC method)
	res, err := asc.Client.DeleteBlogFile(context.Background(), &pb.DeleteBlogFileReq{
		BlogId:   bucket, // Use bucket as blog ID for now
		FileName: objectKey,
	})
	if err != nil {
		if status, ok := status.FromError(err); ok {
			if status.Code() == codes.NotFound {
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "Object not found"})
				return
			}
		}
		logrus.Errorf("Error deleting object: %v", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Error deleting object"})
		return
	}

	logrus.Infof("Object deleted from bucket: %s, key: %s", bucket, objectKey)
	ctx.JSON(http.StatusOK, gin.H{
		"message": res.Message,
		"bucket":  bucket,
		"key":     objectKey,
	})
}

// GetObjectInfo returns metadata about an object (HEAD request)
func (asc *FileServiceClient) GetObjectInfo(ctx *gin.Context) {
	bucket := ctx.Param("bucket")
	objectKey := ctx.Param("objectKey")

	logrus.Infof("Getting object info from bucket: %s, key: %s", bucket, objectKey)

	// This is a simplified implementation - in a real system, you'd have a dedicated metadata endpoint
	stream, err := asc.Client.GetBlogFile(context.Background(), &pb.GetBlogFileReq{
		BlogId:   bucket,
		FileName: objectKey,
	})
	if err != nil {
		logrus.Errorf("cannot connect to file rpc server, error: %v", err)
		ctx.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"message": "cannot connect to file server"})
		return
	}

	// Just receive the first chunk to verify existence and get basic info
	resp, err := stream.Recv()
	if err != nil {
		if status, ok := status.FromError(err); ok {
			if status.Code() == codes.NotFound {
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "Object not found"})
				return
			}
		}
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Error getting object info"})
		return
	}

	// Set headers with object metadata
	ext := strings.ToLower(objectKey[strings.LastIndex(objectKey, ".")+1:])
	contentType := getContentType(ext)

	ctx.Header("Content-Type", contentType)
	ctx.Header("Content-Length", strconv.Itoa(len(resp.Data)))
	ctx.Header("Last-Modified", time.Now().Format(http.TimeFormat))

	ctx.Status(http.StatusOK)
}
