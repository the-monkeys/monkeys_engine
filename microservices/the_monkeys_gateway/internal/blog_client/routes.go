package blog_client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
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
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/middleware"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

var UserIpMap = map[string]string{}

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
	rateLimiter := middleware.RateLimiterMiddleware("1-S") // 10 requests per second for mins do 1-M

	mware := auth.InitAuthMiddleware(authClient)

	blogClient := &BlogServiceClient{
		Client:  NewBlogServiceClient(cfg),
		userCli: userClient,
		config:  cfg,
	}
	routes := router.Group("/api/v1/blog")
	routes.GET("/latest", rateLimiter, blogClient.GetLatest100Blogs)
	routes.GET("/:blog_id", blogClient.GetPublishedBlogById)
	routes.POST("/tags", blogClient.GetBlogsByTagsName)
	routes.GET("/all/publishes/:username", blogClient.AllPublishesByUserName)
	routes.GET("/published/:acc_id/:blog_id", blogClient.GetPublishedBlogByAccId)
	routes.GET("/news1", rateLimiter, blogClient.GetNews1)
	routes.GET("/news2", rateLimiter, blogClient.GetNews2)
	routes.GET("/news3", rateLimiter, blogClient.GetNews3)

	// Use AuthRequired for basic authorization
	routes.Use(mware.AuthRequired)

	// Use AuthzRequired for routes needing access control
	routes.GET("/draft/:blog_id", mware.AuthzRequired, blogClient.DraftABlog)

	routes.POST("/publish/:blog_id", mware.AuthzRequired, blogClient.PublishBlogById)
	routes.POST("/archive/:blog_id", mware.AuthzRequired, blogClient.ArchiveBlogById)
	routes.GET("/all/drafts/:acc_id", blogClient.AllDrafts)
	routes.GET("/all-col/:acc_id", blogClient.AllCollabBlogs)
	routes.GET("/drafts/:acc_id/:blog_id", mware.AuthzRequired, blogClient.GetDraftBlogByAccId)
	// routes.GET("/all/publishes/:acc_id", blogClient.AllPublishesByAccountId)

	routes.GET("/my-drafts/:blog_id", mware.AuthzRequired, blogClient.GetDraftBlogByBlogId)

	routes.GET("/all/bookmarks", blogClient.GetBookmarks)

	routes.DELETE("/:blog_id", mware.AuthzRequired, blogClient.DeleteBlogById)

	// -------------------------------------------------- V2 --------------------------------------------------
	routesV2 := router.Group("/api/v2/blog")

	// Test Apis
	{
		routesV2.GET("/get-ips", rateLimiter, blogClient.GetIps)
	}
	// Public APIs
	{
		// Get all blogs
		routesV2.GET("/feed", rateLimiter, blogClient.GetLatestBlogs) // Get all blogs, latest first with limit and offset
		// Get blogs by tags, as users can filter the blogs using multiple tags
		routesV2.POST("/tags", rateLimiter, blogClient.GetBlogsByTags) // Get blogs by tags
		// Get blogs by username, not auth required as it is public and can be visible at users profile
		routesV2.GET("/all/:username", rateLimiter, blogClient.UsersBlogs) // Update of blogClient.AllPublishesByUserName
		// Get published blog by blog_id
		routesV2.GET("/:blog_id", rateLimiter, blogClient.GetPublishedBlogByBlogId) // Get published blog by blog_id
	}

	routesV2.Use(mware.AuthRequired)

	// Protected APIs
	{
		// Get blogs of following users
		routesV2.GET("/following", rateLimiter, blogClient.FollowingBlogsFeed) // Blogs for following feed
		// User can get their blogs (draft)
		routesV2.GET("/my-drafts", rateLimiter, blogClient.MyDraftBlogs) // Get all my draft blogs
		// Users can get their blogs (published)
		routesV2.GET("/my-published", rateLimiter, blogClient.MyPublishedBlogs) // Get all my published blogs
		// Users can get the blogs they bookmarked (published)
		routesV2.GET("/my-bookmarks", rateLimiter, blogClient.MyBookmarks) // Update of blogClient.GetBookmarks
		// My feed blogs, contains blogs from people I follow + my own blogs + topics I follow
		// routesV2.GET("/my-feed", blogClient.MyFeedBlogs) // Get my feed blogs
	}

	// Authorization required APIs
	{
		// Write a blog, when the user have edit access
		routesV2.GET("/draft/:blog_id", mware.AuthzRequired, blogClient.WriteBlog)
		// TODO: Add api to /to-publish/:blog_id now v1  blogClient.PublishBlogById is working
		routesV2.POST("/to-draft/:blog_id", mware.AuthzRequired, blogClient.MoveBlogToDraft)
		// Get my draft blog by id
		routesV2.GET("/my-draft/:blog_id", mware.AuthzRequired, blogClient.GetDraftBlogByBlogIdV2)
	}

	return blogClient
}

