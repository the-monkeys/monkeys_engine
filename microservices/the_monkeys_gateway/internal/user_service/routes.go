package user_service

import (
	"context"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_user/pb"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/constants"

	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/internal/auth"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/middleware"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type UserServiceClient struct {
	Client pb.UserServiceClient
}

func NewUserServiceClient(cfg *config.Config) pb.UserServiceClient {
	cc, err := grpc.NewClient(cfg.Microservices.TheMonkeysUser, grpc.WithTransportCredentials(insecure.NewCredentials()))
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
	routes.GET("/connection-count/:username", usc.ConnectionCount)

	routes.Use(mware.AuthRequired)

	{
		routes.PUT("/:id", usc.UpdateUserProfile)
		routes.PATCH("/:id", usc.UpdateUserProfile)
		routes.GET("/:id", usc.GetUserProfile)
		routes.DELETE("/:id", usc.DeleteUserProfile)
		routes.GET("/followers", usc.GetFollowers)
		routes.GET("/following", usc.GetFollowing)
	}

	{
		routes.GET("/activities/:user_name", usc.GetUserActivities)
		routes.PUT("/follow-topics/:user_name", usc.FollowTopic)
		routes.PUT("/un-follow-topics/:user_name", usc.UnFollowTopic)
		routes.POST("/follow/:username", usc.FollowUser)
		routes.POST("/unfollow/:username", usc.UnfollowUser)
		routes.GET("/is-followed/:username", usc.IsUserFollowed)
		routes.GET("/search", usc.SearchUser)
	}

	// Invite and un invite as coauthor
	{
		routes.POST("/invite/:blog_id/", mware.AuthzRequired, usc.InviteCoAuthor)
		routes.POST("/revoke-invite/:blog_id/", mware.AuthzRequired, usc.RevokeInviteCoAuthor)
		routes.GET("/all-blogs/:username", usc.GetBlogsByUserName)
		routes.POST("/bookmark/:blog_id", usc.BookMarkABlog)
		routes.POST("/remove-bookmark/:blog_id", usc.RemoveBookMarkFromABlog)
		routes.GET("/count-bookmarks/:blog_id", usc.CountBookMarks)
		routes.GET("/is-bookmarked/:blog_id", usc.IsBlogBookMarked)
		routes.POST("/like/:blog_id", usc.LikeABlog)
		routes.POST("/unlike/:blog_id", usc.UnlikeABlog)
		routes.GET("/count-likes/:blog_id", usc.CountLikes)
		routes.GET("/is-liked/:blog_id", usc.IsBlogLiked)
	}

	{
		routes.POST("/topics", usc.CreateNewTopics)
	}

	// Admin routes for user management (localhost only)
	adminRoutes := router.Group("/api/v1/admin/user")
	adminRoutes.Use(middleware.LocalhostOnlyMiddleware())
	{
		adminRoutes.DELETE("/force-delete/:id", usc.AdminDeleteUserProfile)
		adminRoutes.DELETE("/force-delete/email/:email", usc.AdminDeleteUserByEmail)
		adminRoutes.POST("/suspend/:id", usc.AdminSuspendUser)
		adminRoutes.POST("/ban/:id", usc.AdminBanUser)
	}

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

	ipAddress := ctx.Request.Header.Get("IP")
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

	res, err := asc.Client.DeleteUserAccount(context.Background(), &pb.DeleteUserProfileReq{
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

	ctx.SetCookie("mat", "", -1, "/", "", true, true)
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

	// Fetch user details by AccountId
	res, err := asc.Client.GetUserDetails(context.Background(), &pb.UserDetailReq{
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

	// Fetch followers and following counts
	connCount, err := asc.Client.GetFollowersFollowingCounts(context.Background(), &pb.UserDetailReq{
		Username: res.Username,
	})
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, ReturnMessage{Message: "couldn't fetch connection counts"})
		return
	}

	// Return the JSON response
	ctx.JSON(http.StatusOK, struct {
		User      *pb.UserDetailsResp `json:"user"`
		Followers int64               `json:"followers"`
		Following int64               `json:"following"`
	}{
		User:      res,
		Followers: connCount.Followers,
		Following: connCount.Following,
	})
}

func (asc *UserServiceClient) FollowTopic(ctx *gin.Context) {
	username := ctx.Param("user_name")
	ipAddress := ctx.Request.Header.Get("IP")
	client := ctx.Request.Header.Get("Client")

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

	res, err := asc.Client.FollowTopics(context.Background(), &pb.TopicActionReq{
		Username: username,
		Topic:    req.Topics,
		Ip:       ipAddress,
		Client:   client,
	})

	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.InvalidArgument:
				ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "invalid request body"})
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

func (asc *UserServiceClient) UnFollowTopic(ctx *gin.Context) {
	username := ctx.Param("user_name")
	ipAddress := ctx.Request.Header.Get("IP")
	client := ctx.Request.Header.Get("Client")

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

	res, err := asc.Client.UnFollowTopics(context.Background(), &pb.TopicActionReq{
		Username: username,
		Topic:    req.Topics,
		Ip:       ipAddress,
		Client:   client,
	})

	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.InvalidArgument:
				ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "invalid request body"})
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

func (asc *UserServiceClient) InviteCoAuthor(ctx *gin.Context) {
	blogId := ctx.Param("blog_id")
	userName := ctx.GetString("userName")

	ipAddress := ctx.Request.Header.Get("IP")
	client := ctx.Request.Header.Get("Client")

	// Check permissions:
	if !utils.CheckUserRoleInContext(ctx, constants.RoleOwner) {
		logrus.Errorf("user does not have the permission to invite a co-author")
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "you are not allowed to perform this action"})
		return
	}
	// accId := ctx.GetString("accountId")

	var req CoAuthor
	if err := ctx.ShouldBindJSON(&req); err != nil {
		logrus.Errorf("error while getting the update data: %v", err)
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	res, err := asc.Client.InviteCoAuthor(context.Background(), &pb.CoAuthorAccessReq{
		AccountId:         req.AccountId,
		Username:          req.Username,
		Email:             req.Email,
		Ip:                ipAddress,
		Client:            client,
		BlogOwnerUsername: userName,
		BlogId:            blogId,
	})

	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.InvalidArgument:
				ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "invalid request body"})
				return
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "the user/blog does not exist"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "something went wrong"})
				return
			}
		}
	}

	ctx.JSON(http.StatusOK, &res)
}

