package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/the-monkeys/the_monkeys/config"

	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_authz/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type ServiceClient struct {
	Client            pb.AuthServiceClient
	Log               logrus.Logger
	googleOauthConfig *oauth2.Config
}

// InitServiceClient initializes the gRPC connection to the auth service.
func InitServiceClient(cfg *config.Config) pb.AuthServiceClient {
	cc, err := grpc.NewClient(cfg.Microservices.TheMonkeysAuthz, grpc.WithTransportCredentials(insecure.NewCredentials()))
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
		googleOauthConfig: &oauth2.Config{
			RedirectURL:  cfg.GoogleOAuth2.RedirectURL,
			ClientID:     cfg.GoogleOAuth2.ClientID,     // Replace with your Google Client ID
			ClientSecret: cfg.GoogleOAuth2.ClientSecret, // Replace with your Google Client Secret
			Scopes:       cfg.GoogleOAuth2.Scope,
			Endpoint:     google.Endpoint,
		},
	}
	fmt.Printf("asc.googleOauthConfig: %+v\n", asc.googleOauthConfig)

	routes := router.Group("/api/v1/auth")

	// SSO Google Login
	router.GET("/auth/google/login", asc.HandleGoogleLogin)
	router.GET("/auth/google/callback", asc.HandleGoogleCallback)

	routes.POST("/register", asc.Register)
	routes.POST("/login", asc.Login)
	routes.GET("/is-authenticated", asc.IsUserAuthenticated)

	routes.POST("/forgot-pass", asc.ForgotPassword)
	routes.GET("/reset-password", asc.PasswordResetEmailVerification)
	routes.POST("/update-password", asc.UpdatePassword)
	routes.GET("/verify-email", asc.VerifyEmail)

	// Authentication Point
	mware := InitAuthMiddleware(asc)
	routes.Use(mware.AuthRequired)

	routes.POST("/req-email-verification", asc.ReqEmailVerification)
	routes.PUT("/settings/username/:username", asc.UpdateUserName)
	routes.PUT("/settings/email/:username", asc.UpdateEmailAddress)
	routes.PUT("/settings/password/:username", asc.ChangePasswordWithCurrentPassword)
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

	ipAddress := ctx.Request.Header.Get("IP")
	client := ctx.Request.Header.Get("Client")

	res, err := asc.Client.RegisterUser(context.Background(), &pb.RegisterUserRequest{
		FirstName:   body.FirstName,
		LastName:    body.LastName,
		Email:       body.Email,
		Password:    body.Password,
		LoginMethod: loginMethod,
		IpAddress:   ipAddress,
		Client:      client,
	})

	if err != nil {
		// Check for gRPC error code
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.InvalidArgument:
				ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "incomplete request, please provide first name, last name, email and password"})
				return
			case codes.AlreadyExists:
				ctx.AbortWithStatusJSON(http.StatusConflict, gin.H{"message": "user with this email already exists"})
				return
			case codes.Aborted:
				ctx.AbortWithStatusJSON(http.StatusPartialContent, gin.H{"message": "Registration done but token is not created, try logging in"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot register the user, something went wrong"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
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

	ipAddress := ctx.Request.Header.Get("Ip")
	client := ctx.Request.Header.Get("Client")

	res, err := asc.Client.Login(context.Background(), &pb.LoginUserRequest{
		Email:     body.Email,
		Password:  body.Password,
		IpAddress: ipAddress,
		Client:    client,
	})

	if err != nil {
		// Check for gRPC error code
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "email/password is incorrect"})
				return
			case codes.Unauthenticated:
				ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "email/password is incorrect"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot generate the token"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "unknown error"})
				return
			}
		}
	}
	ctx.JSON(http.StatusOK, &res)
}

