package blog_client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_blog/pb"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/constants"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/internal/auth"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/internal/user_service"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins
		return true
	},
}

type BlogServiceClient struct {
	Client     pb.BlogServiceClient
	cacheMutex sync.Mutex
	cacheTime  time.Time
	cache      string
	cache1     string
	userCli    *user_service.UserServiceClient
	config     *config.Config
}

func NewBlogServiceClient(cfg *config.Config) pb.BlogServiceClient {
	cc, err := grpc.NewClient(cfg.Microservices.TheMonkeysBlog, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logrus.Errorf("cannot dial to blog server: %v", err)
	}

	logrus.Infof("✅ the monkeys gateway is dialing to the blog rpc server at: %v", cfg.Microservices.TheMonkeysBlog)
	return pb.NewBlogServiceClient(cc)
}

func RegisterBlogRouter(router *gin.Engine, cfg *config.Config, authClient *auth.ServiceClient, userClient *user_service.UserServiceClient) *BlogServiceClient {
	mware := auth.InitAuthMiddleware(authClient)

	blogClient := &BlogServiceClient{
		Client:  NewBlogServiceClient(cfg),
		userCli: userClient,
		config:  cfg,
	}
	routes := router.Group("/api/v1/blog")
	routes.GET("/latest", blogClient.GetLatest100Blogs)
	routes.GET("/:blog_id", blogClient.GetPublishedBlogById)
	routes.GET("/tags", blogClient.GetBlogsByTagsName)
	routes.GET("/all/publishes/:username", blogClient.AllPublishesByUserName)
	routes.GET("/published/:acc_id/:blog_id", blogClient.GetPublishedBlogByAccId)
	routes.GET("/news1", blogClient.GetNews1)
	routes.GET("/news2", blogClient.GetNews2)
	routes.GET("/news3", blogClient.GetNews3)

	// Use AuthRequired for basic authorization
	routes.Use(mware.AuthRequired)

	// Use AuthzRequired for routes needing access control
	routes.GET("/draft/:blog_id", mware.AuthzRequired, blogClient.DraftABlog)
	routes.GET("/draft/v2/:blog_id", mware.AuthzRequired, blogClient.DraftABlogV2)

	routes.POST("/publish/:blog_id", mware.AuthzRequired, blogClient.PublishBlogById)
	routes.POST("/archive/:blog_id", mware.AuthzRequired, blogClient.ArchiveBlogById)
	routes.GET("/all/drafts/:acc_id", blogClient.AllDrafts)
	routes.GET("/all-col/:acc_id", blogClient.AllCollabBlogs)
	routes.GET("/drafts/:acc_id/:blog_id", mware.AuthzRequired, blogClient.GetDraftBlogByAccId)
	// routes.GET("/all/publishes/:acc_id", blogClient.AllPublishesByAccountId)

	routes.GET("/my-drafts/:blog_id", mware.AuthzRequired, blogClient.GetDraftBlogByBlogId)

	routes.GET("/all/bookmarks", blogClient.GetBookmarks)

	routes.DELETE("/:blog_id", mware.AuthzRequired, blogClient.DeleteBlogById)

	return blogClient
}

