package utils

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	authPb "github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_authz/pb"
	blogPb "github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_blog/pb"
)

// CheckUserAccessLevel checks if a specific access level is present in the user_access_level []string.
func CheckUserAccessLevel(accessLevels []string, accessToCheck string) bool {
	for _, access := range accessLevels {
		if access == accessToCheck {
			return true
		}
	}
	return false
}

func CheckUserAccessInContext(ctx *gin.Context, accessToCheck string) bool {
	accessValue, exists := ctx.Get("user_access_level")
	if !exists {
		fmt.Println("user_access_level not found in context")
		return false
	}
	accessLevels, ok := accessValue.([]string)
	if !ok {
		fmt.Println("user_access_level is not of type []string")
		return false
	}
	return CheckUserAccessLevel(accessLevels, accessToCheck)
}

func CheckUserRoleInContext(ctx *gin.Context, role string) bool {
	return strings.EqualFold(ctx.GetString("user_role"), role)
}

func SetMonkeysAuthCookie(ctx *gin.Context, token string) {
	authCookie := &http.Cookie{
		Name:     "mat",
		Value:    token,
		HttpOnly: true,
		Path:     "/",
		MaxAge:   int(time.Duration(24*30)*time.Hour) / int(time.Second),
		SameSite: http.SameSiteNoneMode,
		Secure:   true,
	}

	http.SetCookie(ctx.Writer, authCookie)
}

// GetClientIP extracts the real client IP address from various headers
// Priority: X-Forwarded-For > X-Real-IP > ClientIP() fallback
func GetClientIP(ctx *gin.Context) string {
	// Try X-Forwarded-For first (most common proxy header)
	if xForwardedFor := ctx.Request.Header.Get("X-Forwarded-For"); xForwardedFor != "" {
		// Take the first IP if multiple are present (client -> proxy1 -> proxy2)
		if idx := strings.Index(xForwardedFor, ","); idx > 0 {
			return strings.TrimSpace(xForwardedFor[:idx])
		}
		return strings.TrimSpace(xForwardedFor)
	}

	// Try X-Real-IP (nginx real IP module)
	if xRealIP := ctx.Request.Header.Get("X-Real-IP"); xRealIP != "" {
		return strings.TrimSpace(xRealIP)
	}

	// Fallback to Gin's built-in method
	return ctx.ClientIP()
}

// GetClientInfo extracts comprehensive client information from request headers
type ClientInfo struct {
	IPAddress  string
	UserAgent  string
	Referrer   string
	ClientType string // Better name than just "Client"
	SessionID  string // Session identifier from context or generated
	Platform   string // Platform category (web, mobile, tablet, etc.)
}

// GetClientInfo returns structured client information from the request
func GetClientInfo(ctx *gin.Context) ClientInfo {
	return ClientInfo{
		IPAddress:  GetClientIP(ctx),
		UserAgent:  ctx.Request.UserAgent(),
		Referrer:   ctx.Request.Referer(),
		ClientType: getClientType(ctx),
		SessionID:  getSessionID(ctx),
		Platform:   getPlatform(ctx),
	}
}

// getClientType determines client type from headers with fallback
func getClientType(ctx *gin.Context) string {
	// Check for custom Client-Type header first
	if clientType := ctx.Request.Header.Get("Client-Type"); clientType != "" {
		return clientType
	}

	// Check legacy "Client" header for backward compatibility
	if client := ctx.Request.Header.Get("Client"); client != "" {
		return client
	}

	// Determine from User-Agent as fallback
	userAgent := strings.ToLower(ctx.Request.UserAgent())
	switch {
	case strings.Contains(userAgent, "mobile"):
		return "mobile"
	case strings.Contains(userAgent, "tablet"):
		return "tablet"
	case strings.Contains(userAgent, "electron"):
		return "desktop-app"
	case strings.Contains(userAgent, "postman"):
		return "api-testing"
	case strings.Contains(userAgent, "curl"):
		return "cli"
	case strings.Contains(userAgent, "bot"):
		return "bot"
	default:
		return "web"
	}
}

// getSessionID extracts or generates session ID from request
func getSessionID(ctx *gin.Context) string {
	// Try to get session ID from header first
	if sessionID := ctx.Request.Header.Get("X-Session-ID"); sessionID != "" {
		return sessionID
	}

	// Try to get from context (if set by middleware)
	if sessionID, exists := ctx.Get("session_id"); exists {
		if id, ok := sessionID.(string); ok {
			return id
		}
	}

	// Try to get from cookie
	if cookie, err := ctx.Request.Cookie("session_id"); err == nil {
		return cookie.Value
	}

	// Generate a simple session ID if none found (basic implementation)
	// In production, this should be handled by proper session middleware
	return fmt.Sprintf("sess_%d", time.Now().UnixNano())
}

// getPlatform determines platform category from user agent
func getPlatform(ctx *gin.Context) string {
	// Check for custom Platform header first
	if platform := ctx.Request.Header.Get("X-Platform"); platform != "" {
		return platform
	}

	// Determine from User-Agent
	userAgent := strings.ToLower(ctx.Request.UserAgent())
	switch {
	case strings.Contains(userAgent, "mobile") || strings.Contains(userAgent, "android") || strings.Contains(userAgent, "iphone"):
		return "PLATFORM_MOBILE"
	case strings.Contains(userAgent, "tablet") || strings.Contains(userAgent, "ipad"):
		return "PLATFORM_TABLET"
	case strings.Contains(userAgent, "electron"):
		return "PLATFORM_DESKTOP"
	case strings.Contains(userAgent, "postman") || strings.Contains(userAgent, "curl") || strings.Contains(userAgent, "insomnia"):
		return "PLATFORM_API"
	default:
		return "PLATFORM_WEB"
	}
}

// GetAuthPlatform converts platform string to auth protobuf enum
func GetAuthPlatform(ctx *gin.Context) authPb.Platform {
	platformStr := getPlatform(ctx)
	switch platformStr {
	case "PLATFORM_WEB":
		return authPb.Platform_PLATFORM_WEB
	case "PLATFORM_MOBILE":
		return authPb.Platform_PLATFORM_MOBILE
	case "PLATFORM_TABLET":
		return authPb.Platform_PLATFORM_TABLET
	case "PLATFORM_API":
		return authPb.Platform_PLATFORM_API
	case "PLATFORM_DESKTOP":
		return authPb.Platform_PLATFORM_DESKTOP
	default:
		return authPb.Platform_PLATFORM_UNSPECIFIED
	}
}

// GetBlogPlatform converts platform string to blog protobuf enum
func GetBlogPlatform(ctx *gin.Context) blogPb.Platform {
	platformStr := getPlatform(ctx)
	switch platformStr {
	case "PLATFORM_WEB":
		return blogPb.Platform_PLATFORM_WEB
	case "PLATFORM_MOBILE":
		return blogPb.Platform_PLATFORM_MOBILE
	case "PLATFORM_TABLET":
		return blogPb.Platform_PLATFORM_TABLET
	case "PLATFORM_API":
		return blogPb.Platform_PLATFORM_API
	case "PLATFORM_DESKTOP":
		return blogPb.Platform_PLATFORM_DESKTOP
	default:
		return blogPb.Platform_PLATFORM_UNSPECIFIED
	}
}