func (asc *UserServiceClient) RevokeInviteCoAuthor(ctx *gin.Context) {
	blogId := ctx.Param("blog_id")
	userName := ctx.GetString("userName")

	ipAddress := ctx.Request.Header.Get("IP")
	client := ctx.Request.Header.Get("Client")

	// Check permissions:
	if !utils.CheckUserRoleInContext(ctx, constants.RoleOwner) {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "you are not allowed to perform this action"})
		return
	}
	// accId := ctx.GetString("accountId")

	var req CoAuthor
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	res, err := asc.Client.RevokeCoAuthorAccess(context.Background(), &pb.CoAuthorAccessReq{
		AccountId:         req.AccountId,
		Username:          req.Username,
		Email:             req.Email,
		Ip:                ipAddress,
		Client:            client,
		BlogOwnerUsername: userName,
		BlogId:            blogId,
	})

	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.InvalidArgument:
				ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "invalid request body"})
				return
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "the user/blog does not exist"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "something went wrong"})
				return
			}
		}
	}

	ctx.JSON(http.StatusOK, &res)
}

func (asc *UserServiceClient) GetBlogsByUserName(ctx *gin.Context) {
	username := ctx.Param("username")

	if username != ctx.GetString("userName") {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "you are not allow to perform this action"})
		return
	}

	res, err := asc.Client.GetBlogsByUserIds(context.Background(), &pb.BlogsByUserIdsReq{
		Username: username,
	})

	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "the user does not exist"})
				return
			case codes.AlreadyExists:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "the user already has the blog permission"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "something went wrong"})
				return
			}
		}
	}

	ctx.JSON(http.StatusOK, &res)
}

func (asc *UserServiceClient) CreateNewTopics(ctx *gin.Context) {
	userName := ctx.GetString("userName")

	var req Topics

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	res, err := asc.Client.CreateNewTopics(context.Background(), &pb.CreateTopicsReq{
		Topics:   req.Topics,
		Category: req.Category,
		Username: userName,
		Ip:       ctx.Request.Header.Get("Ip"),
		Client:   ctx.Request.Header.Get("Client"),
	})

	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.InvalidArgument:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "invalid request body"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "something went wrong"})
				return
			}
		}
	}
	ctx.JSON(http.StatusOK, res)
}