func (asc *BlogServiceClient) DraftABlog(ctx *gin.Context) {
	id := ctx.Param("blog_id")

	logrus.Infof("traffic is coming from ip: %v", ctx.ClientIP())
	ipAddress := ctx.Request.Header.Get("IP")
	client := ctx.Request.Header.Get("Client")

	// Check if blog exists
	resp, err := asc.Client.CheckIfBlogsExist(context.Background(), &pb.BlogByIdReq{
		BlogId: id,
	})
	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.InvalidArgument:
				ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "incomplete request, please provide correct input parameters"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot fetch the draft blogs"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
	}

	var action string
	var initialLogDone bool // Flag to avoid repeated logging

	if resp.BlogExists {
		if !utils.CheckUserAccessInContext(ctx, constants.PermissionEdit) {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "you are not allowed to perform this action"})
			return
		}
		action = constants.BLOG_UPDATE
	} else {
		action = constants.BLOG_CREATE
	}

	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		logrus.Errorf("error upgrading connection: %v", err)
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	defer conn.Close() // Ensure WebSocket is closed when done

	if asc.Client == nil {
		logrus.Errorf("BlogServiceClient is not initialized")
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// Infinite loop to listen to WebSocket connection
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			logrus.Errorf("error reading the message: %v", err)
			return
		}

		// Unmarshal the received message into the Blog struct
		var draftBlog pb.DraftBlogRequest
		err = json.Unmarshal(msg, &draftBlog)
		if err != nil {
			logrus.Errorf("Error un-marshalling message: %v", err)
			return
		}

		draftBlog.BlogId = id
		draftBlog.Ip = ipAddress
		draftBlog.Client = client

		// Only set the action and log the initial creation or update once
		if !initialLogDone {
			draftBlog.Action = action
			initialLogDone = true // Prevent further logging
		}

		resp, err := asc.Client.DraftBlog(context.Background(), &draftBlog)
		if err != nil {
			logrus.Errorf("error while creating draft blog: %v", err)
			return
		}

		response, err := json.Marshal(resp)
		if err != nil {
			logrus.Println("Error marshalling response message:", err)
			return
		}

		// Send a response message to the client (optional)
		err = conn.WriteMessage(websocket.TextMessage, response)
		if err != nil {
			logrus.Errorf("error returning the response message: %v", err)
			return
		}
	}
}

func (asc *BlogServiceClient) AllDrafts(ctx *gin.Context) {
	// Check permissions:
	// if !utils.CheckUserAccessInContext(ctx, constants.PermissionEdit) {
	// 	ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "you are not allowed to perform this action"})
	// 	return
	// }

	tokenAccountId := ctx.GetString("accountId")
	accId := ctx.Param("acc_id")

	if tokenAccountId != accId {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "you are not allowed to perform this action"})
		return
	}
	res, err := asc.Client.GetDraftBlogsByAccId(context.Background(), &pb.BlogByIdReq{
		OwnerAccountId: accId,
		// Email:          "",
		// Username:       "",
	})

	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.InvalidArgument:
				ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "incomplete request, please provide correct input parameters"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot fetch the draft blogs"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
	}

	ctx.JSON(http.StatusOK, res)
}

func (asc *BlogServiceClient) AllCollabBlogs(ctx *gin.Context) {

	tokenAccountId := ctx.GetString("accountId")
	accId := ctx.Param("acc_id")

	if tokenAccountId != accId {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "you are not allowed to perform this action"})
		return
	}

	// Get all the drafted blogs
	uc, err := asc.userCli.GetBlogsIds(accId, "colab")
	if err != nil {
		logrus.Errorf("cannot get the colab blogs, error: %v", err)
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "cannot find the colab blogs"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot get the colab blogs"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
	}

	draftBlogId := []string{}
	publishedBlogId := []string{}
	for _, blog := range uc.Blogs {
		if blog.Status == constants.BlogStatusDraft {
			draftBlogId = append(draftBlogId, blog.BlogId)
		} else {
			publishedBlogId = append(publishedBlogId, blog.BlogId)
		}
	}

	if len(draftBlogId) == 0 && len(publishedBlogId) == 0 {
		ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "no draft/published blogs found"})
		return
	}

	var drafts, published interface{}

	// Fetch drafts if they exist
	if len(draftBlogId) > 0 {
		drafts, err = asc.Client.GetAllBlogsByBlogIds(context.Background(), &pb.GetBlogsByBlogIds{
			BlogIds: draftBlogId,
		})
		if err != nil {
			handleBlogFetchError(ctx, err, "draft")
			return
		}
	}

	// Fetch published blogs if they exist
	if len(publishedBlogId) > 0 {
		published, err = asc.Client.GetAllBlogsByBlogIds(context.Background(), &pb.GetBlogsByBlogIds{
			BlogIds: publishedBlogId,
		})
		if err != nil {
			handleBlogFetchError(ctx, err, "published")
			return
		}
	}

	// Respond with the found drafts and/or published blogs
	ctx.JSON(http.StatusOK, gin.H{"drafts": drafts, "published": published})
}

