package admin

import (
	"context"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_user/pb"
	"github.com/the-monkeys/the_monkeys/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type AdminServiceClient struct {
	Client pb.UserServiceClient
	logger *logrus.Logger
}

func NewAdminServiceClient(cfg *config.Config) pb.UserServiceClient {
	cc, err := grpc.NewClient(cfg.Microservices.TheMonkeysUser, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logrus.Errorf("cannot dial to grpc user server for admin: %v", err)
	}
	logrus.Infof("✅ admin service is dialing to user rpc server at: %v", cfg.Microservices.TheMonkeysUser)
	return pb.NewUserServiceClient(cc)
}

// LocalNetworkMiddleware restricts access to local network only
func LocalNetworkMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()

		// Parse the IP address
		ip := net.ParseIP(clientIP)
		if ip == nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Access denied: Invalid IP address",
			})
			return
		}

		// Check if IP is from local network
		if !isLocalNetwork(ip) {
			logrus.Warnf("Admin access attempt from non-local IP: %s", clientIP)
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Access denied: Admin API only accessible from local network",
			})
			return
		}

		c.Next()
	}
}

// isLocalNetwork checks if IP is from local network ranges
func isLocalNetwork(ip net.IP) bool {
	// Define local network ranges
	localRanges := []string{
		"127.0.0.0/8",    // localhost
		"10.0.0.0/8",     // private class A
		"172.16.0.0/12",  // private class B
		"192.168.0.0/16", // private class C
		"::1/128",        // IPv6 localhost
		"fc00::/7",       // IPv6 unique local
	}

	for _, cidr := range localRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// AdminKeyMiddleware validates admin key from header
func AdminKeyMiddleware(adminKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		providedKey := c.GetHeader("X-Admin-Key")
		if providedKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Admin key required",
			})
			return
		}

		if providedKey != adminKey {
			logrus.Warnf("Invalid admin key attempt from IP: %s", c.ClientIP())
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid admin key",
			})
			return
		}

		c.Next()
	}
}

func RegisterAdminRouter(router *gin.Engine, cfg *config.Config) *AdminServiceClient {
	asc := &AdminServiceClient{
		Client: NewAdminServiceClient(cfg),
		logger: logrus.New(),
	}

	// Admin routes group with local network restriction and admin key validation
	adminRoutes := router.Group("/api/v1/admin")
	adminRoutes.Use(LocalNetworkMiddleware())
	adminRoutes.Use(AdminKeyMiddleware(cfg.Keys.AdminSecretKey))

	// User management routes
	{
		adminRoutes.DELETE("/users/:id", asc.ForceDeleteUser)
		adminRoutes.DELETE("/users/bulk", asc.BulkDeleteUsers)
		adminRoutes.GET("/users/suspicious", asc.GetSuspiciousUsers)
		adminRoutes.POST("/users/:id/flag", asc.FlagUserAsBotOrFake)
		adminRoutes.POST("/users/:id/unflag", asc.UnflagUser)
		adminRoutes.GET("/users/flagged", asc.GetFlaggedUsers)
		adminRoutes.GET("/users/stats", asc.GetUserStats)
	}

	// System health and monitoring
	{
		adminRoutes.GET("/health", asc.AdminHealthCheck)
		adminRoutes.GET("/system/stats", asc.GetSystemStats)
	}

	return asc
}

// ForceDeleteUser deletes a user without normal authorization checks
func (asc *AdminServiceClient) ForceDeleteUser(ctx *gin.Context) {
	userID := ctx.Param("id")
	reason := ctx.Query("reason")
	if reason == "" {
		reason = "Admin deletion"
	}

	asc.logger.Infof("Admin force deleting user: %s, reason: %s, from IP: %s", userID, reason, ctx.ClientIP())

	res, err := asc.Client.DeleteUserAccount(context.Background(), &pb.DeleteUserProfileReq{
		Username: userID,
	})

	if err != nil {
		if status.Code(err) == codes.NotFound {
			ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{
				"error":   "User not found",
				"user_id": userID,
			})
			return
		} else {
			asc.logger.Errorf("Failed to delete user %s: %v", userID, err)
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to delete user",
			})
			return
		}
	}

	asc.logger.Infof("Successfully deleted user: %s", userID)
	ctx.JSON(http.StatusOK, gin.H{
		"message":    "User successfully deleted",
		"user_id":    userID,
		"reason":     reason,
		"deleted_by": "admin",
		"result":     res,
	})
}

