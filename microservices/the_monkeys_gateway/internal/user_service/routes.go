package user_service

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_user/pb"
	"github.com/the-monkeys/the_monkeys/config"

	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/internal/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type UserServiceClient struct {
	Client pb.UserServiceClient
}

func NewUserServiceClient(cfg *config.Config) pb.UserServiceClient {
	cc, err := grpc.Dial(cfg.Microservices.TheMonkeysUser, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logrus.Errorf("cannot dial to grpc user server: %v", err)
	}
	logrus.Infof("✅ the monkeys gateway is dialing to user rpc server at: %v", cfg.Microservices.TheMonkeysUser)
	return pb.NewUserServiceClient(cc)
}

func RegisterUserRouter(router *gin.Engine, cfg *config.Config, authClient *auth.ServiceClient) *UserServiceClient {
	mware := auth.InitAuthMiddleware(authClient)

	usc := &UserServiceClient{
		Client: NewUserServiceClient(cfg),
	}
	routes := router.Group("/api/v1/user")
	routes.GET("/topics", usc.GetAllTopics)
	routes.GET("/category", usc.GetAllCategories)
	routes.GET("/public/:id", usc.GetUserPublicProfile)
	routes.GET("/public/account/:acc_id", usc.GetUserDetailsByAccId)

	routes.Use(mware.AuthRequired)

	{
		routes.PUT("/:id", usc.UpdateUserProfile)
		routes.PATCH("/:id", usc.UpdateUserProfile)
		routes.GET("/:id", usc.GetUserProfile)
		routes.DELETE("/:id", usc.DeleteUserProfile)
	}

	{
		routes.GET("/activities/:user_name", usc.GetUserActivities)
		routes.PUT("/topics/:user_name", usc.FollowTopic)
	}

	// Invite and un invite as coauthor
	// routes.GET("/activities/:user_name", usc.GetUserActivities)
	return usc
}

func (asc *UserServiceClient) GetUserProfile(ctx *gin.Context) {
	username := ctx.Param("id")
	var isPrivate bool
	if username == ctx.GetString("userName") {
		isPrivate = true
	}

	res, err := asc.Client.GetUserProfile(context.Background(), &pb.UserProfileReq{
		Username: username,
		// Email:     email,
		IsPrivate: isPrivate,
	})

	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "the user does not exist"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "something went wrong"})
				return
			}
		}
	}

	ctx.JSON(http.StatusOK, &res)
}

func (asc *UserServiceClient) GetUserPublicProfile(ctx *gin.Context) {
	username := ctx.Param("id")
	var isPrivate bool

	res, err := asc.Client.GetUserProfile(context.Background(), &pb.UserProfileReq{
		Username:  username,
		IsPrivate: isPrivate,
	})

	if err != nil {
		if status.Code(err) == codes.NotFound {
			ctx.AbortWithStatusJSON(http.StatusNotFound, ReturnMessage{Message: "user not found"})
			return
		} else {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, ReturnMessage{Message: "something went wrong"})
			return
		}
	}

	ctx.JSON(http.StatusAccepted, &res)
}

func (asc *UserServiceClient) GetUserActivities(ctx *gin.Context) {
	username := ctx.Param("user_name")
	if username != ctx.GetString("userName") {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "you are unauthorized to perform this action"})
		return
	}

	res, err := asc.Client.GetUserActivities(ctx, &pb.UserActivityReq{
		UserName: username,
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			ctx.AbortWithStatusJSON(http.StatusNotFound, ReturnMessage{Message: "no user/activity found"})
			return
		} else {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, ReturnMessage{Message: "couldn't get the user's activities"})
			return
		}
	}

	ctx.JSON(http.StatusOK, res)
}

