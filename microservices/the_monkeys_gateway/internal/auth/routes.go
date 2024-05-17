package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/common"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/errors"

	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_authz/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ServiceClient struct {
	Client pb.AuthServiceClient
	Log    logrus.Logger
}

// InitServiceClient initializes the gRPC connection to the auth service.
func InitServiceClient(cfg *config.Config) pb.AuthServiceClient {
	cc, err := grpc.Dial(cfg.Microservices.TheMonkeysAuthz, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logrus.Errorf("cannot dial to grpc auth server: %v", err)
		return nil
	}

	logrus.Infof("✅ the monkeys gateway is dialing to the auth rpc server at: %v", cfg.Microservices.TheMonkeysAuthz)
	return pb.NewAuthServiceClient(cc)
}

func RegisterAuthRouter(router *gin.Engine, cfg *config.Config) *ServiceClient {

	asc := &ServiceClient{
		Client: InitServiceClient(cfg),
		Log:    *logrus.New(),
	}
	routes := router.Group("/api/v1/auth")

	routes.POST("/register", asc.Register)
	routes.POST("/login", asc.Login)
	routes.GET("/is-authenticated", asc.IsUserAuthenticated)

	routes.POST("/forgot-pass", asc.ForgotPassword)
	routes.GET("/reset-password", asc.ConfirmPasswordVerificationLink)
	routes.POST("/update-password", asc.UpdatePassword)
	routes.GET("/verify-email", asc.VerifyEmail)

	// Authentication Point
	mware := InitAuthMiddleware(asc)
	routes.Use(mware.AuthRequired)

	routes.POST("/req-email-verification", asc.ReqEmailVerification)

	// Roles for blog
	routes.GET("/roles", asc.GetRoles)
	routes.GET("/role/:id", asc.GetRoles)
	routes.GET("/roles/:user_id", asc.GetRoles)
	routes.POST("/roles/:blog_id", asc.GetRoles)

	return asc
}