func (asc *BlogServiceClient) DraftABlog(ctx *gin.Context) {
	id := ctx.Param("blog_id")

	// logrus.Infof("Traffic is coming from IP: %v", ctx.ClientIP())
	ipAddress := ctx.Request.Header.Get("IP")
	client := ctx.Request.Header.Get("Client")

	// Check if the blog exists
	resp, err := asc.Client.CheckIfBlogsExist(context.Background(), &pb.BlogByIdReq{
		BlogId: id,
	})
	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.InvalidArgument:
				ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "Incomplete request, please provide correct input parameters"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Cannot fetch the draft blogs"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Unknown error"})
				return
			}
		}
	}

	var action string
	var initialLogDone bool

	if resp.BlogExists {
		if !utils.CheckUserAccessInContext(ctx, constants.PermissionEdit) {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "You are not allowed to perform this action"})
			return
		}
		action = constants.BLOG_UPDATE
	} else {
		action = constants.BLOG_CREATE
	}

	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		logrus.Errorf("Error upgrading connection: %v", err)
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	// Infinite loop to listen to WebSocket connection
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			logrus.Errorf("Error reading the message: %v", err)
			return
		}

		// Save the incoming message for debugging purposes
		// os.WriteFile("draft.json", msg, 0644)

		// Step 1: Unmarshal into a generic map
		var genericMap map[string]interface{}
		err = json.Unmarshal(msg, &genericMap)
		if err != nil {
			logrus.Errorf("Error unmarshalling message into generic map: %v", err)
			return
		}

		// Step 2: Process "blog.blocks" for table-specific data
		if blog, ok := genericMap["blog"].(map[string]interface{}); ok {
			if blocks, ok := blog["blocks"].([]interface{}); ok {
				for _, block := range blocks {
					blockMap, ok := block.(map[string]interface{})
					if !ok {
						continue
					}

					// Process "table" blocks
					if blockMap["type"] == "table" {
						data, ok := blockMap["data"].(map[string]interface{})
						if !ok {
							continue
						}

						// if withBorder, ok := data["withBorder"].([]interface{}); ok {
						// 	withBorderBool, ok := withBorder[0].(bool)
						// }

						// Transform "content" into pb.TableRow structure
						if content, ok := data["content"].([]interface{}); ok {
							var tableContent []*pb.TableRow
							for _, row := range content {
								rowSlice, ok := row.([]interface{})
								if !ok {
									continue
								}
								var cells []string
								for _, cell := range rowSlice {
									if cellStr, ok := cell.(string); ok {
										cells = append(cells, cellStr)
									}
								}
								tableContent = append(tableContent, &pb.TableRow{Cells: cells})
							}
							data["table_content"] = tableContent
							delete(data, "content") // Remove the original field
						}
					}
				}
			}
		}

		// Step 3: Marshal back into JSON
		updatedJSON, err := json.Marshal(genericMap)
		if err != nil {
			logrus.Errorf("Error marshalling updated JSON: %v", err)
			return
		}

		// Step 4: Unmarshal into pb.DraftBlogRequest
		var draftBlog pb.DraftBlogRequest
		err = json.Unmarshal(updatedJSON, &draftBlog)
		if err != nil {
			logrus.Errorf("Error unmarshalling updated JSON into pb.DraftBlogRequest: %v", err)
			return
		}

		draftBlog.BlogId = id
		draftBlog.Ip = ipAddress
		draftBlog.Client = client

		// Only set the action and log the initial creation or update once
		if !initialLogDone {
			draftBlog.Action = action
			initialLogDone = true
		}

		// Send the draft blog to the gRPC service
		resp, err := asc.Client.DraftBlog(context.Background(), &draftBlog)
		if err != nil {
			logrus.Errorf("Error while creating draft blog: %v", err)
			return
		}

		// Marshal and send the response back to the WebSocket client
		response, err := json.Marshal(resp)
		if err != nil {
			logrus.Errorf("Error marshalling response message: %v", err)
			return
		}

		if err := conn.WriteMessage(websocket.TextMessage, response); err != nil {
			logrus.Errorf("Error returning the response message: %v", err)
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
		Ip:        ctx.Request.Header.Get("IP"),
		Client:    ctx.Request.Header.Get("Client"),
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

// -------------------------------------------------- V2 --------------------------------------------------

func (asc *BlogServiceClient) WriteBlog(ctx *gin.Context) {
	id := ctx.Param("blog_id")

	ipAddress := ctx.Request.Header.Get("IP")
	client := ctx.Request.Header.Get("Client")

	// Check if the blog exists
	resp, err := asc.Client.CheckIfBlogsExist(context.Background(), &pb.BlogByIdReq{
		BlogId: id,
	})
	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.InvalidArgument:
				ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "Incomplete request, please provide correct input parameters"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Cannot fetch the draft blogs"})
				return
			default:

			}
		}
	}

	var action string
	var initialLogDone bool

	if resp.BlogExists {
		if !utils.CheckUserAccessInContext(ctx, constants.PermissionEdit) {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "You are not allowed to perform this action"})
			return
		}
		action = constants.BLOG_UPDATE
	} else {
		action = constants.BLOG_CREATE
	}

	if !resp.IsDraft {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "you need to move the blog to draft to edit it"})
		return
	}

	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		logrus.Errorf("Error upgrading connection: %v", err)
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	// Establish a bi-directional stream with the gRPC server
	stream, err := asc.Client.DraftBlogV2(context.Background())
	if err != nil {
		logrus.Errorf("Error establishing gRPC stream: %v", err)
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	defer stream.CloseSend()

	// Infinite loop to listen to WebSocket connection
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			logrus.Errorf("Error reading the message: %v", err)
			return
		}

		// Save the incoming message for debugging purposes
		// os.WriteFile("draft.json", msg, 0644)

		// Step 1: Unmarshal into a generic map
		var genericMap map[string]interface{}
		err = json.Unmarshal(msg, &genericMap)
		if err != nil {
			logrus.Errorf("Error unmarshalling message into generic map: %v", err)
			return
		}

		// Step 3: Marshal back into JSON
		updatedJSON, err := json.Marshal(genericMap)
		if err != nil {
			logrus.Errorf("Error marshalling updated JSON: %v", err)
			return
		}

		// Step 4: Unmarshal into pb.DraftBlogRequest
		var draftBlog map[string]interface{}
		err = json.Unmarshal(updatedJSON, &draftBlog)
		if err != nil {
			logrus.Errorf("Error unmarshalling updated JSON into pb.DraftBlogRequest: %v", err)
			return
		}

		draftBlog["blog_id"] = id
		draftBlog["Ip"] = ipAddress
		draftBlog["Client"] = client

		// Only set the action and log the initial creation or update once
		if !initialLogDone {
			draftBlog["Action"] = action
			initialLogDone = true
		}

		// Convert draftBlog to google.protobuf.Any
		draftStruct, err := structpb.NewStruct(draftBlog)
		if err != nil {
			logrus.Errorf("Error converting draftBlog to Any: %v", err)
			return
		}

		// Wrap *structpb.Struct in *anypb.Any
		anyMsg, err := anypb.New(draftStruct)
		if err != nil {
			logrus.Errorf("Error wrapping structpb.Struct in anypb.Any: %v", err)
			return
		}

		// Send the draft blog to the gRPC service
		if err := stream.Send(anyMsg); err != nil {
			logrus.Errorf("Error sending draft blog to gRPC stream: %v", err)
			return
		}

		// Receive the response from the gRPC service
		resp, err := stream.Recv()
		if err != nil {
			logrus.Errorf("Error receiving response from gRPC stream: %v", err)
			return
		}

		// Marshal and send the response back to the WebSocket client
		response, err := json.Marshal(resp)
		if err != nil {
			logrus.Errorf("Error marshalling response message: %v", err)
			return
		}

		if err := conn.WriteMessage(websocket.TextMessage, response); err != nil {
			logrus.Errorf("Error returning the response message: %v", err)
			return
		}
	}
}