func (usc *UserServiceClient) UpdateUserProfile(ctx *gin.Context) {
	username := ctx.Param("id")
	if username != ctx.GetString("userName") {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "you are unauthorized to perform this action"})
		return
	}

	ipAddress := ctx.Request.Header.Get("Ip")
	client := ctx.Request.Header.Get("Client")

	var req UpdateUserProfileRequest

	if err := ctx.ShouldBindJSON(&req); err != nil {
		logrus.Errorf("error while getting the update data: %v", err)
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	body := req.Values
	var isPartial bool
	if ctx.Request.Method == http.MethodPatch {
		isPartial = true
	}

	res, err := usc.Client.UpdateUserProfile(context.Background(), &pb.UpdateUserProfileReq{
		Username:      username,
		FirstName:     body.FirstName,
		LastName:      body.LastName,
		DateOfBirth:   body.DateOfBirth,
		Bio:           body.Bio,
		Address:       body.Address,
		ContactNumber: body.ContactNumber,
		Twitter:       body.Twitter,
		Instagram:     body.Instagram,
		Linkedin:      body.LinkedIn,
		Github:        body.Github,
		Ip:            ipAddress,
		Client:        client,
		Partial:       isPartial,
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			ctx.AbortWithStatusJSON(http.StatusNotFound, ReturnMessage{Message: "user not found"})
			return
		} else {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, ReturnMessage{Message: "couldn't update user informations"})
			return
		}
	}
	ctx.JSON(http.StatusOK, res)
}

func (asc *UserServiceClient) DeleteUserProfile(ctx *gin.Context) {
	username := ctx.Param("id")
	tokenUsername := ctx.GetString("userName")

	if username != tokenUsername {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, ReturnMessage{Message: "you are unauthorized to perform this action"})
		return
	}

	res, err := asc.Client.DeleteUserProfile(context.Background(), &pb.DeleteUserProfileReq{
		Username: username,
	})

	if err != nil {
		if status.Code(err) == codes.NotFound {
			ctx.AbortWithStatusJSON(http.StatusNotFound, ReturnMessage{Message: "no user/activity found"})
			return
		} else {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, ReturnMessage{Message: "couldn't get the user's activities"})
			return
		}
	}

	ctx.JSON(http.StatusOK, res)
}

func (asc *UserServiceClient) GetAllTopics(ctx *gin.Context) {
	res, err := asc.Client.GetAllTopics(context.Background(), &pb.GetTopicsRequests{})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get the list of topics"})
		return
	}

	ctx.JSON(http.StatusOK, res)
}

func (asc *UserServiceClient) GetAllCategories(ctx *gin.Context) {
	res, err := asc.Client.GetAllCategories(context.Background(), &pb.GetAllCategoriesReq{})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get the all the Categories"})
		return
	}

	ctx.JSON(http.StatusOK, res)
}

func (asc *UserServiceClient) GetUserDetailsByAccId(ctx *gin.Context) {
	accId := ctx.Param("acc_id")

	res, err := asc.Client.GetUserDetailsByAccId(context.Background(), &pb.UserDetailsByAccIdReq{
		AccountId: accId,
	})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			ctx.AbortWithStatusJSON(http.StatusNotFound, ReturnMessage{Message: "no user found"})
		case codes.Internal:
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, ReturnMessage{Message: "couldn't get the user info due to internal error"})
		default:
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, ReturnMessage{Message: "an unexpected error occurred"})
		}
		return
	}

	ctx.JSON(http.StatusOK, res)
}

func (asc *UserServiceClient) FollowTopic(ctx *gin.Context) {
	username := ctx.Param("user_name")

	if username != ctx.GetString("userName") {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "you are not allow to perform this action"})
		return
	}

	var req FollowTopic
	if err := ctx.ShouldBindJSON(&req); err != nil {
		logrus.Errorf("error while getting the update data: %v", err)
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	res, err := asc.Client.FollowTopics(context.Background(), &pb.FollowTopicReq{
		Username: username,
		Topic:    req.Topics,
	})

	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.InvalidArgument:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "invalid request body"})
				return
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "the user does not exist"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "something went wrong"})
				return
			}
		}
	}

	ctx.JSON(http.StatusOK, &res)
}