func (asc *ServiceClient) Register(ctx *gin.Context) {
	body := RegisterRequestBody{}

	if err := ctx.BindJSON(&body); err != nil {
		asc.Log.Errorf("json body is not correct, error: %v", err)
		_ = ctx.AbortWithError(http.StatusBadRequest, err)
		return
	}

	logrus.Infof("traffic is coming from ip: %v", ctx.ClientIP())

	body.FirstName = strings.TrimSpace(body.FirstName)
	body.LastName = strings.TrimSpace(body.LastName)
	body.Email = strings.TrimSpace(body.Email)

	// check for google login
	var loginMethod pb.RegisterUserRequest_LoginMethod
	if body.LoginMethod == "google-oauth2" {
		loginMethod = pb.RegisterUserRequest_GOOGLE_ACC
	} else if body.LoginMethod == "the-monkeys" {
		loginMethod = pb.RegisterUserRequest_The_MONKEYS
	}

	res, err := asc.Client.RegisterUser(context.Background(), &pb.RegisterUserRequest{
		FirstName:   body.FirstName,
		LastName:    body.LastName,
		Email:       body.Email,
		Password:    body.Password,
		LoginMethod: loginMethod,
	})

	if err != nil {
		asc.Log.Errorf("rpc auth server returned error, error: %v", err)
		_ = ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	if res.StatusCode == http.StatusConflict {
		ctx.JSON(http.StatusConflict, res)
		return
	}

	ctx.JSON(int(res.StatusCode), &res)
}

func (asc *ServiceClient) Login(ctx *gin.Context) {
	body := LoginRequestBody{}

	logrus.Infof("traffic is coming from ip: %v", ctx.ClientIP())

	if err := ctx.BindJSON(&body); err != nil {
		asc.Log.Errorf("json body is not correct, error: %v", err)
		_ = ctx.AbortWithError(http.StatusBadRequest, err)
		return
	}

	body.Email = strings.TrimSpace(body.Email)

	res, err := asc.Client.Login(context.Background(), &pb.LoginUserRequest{
		Email:    body.Email,
		Password: body.Password,
	})

	if err != nil {
		asc.Log.Errorf("internal server error, user containing email: %s cannot login", body.Email)
		_ = ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	if res.StatusCode == http.StatusNotFound {
		asc.Log.Errorf("user containing email: %s, doesn't exists", body.Email)
		ctx.AbortWithStatusJSON(http.StatusNotFound, res)
		return
	}

	if res.StatusCode == http.StatusBadRequest {
		asc.Log.Errorf("incorrect password given for the user containing email: %s", body.Email)
		_ = ctx.AbortWithError(http.StatusNotFound, common.ErrBadRequest)
		return
	}

	ctx.JSON(http.StatusOK, &res)
}

func (asc *ServiceClient) ForgotPassword(ctx *gin.Context) {
	body := ForgetPass{}

	if err := ctx.BindJSON(&body); err != nil {
		asc.Log.Errorf("json body is not correct, error: %v", err)
		_ = ctx.AbortWithError(http.StatusBadRequest, err)
		return
	}

	res, err := asc.Client.ForgotPassword(context.Background(), &pb.ForgotPasswordReq{
		Email: body.Email,
	})

	if err != nil {
		errors.RestError(ctx, err, "user")
		return
	}

	ctx.JSON(http.StatusOK, &res)
}

// TODO: Rename it to Password Reset Email Verification
func (asc *ServiceClient) ConfirmPasswordVerificationLink(ctx *gin.Context) {
	userAny := ctx.Query("user")
	secretAny := ctx.Query("evpw")

	res, err := asc.Client.ResetPassword(context.Background(), &pb.ResetPasswordReq{
		Username: userAny,
		Token:    secretAny,
	})

	if err != nil {
		asc.Log.Errorf("rpc auth server returned error: %v", err)
		_ = ctx.AbortWithError(http.StatusForbidden, err)
		return
	}

	if res.StatusCode == http.StatusNotFound {
		asc.Log.Infof("user containing email: %s, doesn't exists", userAny)
		_ = ctx.AbortWithError(http.StatusNotFound, common.ErrNotFound)
		return
	}

	if res.StatusCode == http.StatusBadRequest {
		asc.Log.Infof("incorrect password given for the user containing email: %s", userAny)
		_ = ctx.AbortWithError(http.StatusNotFound, common.ErrBadRequest)
		return
	}

	ctx.JSON(http.StatusOK, &res)
}

func (asc *ServiceClient) UpdatePassword(ctx *gin.Context) {
	authorization := ctx.Request.Header.Get("Authorization")
	if authorization == "" {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	token := strings.Split(authorization, "Bearer ")

	if len(token) < 2 {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	res, err := asc.Client.Validate(context.Background(), &pb.ValidateRequest{
		Token: token[1],
	})

	if err != nil || res.StatusCode != http.StatusOK {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	pass := UpdatePassword{}
	if err := ctx.BindJSON(&pass); err != nil {
		asc.Log.Errorf("json body is not correct, error: %v", err)
		_ = ctx.AbortWithError(http.StatusBadRequest, err)
		return
	}

	passResp, err := asc.Client.UpdatePassword(context.Background(), &pb.UpdatePasswordReq{
		Password: pass.Password,
		Username: res.UserName,
		Email:    res.Email,
	})
	if err != nil {
		errors.RestError(ctx, err, "user")
		return
	}

	ctx.JSON(http.StatusOK, passResp)
}

func (asc *ServiceClient) ReqEmailVerification(ctx *gin.Context) {
	var vrEmail VerifyEmail

	if err := ctx.BindJSON(&vrEmail); err != nil {
		asc.Log.Errorf("json body is not correct, error: %v", err)
		ctx.AbortWithStatusJSON(http.StatusBadRequest, IncorrectReqBody{Error: "Invalid json body"})
		return
	}

	res, err := asc.Client.RequestForEmailVerification(context.Background(), &pb.EmailVerificationReq{
		Email: vrEmail.Email,
	})

	if err != nil {
		asc.Log.Errorf("rpc auth server returned error: %v", err)
		_ = ctx.AbortWithError(http.StatusForbidden, err)
		return
	}

	if res.StatusCode == http.StatusNotFound || res.Error != nil {
		asc.Log.Infof("user containing email: %s, doesn't exists", vrEmail.Email)
		_ = ctx.AbortWithError(http.StatusNotFound, common.ErrNotFound)
		return
	}

	if res.StatusCode == http.StatusBadRequest || res.Error != nil {
		asc.Log.Infof("incorrect password given for the user containing email: %s", vrEmail.Email)
		_ = ctx.AbortWithError(http.StatusNotFound, common.ErrBadRequest)
		return
	}

	ctx.JSON(http.StatusOK, &res)
}

// To verify email
func (asc *ServiceClient) VerifyEmail(ctx *gin.Context) {
	username := ctx.Query("user")
	evsecret := ctx.Query("evpw")

	// Verify Headers
	res, err := asc.Client.VerifyEmail(context.Background(), &pb.VerifyEmailReq{
		Username: username,
		Token:    evsecret,
	})

	if err != nil {
		asc.Log.Errorf("rpc auth server returned error: %v", err)
		_ = ctx.AbortWithError(http.StatusForbidden, err)
		return
	}

	// TODO: COrrect the errors
	if res.StatusCode == http.StatusNotFound || res.Error != nil {
		asc.Log.Infof("user containing username: %s, doesn't exists", username)
		_ = ctx.AbortWithError(http.StatusNotFound, common.ErrNotFound)
		return
	}

	if res.StatusCode == http.StatusBadRequest || res.Error != nil {
		// asc.Log.Infof("incorrect password given for the user containing email: %s", vrEmail.Email)
		_ = ctx.AbortWithError(http.StatusNotFound, common.ErrBadRequest)
		return
	}

	ctx.JSON(http.StatusOK, &res)
}

func (asc *ServiceClient) IsUserAuthenticated(ctx *gin.Context) {
	authorization := ctx.Request.Header.Get("authorization")

	if authorization == "" {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	token := strings.Split(authorization, "Bearer ")

	if len(token) < 2 {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, Authorization{AuthorizationStatus: false, Error: "unauthorized"})
		return
	}
	user := ctx.Request.Header.Get("user")
	if user == "" {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, Authorization{AuthorizationStatus: false, Error: "unauthorized"})
		return
	}
	res, err := asc.Client.Validate(context.Background(), &pb.ValidateRequest{
		Token: token[1],
	})
	if err != nil || res.StatusCode != http.StatusOK {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, Authorization{AuthorizationStatus: false, Error: "unauthorized"})
		return
	}

	if res.UserName != user {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, Authorization{AuthorizationStatus: false, Error: "unauthorized"})
		return
	}

	ctx.JSON(http.StatusOK, struct {
		Authorized bool `json:"authorized"`
	}{Authorized: true})
}

func (asc *ServiceClient) GetRoles(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, "administrator")
}
