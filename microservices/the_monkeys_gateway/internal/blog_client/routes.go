package blog_client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_blog/pb"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/internal/auth"
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
}

func NewBlogServiceClient(cfg *config.Config) pb.BlogServiceClient {
	cc, err := grpc.Dial(cfg.Microservices.TheMonkeysBlog, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logrus.Errorf("cannot dial to blog server: %v", err)
	}

	logrus.Infof("✅ the monkeys gateway is dialing to the blog rpc server at: %v", cfg.Microservices.TheMonkeysBlog)
	return pb.NewBlogServiceClient(cc)
}

func RegisterBlogRouter(router *gin.Engine, cfg *config.Config, authClient *auth.ServiceClient) *BlogServiceClient {
	mware := auth.InitAuthMiddleware(authClient)

	blogClient := &BlogServiceClient{
		Client: NewBlogServiceClient(cfg),
	}
	routes := router.Group("/api/v1/blog")
	routes.GET("/latest", blogClient.GetLatest100Blogs)
	routes.GET("/:blog_id", blogClient.GetPublishedBlogById)
	routes.GET("/tags", blogClient.GetBlogsByTagsName)
	routes.GET("/news1", blogClient.GetNews1)
	routes.GET("/news2", blogClient.GetNews2)
	routes.GET("/news3", blogClient.GetNews3)

	// Use AuthRequired for basic authorization
	routes.Use(mware.AuthRequired)
	routes.GET("/draft/:blog_id", blogClient.DraftABlog)

	// Use AuthzRequired for routes needing access control
	routes.POST("/publish/:blog_id", mware.AuthzRequired, blogClient.PublishBlogById)
	routes.POST("/archive/:blog_id", mware.AuthzRequired, blogClient.ArchiveBlogById)
	// routes.DELETE("/delete/:id", blogClient.DeleteBlogById)
	routes.GET("/all/drafts/:acc_id", mware.AuthzRequired, blogClient.AllDrafts)
	routes.GET("/all/drafts/:acc_id/:blog_id", mware.AuthzRequired, blogClient.GetDraftBlogByAccId)
	routes.GET("/all/published/:acc_id/:blog_id", mware.AuthzRequired, blogClient.GetPublishedBlogByAccId)

	return blogClient
}

func (asc *BlogServiceClient) DraftABlog(ctx *gin.Context) {
	id := ctx.Param("blog_id")

	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		logrus.Errorf("error upgrading connection: %v", err)
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
		var draftBlog *pb.DraftBlogRequest
		err = json.Unmarshal(msg, &draftBlog)
		if err != nil {
			logrus.Errorf("Error un marshalling message: %v", err)
			return
		}

		draftBlog.BlogId = id

		resp, err := asc.Client.DraftBlog(context.Background(), draftBlog)
		if err != nil {
			logrus.Errorf("error while creating draft blog: %v", err)
			return
		}

		response, err := json.Marshal(resp)
		if err != nil {
			logrus.Println("Error un marshalling response message:", err)
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
	accId := ctx.Param("acc_id")

	res, err := asc.Client.GetDraftBlogs(context.Background(), &pb.GetDraftBlogsReq{
		AccountId: accId,
		Email:     "",
		Username:  "",
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
	// Extract account_id and blog_id from URL parameters
	accID := ctx.Param("acc_id")
	blogID := ctx.Param("blog_id")

	// Ensure acc_id and blog_id are not empty
	if accID == "" || blogID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "account id and blog id are required"})
		return
	}

	// Fetch the drafted blog by blog_id and owner_account_id
	blog, err := asc.Client.GetDraftBlogById(ctx, &pb.GetBlogByIdReq{
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
	blog, err := asc.Client.GetPublishedBlogByIdAndOwnerId(ctx, &pb.GetBlogByIdReq{
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
	id := ctx.Param("blog_id")
	resp, err := asc.Client.PublishBlog(context.Background(), &pb.PublishBlogReq{
		BlogId: id,
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

	res, err := svc.Client.GetPublishedBlogById(context.Background(), &pb.GetBlogByIdReq{BlogId: id})
	if err != nil {
		logrus.Errorf("cannot get the blog, error: %v", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot get the blogs"})
		return
	}

	ctx.JSON(http.StatusCreated, res)
}

func (asc *BlogServiceClient) ArchiveBlogById(ctx *gin.Context) {
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
				ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "the blogs do not exist"})
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

// func (asc *BlogServiceClient) DeleteBlogById(ctx *gin.Context) {
// 	ctx.JSON(http.StatusOK, map[string]string{"message": "This api is not implemented!"})
// }

// ******************************************************* Third Party API ************************************************

type NewsResponse struct {
	Data interface{} `json:"data"`
}

const apiURL = "http://api.mediastack.com/v1/news?access_key=0eb15d25302a5df61462633e05c3cc0f&language=en&categories=business,entertainment,sports,science,technology&limit=100"

func (svc *BlogServiceClient) GetNews1(ctx *gin.Context) {
	svc.cacheMutex.Lock()
	defer svc.cacheMutex.Unlock()

	// Check if cache is valid
	if time.Now().Format("2006-01-02") == svc.cacheTime.Format("2006-01-02") && svc.cache != "" {
		ctx.Data(http.StatusOK, "application/json", []byte(svc.cache))
		return
	}

	// Call the API
	resp, err := http.Get(apiURL)
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

const apiURL2 = "https://newsapi.org/v2/everything?domains=techcrunch.com,thenextweb.com&apiKey=1e59062cc9314effacf2e37e2fcaaab8&language=en"

func (svc *BlogServiceClient) GetNews2(ctx *gin.Context) {
	svc.cacheMutex.Lock()
	defer svc.cacheMutex.Unlock()

	// Check if cache1 is valid
	if time.Now().Format("2006-01-02") == svc.cacheTime.Format("2006-01-02") && svc.cache1 != "" {
		ctx.Data(http.StatusOK, "application/json", []byte(svc.cache1))
		return
	}

	// Call the API
	resp, err := http.Get(apiURL2)
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