func (asc *ServiceClient) ForgotPassword(ctx *gin.Context) {
	body := GetEmail{}

	if err := ctx.BindJSON(&body); err != nil {
		asc.Log.Errorf("json body is not correct, error: %v", err)
		_ = ctx.AbortWithError(http.StatusBadRequest, err)
		return
	}

	ipAddress := ctx.Request.Header.Get("Ip")
	client := ctx.Request.Header.Get("Client")

	res, err := asc.Client.ForgotPassword(context.Background(), &pb.ForgotPasswordReq{
		Email:     body.Email,
		IpAddress: ipAddress,
		Client:    client,
	})

	if err != nil {
		// Check for gRPC error code
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusOK, gin.H{"message": "If the account is registered with this email, you’ll receive an email verification link to reset your password."})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "something went wrong"})
				return
			}
		}
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "sent password reset link to the email", "status": res.StatusCode})
}

// TODO: Rename it to Password Reset Email Verification
func (asc *ServiceClient) PasswordResetEmailVerification(ctx *gin.Context) {
	userAny := ctx.Query("user")
	secretAny := ctx.Query("evpw")

	ipAddress := ctx.Request.Header.Get("Ip")
	client := ctx.Request.Header.Get("Client")

	res, err := asc.Client.ResetPassword(context.Background(), &pb.ResetPasswordReq{
		Username:  userAny,
		Token:     secretAny,
		IpAddress: ipAddress,
		Client:    client,
	})

	if err != nil {
		// Check for gRPC error code
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "User not found"})
				return
			case codes.Unauthenticated:
				ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Token expired/incorrect"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
				return
			}
		}
	}

	ctx.JSON(http.StatusOK, gin.H{"response": res})
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

	ipAddress := ctx.Request.Header.Get("Ip")
	client := ctx.Request.Header.Get("Client")

	resp, err := asc.Client.UpdatePassword(context.Background(), &pb.UpdatePasswordReq{
		Password:  pass.NewPassword,
		Username:  res.UserName,
		Email:     res.Email,
		IpAddress: ipAddress,
		Client:    client,
	})
	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "User not found"})
				return
			case codes.Unauthenticated:
				ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Token expired/incorrect"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
				return
			}
		}
	}

	ctx.JSON(http.StatusOK, resp)
}

func (asc *ServiceClient) ReqEmailVerification(ctx *gin.Context) {
	var vrEmail VerifyEmail

	if err := ctx.BindJSON(&vrEmail); err != nil {
		asc.Log.Errorf("json body is not correct, error: %v", err)
		ctx.AbortWithStatusJSON(http.StatusBadRequest, IncorrectReqBody{Error: "Invalid json body"})
		return
	}

	ipAddress := ctx.Request.Header.Get("Ip")
	client := ctx.Request.Header.Get("Client")

	res, err := asc.Client.RequestForEmailVerification(context.Background(), &pb.EmailVerificationReq{
		Email:     vrEmail.Email,
		IpAddress: ipAddress,
		Client:    client,
	})

	if err != nil {
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "user not found"})
				return
			case codes.Internal:
				ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "error occurred while updating email verification token"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "internal server error"})
				return
			}
		}
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": "An email verification link has been sent to your registered email",
		"status":  res.StatusCode,
	})
}

// To verify email
func (asc *ServiceClient) VerifyEmail(ctx *gin.Context) {
	username := ctx.Query("user")
	evSecret := ctx.Query("evpw")

	ipAddress := ctx.Request.Header.Get("Ip")
	client := ctx.Request.Header.Get("Client")

	// Verify Headers
	res, err := asc.Client.VerifyEmail(context.Background(), &pb.VerifyEmailReq{
		Username:  username,
		Token:     evSecret,
		IpAddress: ipAddress,
		Client:    client,
	})

	if err != nil {
		// Check for gRPC error code
		if status, ok := status.FromError(err); ok {
			switch status.Code() {
			case codes.NotFound:
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "User not found"})
				return
			case codes.Unauthenticated:
				ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Token expired/incorrect"})
				return
			default:
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
				return
			}
		}
	}

	// Return success response
	ctx.JSON(http.StatusOK, gin.H{"message": "email verified", "status": res.StatusCode})
}