func (asc *BlogServiceClient) FollowingBlogsFeed(ctx *gin.Context) {
	myUsername := ctx.GetString("userName")
	accountID := ctx.GetString("accountId")
	// Get Accounts I am following
	followings, err := asc.userCli.GetFollowingAccounts(myUsername)
	if err != nil {
		logrus.Errorf("cannot get the following accounts, error: %v", err)
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "cannot find the following accounts"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot get the following accounts"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
	}

	if len(followings.Users) == 0 {
		ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "you are not following anyone"})
		return
	}

	accountIds := []string{accountID}

	for _, user := range followings.Users {
		accountIds = append(accountIds, user.AccountId)
	}

	limit := ctx.DefaultQuery("limit", "100")
	offset := ctx.DefaultQuery("offset", "0")
	// Convert to int
	limitInt, err := strconv.Atoi(limit)
	if err != nil {
		limitInt = 100
	}

	offsetInt, err := strconv.Atoi(offset)
	if err != nil {
		offsetInt = 0
	}

	// Get all the drafted blogs based on the following accounts with limit and offset
	stream, err := asc.Client.BlogsOfFollowingAccounts(context.Background(), &pb.FollowingAccounts{
		AccountIds: accountIds,
		Limit:      int32(limitInt),
		Offset:     int32(offsetInt),
	})

	if err != nil {
		logrus.Errorf("cannot get the following blogs, error: %v", err)
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "cannot find the following blogs"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot get the following blogs"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
	}

	var allBlogs []map[string]interface{}
	for {
		blog, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			if status, ok := status.FromError(err); ok {
				switch status.Code() {
				case codes.NotFound:
					ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "no blogs found from people you are following"})
					return
				case codes.Internal:
					ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "error receiving blog from stream"})
					return
				default:
					ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
					return
				}
			}
		}

		var blogMaps []map[string]interface{}
		if err := json.Unmarshal(blog.Value, &blogMaps); err != nil {
			logrus.Errorf("cannot unmarshal the blog, error: %v", err)
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot unmarshal the blog"})
			return
		}
		allBlogs = append(allBlogs, blogMaps...)
	}

	for _, blog := range allBlogs {
		blogID, ok := blog["blog_id"].(string)
		if !ok {
			logrus.Errorf("BlogId is either missing or not a string: %v", blog)
			continue
		}

		likeCount, _ := asc.userCli.GetNoOfLikeCounts(blogID)
		blog["LikeCount"] = likeCount

		isLikedByMe, _ := asc.userCli.HaveILikedTheBlog(blogID, myUsername)
		blog["IsLikedByMe"] = isLikedByMe

		bookmarkCount, _ := asc.userCli.GetNoOfBookmarkCounts(blogID)
		blog["BookmarkCount"] = bookmarkCount

		isBookmarkedByMe, _ := asc.userCli.HaveIBookmarkedTheBlog(blogID, myUsername)
		blog["IsBookmarkedByMe"] = isBookmarkedByMe
	}

	responseBlogs := map[string]interface{}{
		"blogs": allBlogs,
	}

	ctx.JSON(http.StatusOK, responseBlogs)
}