func handleBlogFetchError(ctx *gin.Context, err error, blogType string) {
	if status, ok := status.FromError(err); ok {
		switch status.Code() {
		case codes.InvalidArgument:
			ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("incomplete request, unable to fetch %s blogs", blogType)})
		case codes.Internal:
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": fmt.Sprintf("cannot fetch the %s blogs", blogType)})
		default:
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
		}
	} else {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
	}
}

func (asc *BlogServiceClient) AllPublishesByAccountId(ctx *gin.Context) {
	accId := ctx.Param("acc_id")

	res, err := asc.Client.GetPublishedBlogsByAccID(context.Background(), &pb.BlogByIdReq{
		OwnerAccountId: accId,
	})

	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.InvalidArgument:
				ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "incomplete request, please provide correct input parameters"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot fetch the draft blogs"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
	}

	ctx.JSON(http.StatusOK, res)
}

func (asc *BlogServiceClient) AllPublishesByUserName(ctx *gin.Context) {
	userName := ctx.Param("username")

	// Get the account_id from the username
	userInfo, err := asc.userCli.GetUserDetails(userName)
	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "the user does not exist"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot fetch the user details"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
	}

	res, err := asc.Client.GetPublishedBlogsByAccID(context.Background(), &pb.BlogByIdReq{
		OwnerAccountId: userInfo.AccountId,
	})

	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.InvalidArgument:
				ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "incomplete request, please provide correct input parameters"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot fetch the draft blogs"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
	}

	ctx.JSON(http.StatusOK, res)
}

func (asc *BlogServiceClient) GetDraftBlogByAccId(ctx *gin.Context) {
	// Check permissions:
	if !utils.CheckUserAccessInContext(ctx, constants.PermissionEdit) {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "you are not allowed to perform this action"})
		return
	}

	// Extract account_id and blog_id from URL parameters
	accID := ctx.Param("acc_id")
	blogID := ctx.Param("blog_id")

	// Ensure acc_id and blog_id are not empty
	if accID == "" || blogID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "account id and blog id are required"})
		return
	}

	// Fetch the drafted blog by blog_id and owner_account_id
	blog, err := asc.Client.GetDraftBlogById(ctx, &pb.BlogByIdReq{
		BlogId:         blogID,
		OwnerAccountId: accID,
	})
	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "drafted blog not found"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch drafted blog"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
	}

	// Return the drafted blog as a JSON response
	ctx.JSON(http.StatusOK, blog)
}

func (asc *BlogServiceClient) GetPublishedBlogByAccId(ctx *gin.Context) {
	// Extract account_id and blog_id from URL parameters
	accID := ctx.Param("acc_id")
	blogID := ctx.Param("blog_id")

	// Ensure acc_id and blog_id are not empty
	if accID == "" || blogID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "account id and blog id are required"})
		return
	}

	// Fetch the published blog by blog_id and owner_account_id
	blog, err := asc.Client.GetPublishedBlogByIdAndOwnerId(ctx, &pb.BlogByIdReq{
		BlogId:         blogID,
		OwnerAccountId: accID,
	})
	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "published blog not found"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "failed to fetch published blog"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
	}

	// If no blog is found, return a 404
	if blog == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "published blog not found"})
		return
	}

	// Return the published blog as a JSON response
	ctx.JSON(http.StatusOK, blog)
}

func (asc *BlogServiceClient) PublishBlogById(ctx *gin.Context) {
	ipAddress := ctx.Request.Header.Get("IP")
	client := ctx.Request.Header.Get("Client")

	// Check permissions:
	if !utils.CheckUserAccessInContext(ctx, "Publish") {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "you are not allowed to perform this action"})
		return
	}
	accId := ctx.GetString("accountId")

	id := ctx.Param("blog_id")
	resp, err := asc.Client.PublishBlog(context.Background(), &pb.PublishBlogReq{
		BlogId:    id,
		AccountId: accId,
		Ip:        ipAddress,
		Client:    client,
	})

	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "the blog does not exist"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot get the draft blogs"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
	}

	ctx.JSON(http.StatusOK, resp)
}

