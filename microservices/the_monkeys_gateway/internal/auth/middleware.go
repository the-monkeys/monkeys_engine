package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_authz/pb"
)

type AuthMiddlewareConfig struct {
	svc *ServiceClient
}

func InitAuthMiddleware(svc *ServiceClient) AuthMiddlewareConfig {
	return AuthMiddlewareConfig{svc}
}

// Extracts the token from the Authorization header or query parameter
func (c *AuthMiddlewareConfig) extractToken(ctx *gin.Context) (string, error) {
	authorization := ctx.Request.Header.Get("Authorization")

	if authorization == "" {
		tokenQuery := ctx.Query("token")
		if tokenQuery == "" {
			return "", fmt.Errorf("unauthorized")
		}
		authorization = "Bearer " + tokenQuery
	}

	tokenParts := strings.Split(authorization, "Bearer ")
	if len(tokenParts) < 2 {
		return "", fmt.Errorf("unauthorized")
	}

	return tokenParts[1], nil
}

// Validate the token and retrieve user information
func (c *AuthMiddlewareConfig) validateToken(ctx *gin.Context) (*pb.ValidateResponse, error) {
	token, err := c.extractToken(ctx)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, Authorization{AuthorizationStatus: false, Error: "unauthorized"})
		return nil, err
	}

	res, err := c.svc.Client.Validate(context.Background(), &pb.ValidateRequest{Token: token})
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, Authorization{AuthorizationStatus: false, Error: "unauthorized"})
		return nil, err
	}

	ctx.Set("userName", res.UserName)
	return res, nil
}

// Middleware to check basic authorization
func (c *AuthMiddlewareConfig) AuthRequired(ctx *gin.Context) {
	if _, err := c.validateToken(ctx); err != nil {
		return
	}

	ctx.Next()
}

// Middleware to check authorization with specific access level
func (c *AuthMiddlewareConfig) AuthzRequired(ctx *gin.Context) {
	res, err := c.validateToken(ctx)
	if err != nil {
		return
	}

	blogID := ctx.Param("blog_id")
	userName := res.UserName
	email := ctx.Param("email")
	accountID := ctx.Param("account_id")
	// userID := ctx.Param("user_id")

	accessResp, err := c.svc.Client.CheckAccessLevel(context.Background(), &pb.AccessCheckReq{
		// Token:     res,
		Email:     email,
		AccountId: accountID,
		UserName:  userName,
		BlogId:    blogID,
	})

	if err != nil || accessResp.StatusCode != http.StatusOK {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	ctx.Set("user_access_level", accessResp.Access)

	ctx.Next()
}

func (c *AuthMiddlewareConfig) CanPublish(ctx *gin.Context) {
	// TODO: Check if the user can publish access
	logrus.Infof("The user has published access to the blog!")
	ctx.Next()
}

func (c *AuthMiddlewareConfig) CheckWriteAccess(ctx *gin.Context) {
	// TODO: Check if the user can publish access
	logrus.Infof("The user has write/edit access to the blog!")
	ctx.Next()
}