// TODO: Delete this route
func (asc *BlogServiceClient) GetIps(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, UserIpMap)
}

func (asc *BlogServiceClient) GetLatestBlogs(ctx *gin.Context) {
	// Check if the file exists, if not create a new one and add ctx.ClientIP() in json
	// go utils.GetClientIP(ctx)
	accuntID := ctx.GetString("accountId")
	if accuntID == "" {
		UserIpMap[ctx.ClientIP()] = ctx.ClientIP()
	} else {
		UserIpMap[accuntID] = ctx.ClientIP()
	}

	// Get Limits and offset
	limit := ctx.DefaultQuery("limit", "100")
	offset := ctx.DefaultQuery("offset", "0")
	// Convert to int
	limitInt, err := strconv.Atoi(limit)
	if err != nil {
		limitInt = 100
	}

	offsetInt, err := strconv.Atoi(offset)
	if err != nil {
		offsetInt = 0
	}

	stream, err := asc.Client.GetFeedBlogs(context.Background(), &pb.FeedReq{
		Limit:  int32(limitInt),
		Offset: int32(offsetInt),
	})

	if err != nil {
		logrus.Errorf("cannot get the blogs by tags, error: %v", err)
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "cannot find the blogs for the given tags"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot get the blogs by tags"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
	}

	var allBlogs []map[string]interface{}
	for {
		blog, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			if status, ok := status.FromError(err); ok {
				switch status.Code() {
				case codes.NotFound:
					ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "no blogs found for the given tags"})
					return
				case codes.Internal:
					ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "error receiving blog from stream"})
					return
				default:
					ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
					return
				}
			}
		}

		var blogMaps []map[string]interface{}
		if err := json.Unmarshal(blog.Value, &blogMaps); err != nil {
			logrus.Errorf("cannot unmarshal the blog, error: %v", err)
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot unmarshal the blog"})
			return
		}
		allBlogs = append(allBlogs, blogMaps...)
	}

	for _, blog := range allBlogs {
		blogID, ok := blog["blog_id"].(string)
		if !ok {
			logrus.Errorf("BlogId is either missing or not a string: %v", blog)
			continue
		}

		likeCount, _ := asc.userCli.GetNoOfLikeCounts(blogID)
		blog["like_count"] = likeCount

		bookmarkCount, _ := asc.userCli.GetNoOfBookmarkCounts(blogID)
		blog["bookmark_count"] = bookmarkCount

	}

	responseBlogs := map[string]interface{}{
		"blogs": allBlogs,
	}

	ctx.JSON(http.StatusOK, responseBlogs)
}