func (asc *BlogServiceClient) GetBlogsByTagsName(ctx *gin.Context) {
	tags := Tags{}
	if err := ctx.BindJSON(&tags); err != nil {
		logrus.Errorf("error while marshalling tags: %v", err)
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "tags aren't properly formatted"})
		return
	}

	req := &pb.GetBlogsByTagsNameReq{}
	req.TagNames = append(req.TagNames, tags.Tags...)

	resp, err := asc.Client.GetPublishedBlogsByTagsName(context.Background(), req)
	if err != nil {
		logrus.Errorf("error while fetching the blog: %v", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot get the blogs"})
		return
	}

	ctx.JSON(http.StatusOK, resp)
}

func (svc *BlogServiceClient) GetPublishedBlogById(ctx *gin.Context) {
	id := ctx.Param("blog_id")

	res, err := svc.Client.GetPublishedBlogById(context.Background(), &pb.BlogByIdReq{BlogId: id})
	if err != nil {
		logrus.Errorf("cannot get the blog, error: %v", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot get the blogs"})
		return
	}

	ctx.JSON(http.StatusCreated, res)
}

func (asc *BlogServiceClient) ArchiveBlogById(ctx *gin.Context) {
	// Check permissions:
	if !utils.CheckUserAccessInContext(ctx, "Archive") {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "you are not allowed to perform this action"})
		return
	}

	id := ctx.Param("blog_id")
	resp, err := asc.Client.ArchiveBlogById(context.Background(), &pb.ArchiveBlogReq{
		BlogId: id,
	})

	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "the blog does not exist"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot archive the blogs"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
	}

	ctx.JSON(http.StatusOK, resp)
}

func (asc *BlogServiceClient) GetLatest100Blogs(ctx *gin.Context) {
	res, err := asc.Client.GetLatest100Blogs(context.Background(), &pb.GetBlogsByTagsNameReq{})
	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "the blogs do not exist"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot find the latest blogs"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
	}

	ctx.JSON(http.StatusOK, res)
}

func (asc *BlogServiceClient) DeleteBlogById(ctx *gin.Context) {
	// Check permissions to Delete
	if !utils.CheckUserAccessInContext(ctx, "Delete") {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "you are not allowed to perform this action"})
		return
	}

	blogId := ctx.Param("blog_id")
	accId := ctx.GetString("accountId")
	res, err := asc.Client.DeleteABlogByBlogId(context.Background(), &pb.DeleteBlogReq{
		BlogId:         blogId,
		OwnerAccountId: accId,
	})
	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "the blog does not exist"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "couldn't delete the blog due to some internal error"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
	}

	ctx.JSON(http.StatusOK, res)
}

func (asc *BlogServiceClient) GetBookmarks(ctx *gin.Context) {
	tokenAccountId := ctx.GetString("accountId")

	// Get all the drafted blogs
	uc, err := asc.userCli.GetBlogsIds(tokenAccountId, "bookmark")
	if err != nil {
		logrus.Errorf("cannot get the bookmarked blogs, error: %v", err)
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "cannot find the bookmarked blogs"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot get the bookmarked blogs"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
	}

	fmt.Printf("uc: %+v\n", uc)

	blogId := []string{}

	for _, blog := range uc.Blogs {
		blogId = append(blogId, blog.BlogId)
	}

	resp, err := asc.Client.GetAllBlogsByBlogIds(context.Background(), &pb.GetBlogsByBlogIds{
		BlogIds: blogId,
	})
	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "the bookmarks do not exist"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot find the bookmarks"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
	}

	ctx.JSON(http.StatusOK, resp)
}

// TODO: Add access control over this function for all blogs
// func (asc *BlogServiceClient) GetAllBlogsByBlogIds(ctx *gin.Context) {
// 	ids := ctx.Query("ids")
// 	idSlice := strings.Split(ids, ",")

// 	if len(idSlice) == 0 {
// 		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "please provide blog ids"})
// 		return
// 	}

