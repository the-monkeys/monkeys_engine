package user_service

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_user/pb"
	"github.com/the-monkeys/the_monkeys/config"

	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/errors"
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

	routes.Use(mware.AuthRequired)

	routes.GET("/:id", usc.GetUserProfile)
	routes.POST("/activities/:user_name", usc.GetUserActivities)
	routes.PATCH("/:username", usc.UpdateUserProfile)
	routes.PUT("/:username", usc.UpdateUserProfile)
	routes.DELETE("/:username", usc.DeleteUserProfile)

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
				ctx.AbortWithStatusJSON(http.StatusOK, gin.H{"message": "the user does not exist"})
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
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, ReturnMessage{Message: "user not found"})
			return
		} else {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, ReturnMessage{Message: "something went wrong"})
			return
		}
	}

	ctx.JSON(http.StatusAccepted, &res)
}

// func (asc *UserServiceClient) DeleteMyAccount(ctx *gin.Context) {
// 	// get id
// 	id := ctx.Param("id")
// 	userId, err := strconv.ParseInt(id, 10, 64)
// 	if err != nil {
// 		ctx.AbortWithError(http.StatusBadRequest, err)
// 		return
// 	}

// 	res, err := asc.Client.DeleteMyProfile(context.Background(), &pb.DeleteMyAccountReq{Id: userId})
// 	if err != nil {
// 		logrus.Errorf("cannot connect to user rpc server, error: %v", err)
// 		errors.RestError(ctx, err, "user_service")
// 		return
// 	}

// 	ctx.JSON(http.StatusOK, res)
// }

func (asc *UserServiceClient) GetUserActivities(ctx *gin.Context) {
	res, err := asc.Client.GetUserActivities(ctx, &pb.UserActivityReq{})
	if err != nil {
		logrus.Errorf("cannot connect to user rpc server, error: %v", err)
		errors.RestError(ctx, err, "user_service")
		return
	}

	ctx.JSON(http.StatusOK, res)
}

func (asc *UserServiceClient) UpdateUserProfile(ctx *gin.Context) {
	var isPartial bool

	username := ctx.Param("username")
	if username != ctx.GetString("userName") {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "you are unauthorized to perform this action"})
		return
	}

	ipAddress := ctx.Request.Header.Get("ip")
	client := ctx.Request.Header.Get("client")

	body := UpdateUserProfile{}
	if err := ctx.BindJSON(&body); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if ctx.Request.Method == http.MethodPatch || ctx.Request.Method == http.MethodPut {
		isPartial = true
	}

	// TODO: Remove the following line
	logrus.Infof("req body: %+v", body)
	res, err := asc.Client.UpdateUserProfile(context.Background(), &pb.UpdateUserProfileReq{
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
	username := ctx.Param("username")
	tokenUsername := ctx.GetString("userName")

	if username != tokenUsername {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	res, err := asc.Client.DeleteUserProfile(context.Background(), &pb.DeleteUserProfileReq{
		Username: username,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user profile"})
		return
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