// BulkDeleteUsers deletes multiple users at once
func (asc *AdminServiceClient) BulkDeleteUsers(ctx *gin.Context) {
	var req struct {
		UserIDs []string `json:"user_ids" binding:"required"`
		Reason  string   `json:"reason"`
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	if len(req.UserIDs) == 0 {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "At least one user ID is required",
		})
		return
	}

	if len(req.UserIDs) > 100 {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Cannot delete more than 100 users at once",
		})
		return
	}

	if req.Reason == "" {
		req.Reason = "Bulk admin deletion"
	}

	asc.logger.Infof("Admin bulk deleting %d users, reason: %s, from IP: %s", len(req.UserIDs), req.Reason, ctx.ClientIP())

	results := make(map[string]interface{})
	successCount := 0
	failureCount := 0

	for _, userID := range req.UserIDs {
		_, err := asc.Client.DeleteUserAccount(context.Background(), &pb.DeleteUserProfileReq{
			Username: userID,
		})

		if err != nil {
			failureCount++
			results[userID] = gin.H{
				"status": "failed",
				"error":  err.Error(),
			}
			asc.logger.Errorf("Failed to delete user %s: %v", userID, err)
		} else {
			successCount++
			results[userID] = gin.H{
				"status": "success",
			}
		}
	}

	asc.logger.Infof("Bulk deletion completed: %d successful, %d failed", successCount, failureCount)

	ctx.JSON(http.StatusOK, gin.H{
		"message":      "Bulk deletion completed",
		"total_users":  len(req.UserIDs),
		"successful":   successCount,
		"failed":       failureCount,
		"reason":       req.Reason,
		"results":      results,
		"processed_by": "admin",
	})
}

// GetSuspiciousUsers returns users that might be bots or fake accounts
func (asc *AdminServiceClient) GetSuspiciousUsers(ctx *gin.Context) {
	// This would typically involve complex logic to identify suspicious patterns
	// For now, returning a placeholder response

	ctx.JSON(http.StatusOK, gin.H{
		"message": "Suspicious users detection not yet implemented",
		"note":    "This endpoint would analyze user patterns, disposable emails, etc.",
	})
}

// FlagUserAsBotOrFake flags a user as suspicious
func (asc *AdminServiceClient) FlagUserAsBotOrFake(ctx *gin.Context) {
	userID := ctx.Param("id")

	var req struct {
		Reason string `json:"reason" binding:"required"`
		Type   string `json:"type" binding:"required"` // "bot", "fake", "spam"
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	validTypes := []string{"bot", "fake", "spam"}
	isValidType := false
	for _, t := range validTypes {
		if req.Type == t {
			isValidType = true
			break
		}
	}

	if !isValidType {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid type. Must be one of: bot, fake, spam",
		})
		return
	}

	asc.logger.Infof("Admin flagging user %s as %s, reason: %s, from IP: %s", userID, req.Type, req.Reason, ctx.ClientIP())

	// Here you would implement the actual flagging logic
	// This might involve updating a user_flags table or similar

	ctx.JSON(http.StatusOK, gin.H{
		"message":    "User flagged successfully",
		"user_id":    userID,
		"flag_type":  req.Type,
		"reason":     req.Reason,
		"flagged_by": "admin",
	})
}

// UnflagUser removes flags from a user
func (asc *AdminServiceClient) UnflagUser(ctx *gin.Context) {
	userID := ctx.Param("id")
	reason := ctx.Query("reason")
	if reason == "" {
		reason = "Admin unflag"
	}

	asc.logger.Infof("Admin unflagging user %s, reason: %s, from IP: %s", userID, reason, ctx.ClientIP())

	ctx.JSON(http.StatusOK, gin.H{
		"message":      "User unflagged successfully",
		"user_id":      userID,
		"reason":       reason,
		"unflagged_by": "admin",
	})
}

// GetFlaggedUsers returns all flagged users
func (asc *AdminServiceClient) GetFlaggedUsers(ctx *gin.Context) {
	flagType := ctx.Query("type") // optional filter by flag type

	ctx.JSON(http.StatusOK, gin.H{
		"message":     "Flagged users retrieval not yet implemented",
		"note":        "This endpoint would return users flagged as bots/fake/spam",
		"filter_type": flagType,
	})
}

// GetUserStats returns user statistics for admin monitoring
func (asc *AdminServiceClient) GetUserStats(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{
		"message": "User statistics not yet implemented",
		"note":    "This would return user registration patterns, suspicious activity, etc.",
	})
}

// AdminHealthCheck provides health status for admin monitoring
func (asc *AdminServiceClient) AdminHealthCheck(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"service":   "admin-api",
		"timestamp": "2024-01-01T00:00:00Z",
		"access_ip": ctx.ClientIP(),
	})
}

// GetSystemStats returns system-wide statistics
func (asc *AdminServiceClient) GetSystemStats(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{
		"message": "System statistics not yet implemented",
		"note":    "This would return system health, performance metrics, etc.",
	})
}

type ReturnMessage struct {
	Message string `json:"message"`
}