// 	resp, err := asc.Client.GetAllBlogsByBlogIds(context.Background(), &pb.GetBlogsByBlogIds{
// 		BlogIds: idSlice,
// 	})
// 	if err != nil {
// 		if status, ok := status.FromError(err); ok {
// 			switch status.Code() {
// 			case codes.InvalidArgument:
// 				ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "incomplete request, please provide correct input parameters"})
// 				return
// 			case codes.Internal:
// 				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "couldn't delete the blog due to some internal error"})
// 				return
// 			default:
// 				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
// 				return
// 			}
// 		}
// 	}

// 	ctx.JSON(http.StatusOK, resp)
// }

func (asc *BlogServiceClient) GetDraftBlogByBlogId(ctx *gin.Context) {
	blogId := ctx.Param("blog_id")

	// Check permissions:
	if !utils.CheckUserAccessInContext(ctx, constants.PermissionEdit) {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "you are not allowed to perform this action"})
		return
	}

	resp, err := asc.Client.GetDraftBlogByBlogId(context.Background(), &pb.BlogByIdReq{
		BlogId: blogId,
	})
	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "the blog does not exist"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "couldn't find the blog due to some internal error"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
	}

	ctx.JSON(http.StatusOK, resp)
}

func (asc *BlogServiceClient) GetColDraftBlogByBlogId(ctx *gin.Context) {
	blogId := ctx.Param("blog_id")

	// Check permissions:
	if !utils.CheckUserAccessInContext(ctx, constants.PermissionEdit) {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "you are not allowed to perform this action"})
		return
	}

	resp, err := asc.Client.GetDraftBlogByBlogId(context.Background(), &pb.BlogByIdReq{
		BlogId: blogId,
	})
	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "the blog does not exist"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "couldn't find the blog due to some internal error"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
	}

	ctx.JSON(http.StatusOK, resp)
}

// ******************************************************* Third Party API ************************************************

type NewsResponse struct {
	Data interface{} `json:"data"`
}

const apiURL = "http://api.mediastack.com/v1/news?access_key=%s&language=en&categories=business,entertainment,sports,science,technology&limit=100"

func (svc *BlogServiceClient) GetNews1(ctx *gin.Context) {
	svc.cacheMutex.Lock()
	defer svc.cacheMutex.Unlock()

	// Check if cache is valid
	if time.Now().Format("2006-01-02") == svc.cacheTime.Format("2006-01-02") && svc.cache != "" {
		ctx.Data(http.StatusOK, "application/json", []byte(svc.cache))
		return
	}

	resp, err := http.Get(fmt.Sprintf(apiURL, svc.config.Keys.MediaStack))
	if err != nil || resp.StatusCode != http.StatusOK {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch news"})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response body"})
		return
	}

	// Cache the response
	svc.cache = string(body)
	svc.cacheTime = time.Now()

	ctx.Data(http.StatusOK, "application/json", body)
}

const apiURL2 = "https://newsapi.org/v2/everything?domains=techcrunch.com,thenextweb.com&apiKey=%s&language=en"

func (svc *BlogServiceClient) GetNews2(ctx *gin.Context) {
	svc.cacheMutex.Lock()
	defer svc.cacheMutex.Unlock()

	// Check if cache1 is valid
	if time.Now().Format("2006-01-02") == svc.cacheTime.Format("2006-01-02") && svc.cache1 != "" {
		ctx.Data(http.StatusOK, "application/json", []byte(svc.cache1))
		return
	}
	// Call the API
	resp, err := http.Get(fmt.Sprintf(apiURL2, svc.config.Keys.NewsApi))
	if err != nil || resp.StatusCode != http.StatusOK {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch news"})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response body"})
		return
	}

	// Cache the response
	svc.cache1 = string(body)
	svc.cacheTime = time.Now()

	ctx.Data(http.StatusOK, "application/json", body)
}

func (svc *BlogServiceClient) GetNews3(ctx *gin.Context) {
	// Call the API
	resp, err := http.Get("https://hindustantimes-1-t3366110.deta.app/top-world-news")
	if err != nil || resp.StatusCode != http.StatusOK {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch news"})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response body"})
		return
	}

	ctx.Data(http.StatusOK, "application/json", body)
}