func (asc *BlogServiceClient) GetBlogsByTags(ctx *gin.Context) {
	tags := Tags{}
	if err := ctx.BindJSON(&tags); err != nil {
		logrus.Errorf("error while marshalling tags: %v", err)
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "tags aren't properly formatted"})
		return
	}

	// Get Limits and offset
	limit := ctx.DefaultQuery("limit", "100")
	offset := ctx.DefaultQuery("offset", "0")
	// Convert to int
	limitInt, err := strconv.Atoi(limit)
	if err != nil {
		limitInt = 100
	}

	offsetInt, err := strconv.Atoi(offset)
	if err != nil {
		offsetInt = 0
	}

	stream, err := asc.Client.GetBlogs(context.Background(), &pb.GetBlogsReq{
		IsDraft: false,
		Tags:    tags.Tags,
		Limit:   int32(limitInt),
		Offset:  int32(offsetInt),
	})

	if err != nil {
		logrus.Errorf("cannot get the blogs by tags, error: %v", err)
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "cannot find the blogs for the given tags"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot get the blogs by tags"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
	}

	var allBlogs []map[string]interface{}
	for {
		blog, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			if status, ok := status.FromError(err); ok {
				switch status.Code() {
				case codes.NotFound:
					ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "no blogs found for the given tags"})
					return
				case codes.Internal:
					ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "error receiving blog from stream"})
					return
				default:
					ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
					return
				}
			}
		}

		var blogMaps []map[string]interface{}
		if err := json.Unmarshal(blog.Value, &blogMaps); err != nil {
			logrus.Errorf("cannot unmarshal the blog, error: %v", err)
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot unmarshal the blog"})
			return
		}
		allBlogs = append(allBlogs, blogMaps...)
	}

	for _, blog := range allBlogs {
		blogID, ok := blog["blog_id"].(string)
		if !ok {
			logrus.Errorf("BlogId is either missing or not a string: %v", blog)
			continue
		}

		likeCount, _ := asc.userCli.GetNoOfLikeCounts(blogID)
		blog["like_count"] = likeCount

		bookmarkCount, _ := asc.userCli.GetNoOfBookmarkCounts(blogID)
		blog["bookmark_count"] = bookmarkCount

	}

	responseBlogs := map[string]interface{}{
		"blogs": allBlogs,
	}

	ctx.JSON(http.StatusOK, responseBlogs)
}

