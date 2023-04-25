package blogsandposts

import (
	"context"
	"io"
	"net/http"

	"github.com/89minutes/the_new_project/services/api_gateway/config"
	"github.com/89minutes/the_new_project/services/api_gateway/pkg/auth"
	"github.com/89minutes/the_new_project/services/api_gateway/pkg/blogsandposts/pb"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

type BlogServiceClient struct {
	Client pb.BlogsAndPostServiceClient
}

func NewUserServiceClient(cfg *config.Address) pb.BlogsAndPostServiceClient {
	cc, err := grpc.Dial(cfg.BlogService, grpc.WithInsecure())
	if err != nil {
		logrus.Errorf("cannot dial to grpc user server: %v", err)
	}
	logrus.Infof("The Gateway is dialing to post gRPC server at: %v", cfg.BlogService)
	return pb.NewBlogsAndPostServiceClient(cc)
}

func RegisterBlogRouter(router *gin.Engine, cfg *config.Address, authClient *auth.ServiceClient) *BlogServiceClient {
	mware := auth.InitAuthMiddleware(authClient)

	blogCli := &BlogServiceClient{
		Client: NewUserServiceClient(cfg),
	}
	routes := router.Group("/api/v1/post")
	routes.GET("/", blogCli.Get100Blogs)
	routes.GET("/:id", blogCli.GetArticleById)
	routes.GET("/tag", blogCli.Get100PostsByTags)

	routes.Use(mware.AuthRequired)

	routes.POST("/", blogCli.CreateABlog)
	routes.PUT("/edit/:id", blogCli.EditArticles)
	routes.PATCH("/edit/:id", blogCli.EditArticles)
	routes.DELETE("/delete/:id", blogCli.DeleteBlogById)

	// Based on the editor.js APIS
	routes.POST("/create/:id", blogCli.DraftAndPublish)

	return blogCli
}

func (asc *BlogServiceClient) DraftAndPublish(ctx *gin.Context) {
	id := ctx.Param("id")

	body := Post{}
	if err := ctx.BindJSON(&body); err != nil {
		logrus.Errorf("cannot bind json to struct, error: %v", err)
		_ = ctx.AbortWithError(http.StatusBadRequest, err)
		return
	}

	body.Id = id
	// TODO: Remove this line
	logrus.Infof("The Post: %+v", body)

	res, err := asc.Client.DraftAndPublish(context.Background(), &pb.BlogRequest{
		Id: body.Id,
	})

	if err != nil {
		_ = ctx.AbortWithError(http.StatusBadGateway, err)
		return
	}

	ctx.JSON(http.StatusAccepted, &res)

}

func (asc *BlogServiceClient) CreateABlog(ctx *gin.Context) {

	body := CreatePostRequestBody{}
	if err := ctx.BindJSON(&body); err != nil {
		logrus.Errorf("cannot bind json to struct, error: %v", err)
		_ = ctx.AbortWithError(http.StatusBadRequest, err)
		return
	}

	res, err := asc.Client.CreateABlog(context.Background(), &pb.CreateBlogRequest{
		Id:         uuid.NewString(),
		Title:      body.Title,
		Content:    body.Content,
		AuthorName: body.Author,
		AuthorId:   body.AuthorId,
		Published:  body.Published,
		Tags:       body.Tags,
	})

	if err != nil {
		_ = ctx.AbortWithError(http.StatusBadGateway, err)
		return
	}

	ctx.JSON(http.StatusAccepted, &res)

}

func (svc *BlogServiceClient) Get100Blogs(ctx *gin.Context) {
	logrus.Infof("traffic is coming from ip: %v", ctx.ClientIP())

	stream, err := svc.Client.Get100Blogs(context.Background(), &emptypb.Empty{})
	if err != nil {
		logrus.Errorf("cannot connect to article stream rpc server, error: %v", err)
		_ = ctx.AbortWithError(http.StatusBadGateway, err)
		return
	}

	response := []*pb.GetBlogsResponse{}
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			logrus.Errorf("cannot get the stream data, error: %+v", err)
		}

		response = append(response, resp)
	}

	ctx.JSON(http.StatusCreated, response)
}

func (svc *BlogServiceClient) GetArticleById(ctx *gin.Context) {
	id := ctx.Param("id")

	res, err := svc.Client.GetBlogById(context.Background(), &pb.GetBlogByIdRequest{Id: id})
	if err != nil {
		logrus.Errorf("cannot connect to article rpc server, error: %v", err)
		_ = ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusCreated, res)
}

func (blog *BlogServiceClient) EditArticles(ctx *gin.Context) {
	id := ctx.Param("id")

	reqObj := EditArticleRequestBody{}

	if err := ctx.BindJSON(&reqObj); err != nil {
		logrus.Errorf("invalid body, error: %v", err)
		_ = ctx.AbortWithError(http.StatusBadRequest, err)
		return
	}
	var isPartial bool
	if ctx.Request.Method == http.MethodPatch {
		isPartial = true
	}

	res, err := blog.Client.EditBlogById(context.Background(), &pb.EditBlogRequest{
		Id:        id,
		Title:     reqObj.Title,
		Content:   reqObj.Content,
		Tags:      reqObj.Tags,
		IsPartial: isPartial,
	})

	if err != nil {
		logrus.Errorf("cannot connect to article rpc server, error: %v", err)
		_ = ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusCreated, res)
}

func (svc *BlogServiceClient) DeleteBlogById(ctx *gin.Context) {
	id := ctx.Param("id")

	res, err := svc.Client.DeleteBlogById(context.Background(), &pb.DeleteBlogByIdRequest{Id: id})
	if err != nil {
		logrus.Errorf("cannot connect to article rpc server, error: %v", err)
		_ = ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusCreated, res)
}

func (svc *BlogServiceClient) Get100PostsByTags(ctx *gin.Context) {
	logrus.Infof("traffic is coming from ip: %v", ctx.ClientIP())

	reqObj := Tag{}

	if err := ctx.BindJSON(&reqObj); err != nil {
		logrus.Errorf("invalid body, error: %v", err)
		_ = ctx.AbortWithError(http.StatusBadRequest, err)
		return
	}

	stream, err := svc.Client.GetBlogsByTag(context.Background(), &pb.GetBlogsByTagReq{
		TagName: reqObj.TagName,
	})

	if err != nil {
		logrus.Errorf("cannot connect to article stream rpc server, error: %v", err)
		_ = ctx.AbortWithError(http.StatusBadGateway, err)
		return
	}

	response := []*pb.GetBlogsResponse{}
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			logrus.Errorf("cannot get the stream data, error: %+v", err)
		}

		response = append(response, resp)
	}

	ctx.JSON(http.StatusCreated, response)
}