func (asc *UserServiceClient) BookMarkABlog(ctx *gin.Context) {
	userName := ctx.GetString("userName")
	blogId := ctx.Param("blog_id")

	res, err := asc.Client.BookMarkBlog(context.Background(), &pb.BookMarkReq{
		Username: userName,
		BlogId:   blogId,
		Ip:       ctx.Request.Header.Get("Ip"),
		Client:   ctx.Request.Header.Get("Client"),
	})

	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "the blog does not exist"})
				return
			case codes.AlreadyExists:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "the blog already bookmarked"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "something went wrong"})
				return
			}
		}
	}

	ctx.JSON(http.StatusOK, res)
}

func (asc *UserServiceClient) RemoveBookMarkFromABlog(ctx *gin.Context) {
	userName := ctx.GetString("userName")
	blogId := ctx.Param("blog_id")

	res, err := asc.Client.RemoveBookMark(context.Background(), &pb.BookMarkReq{
		Username: userName,
		BlogId:   blogId,
		Ip:       ctx.Request.Header.Get("Ip"),
		Client:   ctx.Request.Header.Get("Client"),
	})

	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "the blog does not exist"})
				return
			case codes.AlreadyExists:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "the blog already removed from bookmarked"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "something went wrong"})
				return
			}
		}
	}

	ctx.JSON(http.StatusOK, res)
}

func (asc *UserServiceClient) FollowUser(ctx *gin.Context) {
	username := ctx.Param("username")
	followerUsername := ctx.GetString("userName")

	resp, err := asc.Client.FollowUser(context.Background(), &pb.UserFollowReq{
		Username:         username,
		FollowerUsername: followerUsername,
		Ip:               ctx.Request.Header.Get("IP"),
		Client:           ctx.Request.Header.Get("Client"),
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
	ctx.JSON(http.StatusOK, resp)
}

func (asc *UserServiceClient) UnfollowUser(ctx *gin.Context) {
	username := ctx.Param("username")
	followerUsername := ctx.GetString("userName")

	resp, err := asc.Client.UnFollowUser(context.Background(), &pb.UserFollowReq{
		Username:         username,
		FollowerUsername: followerUsername,
		Ip:               ctx.Request.Header.Get("IP"),
		Client:           ctx.Request.Header.Get("Client"),
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
	ctx.JSON(http.StatusOK, resp)
}

func (asc *UserServiceClient) GetFollowers(ctx *gin.Context) {
	username := ctx.GetString("userName")

	resp, err := asc.Client.GetFollowers(context.Background(), &pb.UserDetailReq{
		Username: username,
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
	ctx.JSON(http.StatusOK, resp)
}

func (asc *UserServiceClient) GetFollowing(ctx *gin.Context) {
	username := ctx.GetString("userName")

	resp, err := asc.Client.GetFollowing(context.Background(), &pb.UserDetailReq{
		Username: username,
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
	ctx.JSON(http.StatusOK, resp)
}

func (asc *UserServiceClient) LikeABlog(ctx *gin.Context) {
	userName := ctx.GetString("userName")
	blogId := ctx.Param("blog_id")

	res, err := asc.Client.LikeBlog(context.Background(), &pb.BookMarkReq{
		Username: userName,
		BlogId:   blogId,
		Ip:       ctx.Request.Header.Get("Ip"),
		Client:   ctx.Request.Header.Get("Client"),
	})

	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "the blog does not exist"})
				return
			case codes.AlreadyExists:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "the blog already like"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "something went wrong"})
				return
			}
		}
	}

	ctx.JSON(http.StatusOK, res)
}

func (asc *UserServiceClient) UnlikeABlog(ctx *gin.Context) {
	userName := ctx.GetString("userName")
	blogId := ctx.Param("blog_id")

	res, err := asc.Client.UnlikeBlog(context.Background(), &pb.BookMarkReq{
		Username: userName,
		BlogId:   blogId,
		Ip:       ctx.Request.Header.Get("Ip"),
		Client:   ctx.Request.Header.Get("Client"),
	})

	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "the blog does not exist"})
				return
			case codes.AlreadyExists:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "the blog already removed from like"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "something went wrong"})
				return
			}
		}
	}

	ctx.JSON(http.StatusOK, res)
}