// func (svc *BlogServiceClient) DeleteBlogById(ctx *gin.Context) {
// 	id := ctx.Param("id")

// 	res, err := svc.Client.DeleteBlogById(context.Background(), &pb.DeleteBlogByIdRequest{Id: id})
// 	if err != nil {
// 		logrus.Errorf("cannot connect to article rpc server, error: %v", err)
// 		_ = ctx.AbortWithError(http.StatusInternalServerError, err)
// 		return
// 	}

// 	ctx.JSON(http.StatusCreated, res)
// }

// func (svc *BlogServiceClient) Get100PostsByTags(ctx *gin.Context) {
// 	logrus.Infof("traffic is coming from ip: %v", ctx.ClientIP())

// 	reqObj := Tag{}

// 	if err := ctx.BindJSON(&reqObj); err != nil {
// 		logrus.Errorf("invalid body, error: %v", err)
// 		_ = ctx.AbortWithError(http.StatusBadRequest, err)
// 		return
// 	}

// 	stream, err := svc.Client.GetBlogsByTag(context.Background(), &pb.GetBlogsByTagReq{
// 		TagName: reqObj.TagName,
// 	})

// 	if err != nil {
// 		logrus.Errorf("cannot connect to article stream rpc server, error: %v", err)
// 		_ = ctx.AbortWithError(http.StatusBadGateway, err)
// 		return
// 	}

// 	response := []*pb.GetBlogsResponse{}
// 	for {
// 		resp, err := stream.Recv()
// 		if err == io.EOF {
// 			break
// 		}
// 		if err != nil {
// 			logrus.Errorf("cannot get the stream data, error: %+v", err)
// 		}

// 		response = append(response, resp)
// 	}

// 	ctx.JSON(http.StatusCreated, response)
// }

func (asc *BlogServiceClient) DraftABlogV2(ctx *gin.Context) {
	id := ctx.Param("blog_id")
	tokenAccountId := ctx.GetString("accountId")

	// Check if blog exists
	resp, err := asc.Client.CheckIfBlogsExist(context.Background(), &pb.BlogByIdReq{
		BlogId: id,
	})
	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.InvalidArgument:
				ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "incomplete request, please provide correct input parameters"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot fetch the draft blogs"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
	}

	if resp.BlogExists {
		if !utils.CheckUserAccessInContext(ctx, constants.PermissionEdit) {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "you are not allowed to perform this action"})
			return
		}
	}

	// Upgrade the connection to WebSocket
	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		logrus.Errorf("error upgrading connection: %v", err)
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	if asc.Client == nil {
		logrus.Errorf("BlogServiceClient is not initialized")
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// Infinite loop to listen to WebSocket connection
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			logrus.Errorf("error reading the message: %v", err)
			return
		}

		// Unmarshal the received JSON message into a Go structure
		var incomingData struct {
			OwnerAccountId string    `json:"owner_account_id"`
			Blog           BlogInput `json:"blog"`
			Tags           []string  `json:"tags"`
			ContentType    string    `json:"content_type"` // Identifies whether it's "editorjs" or "platejs"
			Time           int64     `json:"time"`
		}

		err = json.Unmarshal(msg, &incomingData)
		if err != nil {
			logrus.Errorf("Error un-marshalling message: %v", err)
			return
		}

		// Create the DraftBlogV2Req Protobuf message
		var draftBlog *pb.DraftBlogV2Req

		switch incomingData.ContentType {
		case "editorjs":
			draftBlog = &pb.DraftBlogV2Req{
				BlogId:         id,
				OwnerAccountId: tokenAccountId,
				Tags:           incomingData.Tags,
				Blog: &pb.DraftBlogV2Req_EditorJsContent{
					EditorJsContent: &pb.EditorJSContent{
						Blocks: mapBlocksToProto(incomingData.Blog.Blocks), // Handle Editor.js blocks
						Time:   incomingData.Time,
					},
				},
			}
		case "platejs":
			draftBlog = &pb.DraftBlogV2Req{
				BlogId:         id,
				OwnerAccountId: tokenAccountId,
				Tags:           incomingData.Tags,
				Blog: &pb.DraftBlogV2Req_PlateData{
					PlateData: &pb.PlateData{
						Nodes: mapNodesToProto(incomingData.Blog.Nodes), // Handle Plate.js nodes
					},
				},
			}
		default:
			logrus.Errorf("Unsupported content type: %s", incomingData.ContentType)
			return
		}

		// Send the request to the gRPC service
		resp, err := asc.Client.DraftBlogV2(context.Background(), draftBlog)
		if err != nil {
			logrus.Errorf("error while creating draft blog: %v", err)
			return
		}

		response, err := json.Marshal(resp)
		if err != nil {
			logrus.Println("Error marshalling response message:", err)
			return
		}

		// Send a response message to the client
		err = conn.WriteMessage(websocket.TextMessage, response)
		if err != nil {
			logrus.Errorf("error returning the response message: %v", err)
			return
		}
	}
}

