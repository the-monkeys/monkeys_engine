package blog_client

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_blog/pb"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/internal/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var upgrader = websocket.Upgrader{}

type BlogServiceClient struct {
	Client pb.BlogServiceClient
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
	// routes.GET("/", blogCli.Get100Blogs)
	routes.GET("/:id", blogClient.GetBlogeById)
	// routes.GET("/tag", blogCli.Get100PostsByTags)

	routes.Use(mware.AuthRequired)
	routes.GET("/draft/:id", blogClient.DraftABlog)
	routes.POST("/publish/:id", blogClient.PublishBlogById)
	routes.POST("/archive/:id", blogClient.ArchiveBlogById)

	// routes.POST("/", blogCli.CreateABlog)
	// routes.PUT("/edit/:id", blogCli.EditArticles)
	// routes.PATCH("/edit/:id", blogCli.EditArticles)
	// routes.DELETE("/delete/:id", blogCli.DeleteBlogById)

	// Based on the editor.js APIS

	return blogClient
}

func (asc *BlogServiceClient) DraftABlog(ctx *gin.Context) {
	id := ctx.Param("id")

	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		logrus.Println(err)
		return
	}

	// Infinite loop to listen to WebSocket connection
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			logrus.Println(err)
			return
		}

		// Unmarshal the received message into the Blog struct
		var draftBlog *pb.DraftBlogRequest
		err = json.Unmarshal(msg, &draftBlog)
		if err != nil {
			logrus.Println("Error unmarshalling message:", err)
			return
		}

		draftBlog.BlogId = id

		resp, err := asc.Client.DraftBlog(context.Background(), draftBlog)
		if err != nil {
			logrus.Errorf("error while creating draft blog: %v", err)
			_ = ctx.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		response, err := json.Marshal(resp)
		if err != nil {
			logrus.Println("Error unmarshalling response message:", err)
			return
		}

		// Send a response message to the client (optional)
		err = conn.WriteMessage(websocket.TextMessage, response)
		if err != nil {
			logrus.Println(err)
			return
		}
	}
}

func (asc *BlogServiceClient) PublishBlogById(ctx *gin.Context) {
	id := ctx.Param("id")
	resp, err := asc.Client.PublishBlog(context.Background(), &pb.PublishBlogReq{
		BlogId: id,
	})

	if err != nil {
		logrus.Errorf("error while creating draft blog: %v", err)
		_ = ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, resp)
}

func (asc *BlogServiceClient) ArchiveBlogById(ctx *gin.Context) {
	id := ctx.Param("id")
	resp, err := asc.Client.ArchivehBlogById(context.Background(), &pb.ArchiveBlogReq{
		BlogId: id,
	})

	if err != nil {
		logrus.Errorf("error while creating draft blog: %v", err)
		_ = ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, resp)
}

// func (asc *BlogServiceClient) CreateABlog(ctx *gin.Context) {

// 	body := CreatePostRequestBody{}
// 	if err := ctx.BindJSON(&body); err != nil {
// 		logrus.Errorf("cannot bind json to struct, error: %v", err)
// 		_ = ctx.AbortWithError(http.StatusBadRequest, err)
// 		return
// 	}

// 	res, err := asc.Client.CreateABlog(context.Background(), &pb.CreateBlogRequest{
// 		Id:         uuid.NewString(),
// 		Title:      body.Title,
// 		Content:    body.Content,
// 		AuthorName: body.Author,
// 		AuthorId:   body.AuthorId,
// 		Published:  body.Published,
// 		Tags:       body.Tags,
// 	})

// 	if err != nil {
// 		_ = ctx.AbortWithError(http.StatusBadGateway, err)
// 		return
// 	}

// 	ctx.JSON(http.StatusAccepted, &res)

// }

// func (svc *BlogServiceClient) Get100Blogs(ctx *gin.Context) {
// 	logrus.Infof("traffic is coming from ip: %v", ctx.ClientIP())

// 	stream, err := svc.Client.Get100Blogs(context.Background(), &emptypb.Empty{})
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

func (svc *BlogServiceClient) GetBlogeById(ctx *gin.Context) {
	id := ctx.Param("id")

	res, err := svc.Client.GetBlogById(context.Background(), &pb.GetBlogByIdReq{BlogId: id})
	if err != nil {
		logrus.Errorf("cannot get the blog from rpc server, error: %v", err)
		_ = ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusCreated, res)
}

// func (blog *BlogServiceClient) EditArticles(ctx *gin.Context) {
// 	id := ctx.Param("id")

// 	reqObj := EditArticleRequestBody{}

// 	if err := ctx.BindJSON(&reqObj); err != nil {
// 		logrus.Errorf("invalid body, error: %v", err)
// 		_ = ctx.AbortWithError(http.StatusBadRequest, err)
// 		return
// 	}
// 	var isPartial bool
// 	if ctx.Request.Method == http.MethodPatch {
// 		isPartial = true
// 	}

// 	res, err := blog.Client.EditBlogById(context.Background(), &pb.EditBlogRequest{
// 		Id:        id,
// 		Title:     reqObj.Title,
// 		Content:   reqObj.Content,
// 		Tags:      reqObj.Tags,
// 		IsPartial: isPartial,
// 	})

// 	if err != nil {
// 		logrus.Errorf("cannot connect to article rpc server, error: %v", err)
// 		_ = ctx.AbortWithError(http.StatusInternalServerError, err)
// 		return
// 	}

// 	ctx.JSON(http.StatusCreated, res)
// }

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