func (asc *UserServiceClient) IsBlogLiked(ctx *gin.Context) {
	userName := ctx.GetString("userName")
	blogId := ctx.Param("blog_id")

	res, err := asc.Client.GetIfBlogLiked(context.Background(), &pb.BookMarkReq{
		Username: userName,
		BlogId:   blogId,
		Ip:       ctx.Request.Header.Get("Ip"),
		Client:   ctx.Request.Header.Get("Client"),
	})

	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "the blog does not exist"})
				return
			case codes.AlreadyExists:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "the blog already removed from like"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "something went wrong"})
				return
			}
		}
	}

	ctx.JSON(http.StatusOK, res)
}

func (asc *UserServiceClient) IsUserFollowed(ctx *gin.Context) {
	username := ctx.GetString("userName")
	followedUsername := ctx.Param("username")

	res, err := asc.Client.GetIfIFollowedUser(context.Background(), &pb.UserFollowReq{
		Username:         followedUsername,
		FollowerUsername: username,
		Ip:               ctx.Request.Header.Get("IP"),
		Client:           ctx.Request.Header.Get("Client"),
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
	ctx.JSON(http.StatusOK, res)
}

func (asc *UserServiceClient) CountBookMarks(ctx *gin.Context) {
	blogId := ctx.Param("blog_id")

	res, err := asc.Client.GetBookMarkCounts(context.Background(), &pb.BookMarkReq{
		BlogId: blogId,
	})

	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "the blog does not exist"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "something went wrong"})
				return
			}
		}
	}
	ctx.JSON(http.StatusOK, res)
}

func (asc *UserServiceClient) IsBlogBookMarked(ctx *gin.Context) {
	userName := ctx.GetString("userName")
	blogId := ctx.Param("blog_id")

	res, err := asc.Client.GetIfBlogBookMarked(context.Background(), &pb.BookMarkReq{
		Username: userName,
		BlogId:   blogId,
		Ip:       ctx.Request.Header.Get("Ip"),
		Client:   ctx.Request.Header.Get("Client"),
	})

	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "the blog does not exist"})
				return
			case codes.AlreadyExists:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "the blog already bookmarked"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "something went wrong"})
				return
			}
		}
	}

	ctx.JSON(http.StatusOK, res)
}

func (asc *UserServiceClient) CountLikes(ctx *gin.Context) {
	blogId := ctx.Param("blog_id")

	res, err := asc.Client.GetLikeCounts(context.Background(), &pb.BookMarkReq{
		BlogId: blogId,
	})

	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "the blog does not exist"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "something went wrong"})
				return
			}
		}
	}
	ctx.JSON(http.StatusOK, res)
}

func (asc *UserServiceClient) SearchUser(ctx *gin.Context) {
	// Extract query parameters
	searchTerm := ctx.Query("search_term")
	limit := ctx.DefaultQuery("limit", "10")
	offset := ctx.DefaultQuery("offset", "0")

	// Convert limit and offset to integers
	limitInt, err := strconv.Atoi(limit)
	if err != nil || limitInt <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "Invalid limit parameter"})
		return
	}

	offsetInt, err := strconv.Atoi(offset)
	if err != nil || offsetInt < 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "Invalid offset parameter"})
		return
	}

	// Start the gRPC streaming client
	stream, err := asc.Client.SearchUser(context.Background())
	if err != nil {
		logrus.Errorf("Failed to initialize SearchUser stream: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to initiate search"})
		return
	}

	// Send the initial search request
	err = stream.Send(&pb.UserDetailReq{
		SearchTerm: searchTerm,
		Limit:      int32(limitInt),
		Offset:     int32(offsetInt),
	})
	if err != nil {
		logrus.Errorf("Failed to send search request: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to send search request"})
		return
	}

	// Close the stream after sending
	if err := stream.CloseSend(); err != nil {
		logrus.Warnf("Failed to close send stream: %v", err)
	}

	// Collect responses from the stream
	var results []*pb.User
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			// End of stream
			break
		}
		if err != nil {
			logrus.Errorf("Error receiving search response: %v", err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"message": "Error receiving search results"})
			return
		}

		// Append users to results
		results = append(results, resp.Users...)
	}

	// Return results to the client
	ctx.JSON(http.StatusOK, gin.H{"users": results})
}