func (asc *BlogServiceClient) MyDraftBlogs(ctx *gin.Context) {
	tokenAccountId := ctx.GetString("accountId")

	stream, err := asc.Client.GetBlogs(context.Background(), &pb.GetBlogsReq{
		AccountId: tokenAccountId,
		IsDraft:   true,
		Limit:     5,
		Offset:    0,
	})

	if err != nil {
		logrus.Errorf("cannot get the blogs by tags, error: %v", err)
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "cannot find the blogs for the given tags"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot get the blogs by tags"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
	}

	var allBlogs []map[string]interface{}
	for {
		blog, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			if status, ok := status.FromError(err); ok {
				switch status.Code() {
				case codes.NotFound:
					ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "no blogs found for the given tags"})
					return
				case codes.Internal:
					ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "error receiving blog from stream"})
					return
				default:
					ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
					return
				}
			}
		}

		var blogMaps []map[string]interface{}
		if err := json.Unmarshal(blog.Value, &blogMaps); err != nil {
			logrus.Errorf("cannot unmarshal the blog, error: %v", err)
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot unmarshal the blog"})
			return
		}
		allBlogs = append(allBlogs, blogMaps...)
	}

	for _, blog := range allBlogs {
		blogID, ok := blog["blog_id"].(string)
		if !ok {
			logrus.Errorf("BlogId is either missing or not a string: %v", blog)
			continue
		}

		likeCount, _ := asc.userCli.GetNoOfLikeCounts(blogID)
		blog["LikeCount"] = likeCount

		bookmarkCount, _ := asc.userCli.GetNoOfBookmarkCounts(blogID)
		blog["BookmarkCount"] = bookmarkCount

	}

	responseBlogs := map[string]interface{}{
		"blogs": allBlogs,
	}

	ctx.JSON(http.StatusOK, responseBlogs)
}

func (asc *BlogServiceClient) MyPublishedBlogs(ctx *gin.Context) {
	tokenAccountId := ctx.GetString("accountId")

	// Get limit and offset and convert into int32
	limit := ctx.DefaultQuery("limit", "10")
	offset := ctx.DefaultQuery("offset", "0")
	limitInt, err := strconv.Atoi(limit)
	if err != nil {
		limitInt = 10
	}

	offsetInt, err := strconv.Atoi(offset)
	if err != nil {
		offsetInt = 0
	}

	stream, err := asc.Client.GetBlogs(context.Background(), &pb.GetBlogsReq{
		AccountId: tokenAccountId,
		IsDraft:   false,
		Limit:     int32(limitInt),
		Offset:    int32(offsetInt),
	})

	if err != nil {
		logrus.Errorf("cannot get the blogs by tags, error: %v", err)
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "cannot find the blogs for the given tags"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot get the blogs by tags"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
	}

	var allBlogs []map[string]interface{}
	for {
		blog, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			if status, ok := status.FromError(err); ok {
				switch status.Code() {
				case codes.NotFound:
					ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "no blogs found for the given tags"})
					return
				case codes.Internal:
					ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "error receiving blog from stream"})
					return
				default:
					ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
					return
				}
			}
		}

		var blogMaps []map[string]interface{}
		if err := json.Unmarshal(blog.Value, &blogMaps); err != nil {
			logrus.Errorf("cannot unmarshal the blog, error: %v", err)
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot unmarshal the blog"})
			return
		}
		allBlogs = append(allBlogs, blogMaps...)
	}

	for _, blog := range allBlogs {
		blogID, ok := blog["blog_id"].(string)
		if !ok {
			logrus.Errorf("BlogId is either missing or not a string: %v", blog)
			continue
		}

		likeCount, _ := asc.userCli.GetNoOfLikeCounts(blogID)
		blog["LikeCount"] = likeCount

		bookmarkCount, _ := asc.userCli.GetNoOfBookmarkCounts(blogID)
		blog["BookmarkCount"] = bookmarkCount

	}

	responseBlogs := map[string]interface{}{
		"blogs": allBlogs,
	}

	ctx.JSON(http.StatusOK, responseBlogs)
}