// Helper function to map the incoming JSON blocks to Protobuf blocks (Editor.js)
func mapBlocksToProto(blocks []BlockInput) []*pb.Block {
	var protoBlocks []*pb.Block

	for _, block := range blocks {
		protoBlock := &pb.Block{
			Id:     block.Id,
			Type:   block.Type,
			Author: block.Author,
			Time:   block.Time,
			Data: &pb.Data{
				Text:           block.Data.Text,
				Level:          block.Data.Level,
				ListType:       block.Data.ListType,
				WithBorder:     block.Data.WithBorder,
				WithBackground: block.Data.WithBackground,
				Stretched:      block.Data.Stretched,
				Caption:        block.Data.Caption,
			},
		}

		if block.Data.File != nil {
			protoBlock.Data.File = &pb.File{
				Url:       block.Data.File.Url,
				Size:      block.Data.File.Size,
				Name:      block.Data.File.Name,
				Extension: block.Data.File.Extension,
			}
		}

		protoBlocks = append(protoBlocks, protoBlock)
	}

	return protoBlocks
}

// Helper function to map the incoming JSON nodes to Protobuf nodes (Plate.js)
func mapNodesToProto(nodes []NodeInput) []*pb.PlateNode {
	var protoNodes []*pb.PlateNode

	for _, node := range nodes {
		protoNode := &pb.PlateNode{
			Type:       node.Type,
			Text:       node.Text,
			Attributes: node.Attributes,
			Children:   mapNodesToProto(node.Children), // Recursively map child nodes
		}

		protoNodes = append(protoNodes, protoNode)
	}

	return protoNodes
}

// Incoming JSON structure for blocks (Editor.js)
type BlockInput struct {
	Id     string    `json:"id"`
	Type   string    `json:"type"`
	Author []string  `json:"author"`
	Time   int64     `json:"time"`
	Data   DataInput `json:"data"`
}

type DataInput struct {
	Text           string     `json:"text"`
	Level          int32      `json:"level"`
	ListType       string     `json:"list_type"`
	WithBorder     bool       `json:"withBorder"`
	WithBackground bool       `json:"withBackground"`
	Stretched      bool       `json:"stretched"`
	Caption        string     `json:"caption"`
	File           *FileInput `json:"file"`
}

type FileInput struct {
	Url       string `json:"url"`
	Size      int32  `json:"size"`
	Name      string `json:"name"`
	Extension string `json:"extension"`
}

// Incoming JSON structure for nodes (Plate.js)
type NodeInput struct {
	Type       string            `json:"type"`
	Text       string            `json:"text"`
	Attributes map[string]string `json:"attributes"`
	Children   []NodeInput       `json:"children"`
}

// BlogInput handles both Editor.js blocks and Plate.js nodes
type BlogInput struct {
	Time   int64        `json:"time"`
	Blocks []BlockInput `json:"blocks"` // For Editor.js content
	Nodes  []NodeInput  `json:"nodes"`  // For Plate.js content
}