func (asc *ServiceClient) IsUserAuthenticated(ctx *gin.Context) {
	authorization := ctx.Request.Header.Get("Authorization")

	if authorization == "" {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	token := strings.Split(authorization, "Bearer ")

	if len(token) < 2 {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, Authorization{AuthorizationStatus: false, Error: "unauthorized"})
		return
	}
	user := ctx.Request.Header.Get("Username")
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

func (asc *ServiceClient) UpdateUserName(ctx *gin.Context) {
	currentUsername := ctx.Param("username")

	if currentUsername != ctx.GetString("userName") {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "you aren't authorized to perform this action"})
		return
	}

	ipAddress := ctx.Request.Header.Get("Ip")
	client := ctx.Request.Header.Get("Client")

	var updateUsername UpdateUsername

	if err := ctx.BindJSON(&updateUsername); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	resp, err := asc.Client.UpdateUsername(context.Background(), &pb.UpdateUsernameReq{
		CurrentUsername: currentUsername,
		NewUsername:     updateUsername.Username,
		Client:          client,
		Ip:              ipAddress,
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "user not found"})
			return
		} else {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "couldn't update username"})
			return
		}
	}

	ctx.JSON(http.StatusOK, resp)
}

func (asc *ServiceClient) ChangePasswordWithCurrentPassword(ctx *gin.Context) {
	username := ctx.Param("username")

	if username != ctx.GetString("userName") {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "you aren't authorized to perform this action"})
		return
	}

	ipAddress := ctx.Request.Header.Get("Ip")
	client := ctx.Request.Header.Get("Client")

	var updatePass UpdatePassword

	if err := ctx.BindJSON(&updatePass); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	resp, err := asc.Client.UpdatePasswordWithPassword(context.Background(), &pb.UpdatePasswordWithPasswordReq{
		Username:        username,
		CurrentPassword: updatePass.CurrentPassword,
		NewPassword:     updatePass.NewPassword,
		Client:          client,
		IpAddress:       ipAddress,
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "user not found"})
			return
		} else if status.Code(err) == codes.Unauthenticated {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "password incorrect"})
			return
		} else {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "couldn't update password"})
			return
		}
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "successfully updated password", "status": resp.StatusCode})
}

func (asc *ServiceClient) UpdateEmailAddress(ctx *gin.Context) {
	username := ctx.Param("username")

	if username != ctx.GetString("userName") {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "you aren't authorized to perform this action"})
		return
	}

	ipAddress := ctx.Request.Header.Get("Ip")
	client := ctx.Request.Header.Get("Client")

	var emailBody GetEmail

	if err := ctx.BindJSON(&emailBody); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	resp, err := asc.Client.UpdateEmailId(context.Background(), &pb.UpdateEmailIdReq{
		Username:  username,
		NewEmail:  emailBody.Email,
		Client:    client,
		IpAddress: ipAddress,
	})

	if err != nil {
		if status.Code(err) == codes.NotFound {
			ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "user not found"})
			return
		} else if status.Code(err) == codes.AlreadyExists {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "the email is already in use"})
			return
		} else {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "couldn't update email"})
			return
		}
	}

	ctx.JSON(http.StatusOK, resp)
}

func (asc *ServiceClient) HandleGoogleLogin(c *gin.Context) {
	// Redirect to Google's OAuth 2.0 server
	url := asc.googleOauthConfig.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("url: %v\n", url)
	c.Redirect(http.StatusTemporaryRedirect, url)
}

func (asc *ServiceClient) HandleGoogleCallback(c *gin.Context) {
	// Exchange the code for a token
	code := c.Query("code")
	token, err := asc.googleOauthConfig.Exchange(context.Background(), code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Code exchange failed"})
		return
	}

	// Retrieve user information
	client := asc.googleOauthConfig.Client(context.Background(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
		return
	}
	defer resp.Body.Close()

	// Process user info (you could store it in a database or create a session)
	var userInfo GoogleUser
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse user info"})
		return
	}

	loginResp, err := asc.Client.GoogleLogin(context.Background(), &pb.RegisterUserRequest{
		Email:       userInfo.Email,
		LoginMethod: pb.RegisterUserRequest_GOOGLE_ACC,
		FirstName:   userInfo.GivenName,
		LastName:    userInfo.FamilyName,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to login using google"})
		return
	}

	// Example response for client
	c.JSON(http.StatusOK, loginResp)
}