func (asc *BlogServiceClient) UsersBlogs(ctx *gin.Context) {
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

	// Get limit and offset and convert into int32
	limit := ctx.DefaultQuery("limit", "10")
	offset := ctx.DefaultQuery("offset", "0")
	limitInt, err := strconv.Atoi(limit)
	if err != nil {
		limitInt = 10
	}

	offsetInt, err := strconv.Atoi(offset)
	if err != nil {
		offsetInt = 0
	}

	stream, err := asc.Client.GetBlogs(context.Background(), &pb.GetBlogsReq{
		AccountId: userInfo.AccountId,
		IsDraft:   false,
		Limit:     int32(limitInt),
		Offset:    int32(offsetInt),
	})

	if err != nil {
		logrus.Errorf("cannot get the blogs by tags, error: %v", err)
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "cannot find the blogs for the given tags"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot get the blogs by tags"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
	}

	var allBlogs []map[string]interface{}
	for {
		blog, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			if status, ok := status.FromError(err); ok {
				switch status.Code() {
				case codes.NotFound:
					ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "no blogs found for the given tags"})
					return
				case codes.Internal:
					ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "error receiving blog from stream"})
					return
				default:
					ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
					return
				}
			}
		}

		var blogMaps []map[string]interface{}
		if err := json.Unmarshal(blog.Value, &blogMaps); err != nil {
			logrus.Errorf("cannot unmarshal the blog, error: %v", err)
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot unmarshal the blog"})
			return
		}
		allBlogs = append(allBlogs, blogMaps...)
	}

	for _, blog := range allBlogs {
		blogID, ok := blog["blog_id"].(string)
		if !ok {
			logrus.Errorf("BlogId is either missing or not a string: %v", blog)
			continue
		}

		likeCount, _ := asc.userCli.GetNoOfLikeCounts(blogID)
		blog["LikeCount"] = likeCount

		bookmarkCount, _ := asc.userCli.GetNoOfBookmarkCounts(blogID)
		blog["BookmarkCount"] = bookmarkCount

	}

	responseBlogs := map[string]interface{}{
		"blogs": allBlogs,
	}

	ctx.JSON(http.StatusOK, responseBlogs)
}

func (asc *BlogServiceClient) MoveBlogToDraft(ctx *gin.Context) {
	// Check permissions:
	if !utils.CheckUserAccessInContext(ctx, constants.PermissionEdit) {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "you are not allowed to perform this action"})
		return
	}

	accId := ctx.GetString("accountId")

	id := ctx.Param("blog_id")
	resp, err := asc.Client.MoveBlogToDraftStatus(context.Background(), &pb.BlogReq{
		BlogId:    id,
		AccountId: accId,
		Ip:        ctx.Request.Header.Get("IP"),
		Client:    ctx.Request.Header.Get("Client"),
	})

	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "the blog does not exist"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot move the blog to draft"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
	}

	ctx.JSON(http.StatusOK, resp)
}