func (asc *UserServiceClient) ConnectionCount(ctx *gin.Context) {
	username := ctx.Param("username")

	res, err := asc.Client.GetFollowersFollowingCounts(context.Background(), &pb.UserDetailReq{
		Username: username,
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
	ctx.JSON(http.StatusOK, res)
}

// AdminDeleteUserProfile force deletes a user account without consent (Admin only)
func (usc *UserServiceClient) AdminDeleteUserProfile(ctx *gin.Context) {
	username := ctx.Param("id")
	reason := ctx.DefaultQuery("reason", "Admin action - no reason provided")

	logrus.Warnf("Admin force delete requested for user: %s, reason: %s", username, reason)

	res, err := usc.Client.DeleteUserAccount(context.Background(), &pb.DeleteUserProfileReq{
		Username: username,
	})

	if err != nil {
		if status.Code(err) == codes.NotFound {
			ctx.AbortWithStatusJSON(http.StatusNotFound, ReturnMessage{Message: "user not found"})
			return
		} else {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, ReturnMessage{Message: "couldn't delete user account"})
			return
		}
	}

	logrus.Infof("Admin successfully deleted user account: %s", username)
	ctx.JSON(http.StatusOK, gin.H{
		"message":  "User account successfully deleted",
		"username": username,
		"reason":   reason,
		"result":   res,
	})
}

// AdminDeleteUserByEmail force deletes a user account by email (Admin only)
func (usc *UserServiceClient) AdminDeleteUserByEmail(ctx *gin.Context) {
	email := ctx.Param("email")
	reason := ctx.DefaultQuery("reason", "Admin action - no reason provided")

	logrus.Warnf("Admin force delete by email requested for: %s, reason: %s", email, reason)

	// Create a request to delete by email - you may need to modify the protobuf
	res, err := usc.Client.DeleteUserAccount(context.Background(), &pb.DeleteUserProfileReq{
		Username: email, // Using email as identifier for now
	})

	if err != nil {
		if status.Code(err) == codes.NotFound {
			ctx.AbortWithStatusJSON(http.StatusNotFound, ReturnMessage{Message: "user not found"})
			return
		} else {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, ReturnMessage{Message: "couldn't delete user account"})
			return
		}
	}

	logrus.Infof("Admin successfully deleted user account by email: %s", email)
	ctx.JSON(http.StatusOK, gin.H{
		"message": "User account successfully deleted",
		"email":   email,
		"reason":  reason,
		"result":  res,
	})
}

// AdminSuspendUser suspends a user account (Admin only)
func (usc *UserServiceClient) AdminSuspendUser(ctx *gin.Context) {
	username := ctx.Param("id")
	reason := ctx.DefaultQuery("reason", "Admin action - no reason provided")
	duration := ctx.DefaultQuery("duration", "24h") // Default 24 hours

	logrus.Warnf("Admin suspend requested for user: %s, reason: %s, duration: %s", username, reason, duration)

	// Note: This assumes the user service has a SuspendUser method
	// You may need to implement this in the user service protobuf and service
	ctx.JSON(http.StatusOK, gin.H{
		"message":  "User suspension functionality not yet implemented in user service",
		"username": username,
		"reason":   reason,
		"duration": duration,
	})
}

// AdminBanUser permanently bans a user account (Admin only)
func (usc *UserServiceClient) AdminBanUser(ctx *gin.Context) {
	username := ctx.Param("id")
	reason := ctx.DefaultQuery("reason", "Admin action - no reason provided")

	logrus.Warnf("Admin ban requested for user: %s, reason: %s", username, reason)

	// Note: This assumes the user service has a BanUser method
	// You may need to implement this in the user service protobuf and service
	ctx.JSON(http.StatusOK, gin.H{
		"message":  "User ban functionality not yet implemented in user service",
		"username": username,
		"reason":   reason,
	})
}