func (asc *BlogServiceClient) MyBookmarks(ctx *gin.Context) {
	tokenAccountId := ctx.GetString("userName")

	// Get limit and offset and convert into int32
	limit := ctx.DefaultQuery("limit", "10")
	offset := ctx.DefaultQuery("offset", "0")
	limitInt, err := strconv.Atoi(limit)
	if err != nil {
		limitInt = 10
	}

	offsetInt, err := strconv.Atoi(offset)
	if err != nil {
		offsetInt = 0
	}

	// Get all the draft blogs by my username
	blogResp, err := asc.userCli.GetUsersBookmarks(tokenAccountId)

	if err != nil {
		logrus.Errorf("cannot get the bookmarks, error: %v", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot get the bookmarks"})
		return
	}

	if len(blogResp) == 0 {
		ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "no blogs found"})
		return
	}

	stream, err := asc.Client.GetBlogsBySlice(context.Background(), &pb.GetBlogsBySliceReq{
		BlogIds: blogResp,
		IsDraft: false,
		Limit:   int32(limitInt),
		Offset:  int32(offsetInt),
	})

	if err != nil {
		logrus.Errorf("cannot get the blogs by tags, error: %v", err)
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "cannot find the blogs for the given tags"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot get the blogs by tags"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
	}

	var allBlogs []map[string]interface{}
	for {
		blog, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			if status, ok := status.FromError(err); ok {
				switch status.Code() {
				case codes.NotFound:
					ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "no blogs found for the given tags"})
					return
				case codes.Internal:
					ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "error receiving blog from stream"})
					return
				default:
					ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
					return
				}
			}
		}

		var blogMaps []map[string]interface{}
		if err := json.Unmarshal(blog.Value, &blogMaps); err != nil {
			logrus.Errorf("cannot unmarshal the blog, error: %v", err)
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot unmarshal the blog"})
			return
		}
		allBlogs = append(allBlogs, blogMaps...)
	}

	for _, blog := range allBlogs {
		blogID, ok := blog["blog_id"].(string)
		if !ok {
			logrus.Errorf("BlogId is either missing or not a string: %v", blog)
			continue
		}

		likeCount, _ := asc.userCli.GetNoOfLikeCounts(blogID)
		blog["LikeCount"] = likeCount

		bookmarkCount, _ := asc.userCli.GetNoOfBookmarkCounts(blogID)
		blog["BookmarkCount"] = bookmarkCount

	}

	responseBlogs := map[string]interface{}{
		"blogs": allBlogs,
	}

	ctx.JSON(http.StatusOK, responseBlogs)
}

func (asc *BlogServiceClient) GetPublishedBlogByBlogId(ctx *gin.Context) {
	blogId := ctx.Param("blog_id")

	resp, err := asc.Client.GetBlog(context.Background(), &pb.BlogReq{
		BlogId:  blogId,
		IsDraft: false,
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

	var blogMap map[string]interface{}
	if err := json.Unmarshal(resp.Value, &blogMap); err != nil {
		logrus.Errorf("cannot unmarshal the blog, error: %v", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot unmarshal the blog"})
		return
	}

	// Initialize the map if it is nil
	if blogMap == nil {
		// blogMap = make(map[string]interface{})
		ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "the blog does not exist"})
		return
	}

	likeCount, _ := asc.userCli.GetNoOfLikeCounts(blogId)
	blogMap["LikeCount"] = likeCount

	bookmarkCount, _ := asc.userCli.GetNoOfBookmarkCounts(blogId)
	blogMap["BookmarkCount"] = bookmarkCount

	ctx.JSON(http.StatusOK, blogMap)
}

func (asc *BlogServiceClient) GetDraftBlogByBlogIdV2(ctx *gin.Context) {
	blogId := ctx.Param("blog_id")

	// Check permissions:
	if !utils.CheckUserAccessInContext(ctx, constants.PermissionEdit) {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "you are not allowed to perform this action"})
		return
	}

	resp, err := asc.Client.GetBlog(context.Background(), &pb.BlogReq{
		BlogId:  blogId,
		IsDraft: true,
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

	var blogMap map[string]interface{}
	if err := json.Unmarshal(resp.Value, &blogMap); err != nil {
		logrus.Errorf("cannot unmarshal the blog, error: %v", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot unmarshal the blog"})
		return
	}

	// Initialize the map if it is nil
	if blogMap == nil {
		ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "the blog does not exist"})
		return
	}

	ctx.JSON(http.StatusOK, blogMap)
}
