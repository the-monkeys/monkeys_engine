package utils

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	authPb "github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_authz/pb"
	blogPb "github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_blog/pb"
)

// Utility functions for parsing client data

// parseAcceptLanguage parses the Accept-Language header and returns preferred languages
func parseAcceptLanguage(acceptLang string) []string {
	if acceptLang == "" {
		return []string{}
	}

	languages := []string{}
	parts := strings.Split(acceptLang, ",")

	for _, part := range parts {
		// Remove quality values (e.g., "en-US;q=0.8" -> "en-US")
		lang := strings.TrimSpace(strings.Split(part, ";")[0])
		if lang != "" {
			languages = append(languages, lang)
		}
	}

	return languages
}

// inferCountryFromLanguage attempts to infer country from Accept-Language header
func inferCountryFromLanguage(acceptLang string) string {
	if acceptLang == "" {
		return ""
	}

	// Get first language preference
	firstLang := strings.Split(acceptLang, ",")[0]
	firstLang = strings.TrimSpace(strings.Split(firstLang, ";")[0])

	// Extract country code from language-country format (e.g., en-US -> US)
	if parts := strings.Split(firstLang, "-"); len(parts) > 1 {
		return strings.ToUpper(parts[1])
	}

	return ""
}

// detectBotFromUserAgent checks if the User-Agent suggests a bot
func detectBotFromUserAgent(userAgent string) bool {
	if userAgent == "" {
		return false
	}

	userAgentLower := strings.ToLower(userAgent)
	botKeywords := []string{"bot", "crawler", "spider", "scraper", "wget", "curl", "python-requests"}

	for _, keyword := range botKeywords {
		if strings.Contains(userAgentLower, keyword) {
			return true
		}
	}

	return false
}

// calculateBasicTrustScore provides a simple trust score based on available data
func calculateBasicTrustScore(userAgent, referrer string, isBot bool) float64 {
	score := 1.0 // Start with neutral score

	if isBot {
		score *= 0.3 // Bots get lower trust score
	}

	if userAgent == "" {
		score *= 0.5 // Missing user agent is suspicious
	}

	if referrer != "" && !strings.Contains(referrer, "direct") {
		score *= 1.2 // Having a referrer increases trust slightly
	}

	// Ensure score is between 0 and 1
	if score > 1.0 {
		score = 1.0
	}
	if score < 0.0 {
		score = 0.0
	}

	return score
}

// getBrowserEngine attempts to identify browser engine from User-Agent
func getBrowserEngine(userAgent string) string {
	if userAgent == "" {
		return "unknown"
	}

	userAgentLower := strings.ToLower(userAgent)

	switch {
	case strings.Contains(userAgentLower, "gecko") && !strings.Contains(userAgentLower, "webkit"):
		return "gecko"
	case strings.Contains(userAgentLower, "webkit"):
		if strings.Contains(userAgentLower, "blink") || strings.Contains(userAgentLower, "chrome") {
			return "blink"
		}
		return "webkit"
	case strings.Contains(userAgentLower, "trident") || strings.Contains(userAgentLower, "msie"):
		return "trident"
	case strings.Contains(userAgentLower, "presto"):
		return "presto"
	default:
		return "unknown"
	}
}

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

func SetMonkeysRefreshCookie(ctx *gin.Context, token string) {
	refreshCookie := &http.Cookie{
		Name:     "mrt",
		Value:    token,
		HttpOnly: true,
		Path:     "/",
		MaxAge:   int(time.Duration(24*72)*time.Hour) / int(time.Second),
		SameSite: http.SameSiteNoneMode,
		Secure:   true,
	}

	http.SetCookie(ctx.Writer, refreshCookie)
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

// GetIntQuery extracts an integer from query parameters with a default value
func GetIntQuery(ctx *gin.Context, key string, defaultValue int) int {
	val := ctx.Query(key)
	if val == "" {
		return defaultValue
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return defaultValue
	}
	return i
}

// GetClientInfo extracts comprehensive client information from request headers
type ClientInfo struct {
	// Basic information (existing)
	IPAddress      string
	UserAgent      string
	Referrer       string
	ClientType     string // Better name than just "Client"
	SessionID      string // Session identifier from context or generated
	VisitorID      string // Visitor identifier for tracking across sessions
	Platform       string // Platform category (web, mobile, tablet, etc.)
	Origin         string
	RealIP         string
	Browser        string
	Accept         string
	Connection     string
	DeviceType     string
	Os             string
	ForwardedFor   string
	ForwardedHost  string
	ForwardedProto string

	// Browser fingerprinting
	AcceptLanguage   string   // Preferred languages
	AcceptEncoding   string   // Supported encodings
	DNT              string   // Do Not Track header
	Timezone         string   // User's timezone (from custom header)
	ScreenResolution string   // Screen resolution (from custom header)
	ColorDepth       string   // Color depth (from custom header)
	DeviceMemory     string   // Device memory (from Device-Memory header)
	Languages        []string // Parsed Accept-Language preferences

	// Location & Geographic hints
	Country        string // Inferred from Accept-Language or IP
	TimezoneOffset string // UTC offset (from custom header)

	// Marketing & UTM tracking
	UTMSource   string // Marketing source
	UTMMedium   string // Marketing medium
	UTMCampaign string // Campaign name
	UTMContent  string // Content identifier
	UTMTerm     string // Keyword term

	// Behavioral indicators
	IsBot        bool    // Bot detection flag
	TrustScore   float64 // Basic trust scoring
	RequestCount int     // Requests in current session

	// Technical environment
	IsSecureContext   bool   // HTTPS connection
	ConnectionType    string // Connection type hint
	BrowserEngine     string // Browser engine (Webkit, Gecko, Blink)
	JavaScriptEnabled bool   // JavaScript support (inferred)

	// Timestamps
	FirstSeen   string // First request timestamp
	LastSeen    string // Current request timestamp
	CollectedAt string // When this data was collected
}

// GetClientInfo returns structured client information from the request
func GetClientInfo(ctx *gin.Context) ClientInfo {
	// Get basic information
	ipAddress := GetClientIP(ctx)
	userAgent := ctx.Request.UserAgent()
	referrer := ctx.Request.Referer()
	clientType := getClientType(ctx)
	sessionID := getSessionID(ctx)
	visitorID := getVisitorID(ctx)
	platform := getPlatform(ctx)
	browser := getBrowser(ctx)
	dnt := getDNT(ctx)

	// Get enhanced browser information
	accept, acceptLanguage, acceptEncoding, timezone, screenRes, languages := getEnhancedBrowserInfo(ctx)

	// Get location information
	country, timezoneOffset := getLocationInfo(ctx)

	// Get UTM parameters
	utmSource, utmMedium, utmCampaign, utmContent, utmTerm := getUTMParameters(ctx)

	// Get behavioral indicators
	isBot, trustScore, requestCount := getBehavioralIndicators(ctx)

	// Get technical environment
	isSecure, connectionType, browserEngine, jsEnabled := getTechnicalEnvironment(ctx)

	// Get timestamps
	firstSeen, lastSeen, collectedAt := getTimestamps(ctx)

	ci := ClientInfo{
		// Basic information (existing)
		IPAddress:      ipAddress,
		UserAgent:      userAgent,
		Referrer:       referrer,
		ClientType:     clientType,
		SessionID:      sessionID,
		VisitorID:      visitorID,
		Platform:       platform,
		Browser:        browser,
		Origin:         ctx.Request.Header.Get("Origin"),
		RealIP:         GetClientIP(ctx),
		DeviceType:     ctx.Request.Header.Get("X-Device"),
		Os:             ctx.Request.Header.Get("X-Os"),
		Accept:         accept,
		Connection:     ctx.Request.Header.Get("Connection"),
		ForwardedFor:   ctx.Request.Header.Get("X-Forwarded-For"),
		ForwardedHost:  ctx.Request.Header.Get("X-Forwarded-Host"),
		ForwardedProto: ctx.Request.Header.Get("X-Forwarded-Proto"),
		DNT:            dnt,

		// Browser fingerprinting
		AcceptLanguage:   acceptLanguage,
		AcceptEncoding:   acceptEncoding,
		Timezone:         timezone,
		ScreenResolution: screenRes,
		ColorDepth:       ctx.Request.Header.Get("X-Color-Depth"),
		DeviceMemory:     ctx.Request.Header.Get("X-Device-Memory"),
		Languages:        languages,

		// Location & Geographic hints
		Country:        country,
		TimezoneOffset: timezoneOffset,

		// Marketing & UTM tracking
		UTMSource:   utmSource,
		UTMMedium:   utmMedium,
		UTMCampaign: utmCampaign,
		UTMContent:  utmContent,
		UTMTerm:     utmTerm,

		// Behavioral indicators
		IsBot:        isBot,
		TrustScore:   trustScore,
		RequestCount: requestCount,

		// Technical environment
		IsSecureContext:   isSecure,
		ConnectionType:    connectionType,
		BrowserEngine:     browserEngine,
		JavaScriptEnabled: jsEnabled,

		// Timestamps
		FirstSeen:   firstSeen,
		LastSeen:    lastSeen,
		CollectedAt: collectedAt,
	}

	return ci
}

// getDNT determines DNT (Do Not Track) from headers with fallback
func getDNT(ctx *gin.Context) string {
	dnt := ctx.Request.Header.Get("Sec-Gpc")
	if dnt == "" {
		dnt = ctx.Request.Header.Get("Dnt")
	}
	return dnt
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
	// get session ID from header first
	if sessionID := ctx.Request.Header.Get("X-Session-ID"); sessionID != "" {
		return sessionID
	}
	return ""
}

func getVisitorID(ctx *gin.Context) string {
	if visitorID := ctx.Request.Header.Get("X-Visitor-ID"); visitorID != "" {
		return visitorID
	}
	return ""
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

func getBrowser(ctx *gin.Context) string {
	// First try the X-Browser header
	if browser := ctx.Request.Header.Get("X-Browser"); browser != "" {
		return browser
	}

	// Fallback to Sec-Ch-Ua header
	uaList := ctx.Request.Header["Sec-Ch-Ua"]
	if len(uaList) == 0 {
		return "unknown"
	}

	// Priority order
	preferred := []string{"Brave", "Chrome", "Chromium", "Firefox", "Edge", "Safari"}

	// Combine all lines into one string
	raw := strings.Join(uaList, ", ")

	// Split by commas
	parts := strings.SplitSeq(raw, ",")

	for p := range parts {
		p = strings.TrimSpace(p)

		// Extract the browser name inside quotes
		if strings.HasPrefix(p, `"`) {
			end := strings.Index(p[1:], `"`)
			if end != -1 {
				name := p[1 : end+1] // correctly extract the quoted name

				// Skip noise brands
				if strings.EqualFold(name, "Not_A Brand") || strings.EqualFold(name, "Not A(Brand") {
					continue
				}

				// Return the first preferred match
				for _, b := range preferred {
					if strings.EqualFold(name, b) {
						return b
					}
				}
			}
		}
	}

	return "unknown"
}

// getEnhancedBrowserInfo extracts additional browser fingerprinting data
func getEnhancedBrowserInfo(ctx *gin.Context) (string, string, string, string, string, []string) {
	accept := ctx.Request.Header.Get("Accept")
	acceptLanguage := ctx.Request.Header.Get("Accept-Language")
	acceptEncoding := ctx.Request.Header.Get("Accept-Encoding")
	timezone := ctx.Request.Header.Get("X-Timezone")
	screenRes := ctx.Request.Header.Get("X-Screen-Resolution")
	languages := parseAcceptLanguage(acceptLanguage)

	return accept, acceptLanguage, acceptEncoding, timezone, screenRes, languages
}

// getLocationInfo extracts location-related information
func getLocationInfo(ctx *gin.Context) (string, string) {
	acceptLanguage := ctx.Request.Header.Get("Accept-Language")
	country := inferCountryFromLanguage(acceptLanguage)
	timezoneOffset := ctx.Request.Header.Get("X-Timezone-Offset")

	return country, timezoneOffset
}

// getUTMParameters extracts UTM tracking parameters from query string
func getUTMParameters(ctx *gin.Context) (string, string, string, string, string) {
	return ctx.Query("utm_source"),
		ctx.Query("utm_medium"),
		ctx.Query("utm_campaign"),
		ctx.Query("utm_content"),
		ctx.Query("utm_term")
}

// getBehavioralIndicators calculates behavioral indicators
func getBehavioralIndicators(ctx *gin.Context) (bool, float64, int) {
	userAgent := ctx.Request.UserAgent()
	referrer := ctx.Request.Referer()

	isBot := detectBotFromUserAgent(userAgent)
	trustScore := calculateBasicTrustScore(userAgent, referrer, isBot)

	// Try to get request count from session/context (placeholder implementation)
	requestCount := 1 // This would typically come from session data
	if reqCountStr := ctx.Request.Header.Get("X-Request-Count"); reqCountStr != "" {
		if count, err := strconv.Atoi(reqCountStr); err == nil {
			requestCount = count
		}
	}

	return isBot, trustScore, requestCount
}

// getTechnicalEnvironment extracts technical environment information
func getTechnicalEnvironment(ctx *gin.Context) (bool, string, string, bool) {
	isSecure := ctx.Request.TLS != nil
	connectionType := ctx.Request.Header.Get("X-Connection-Type")
	userAgent := ctx.Request.UserAgent()
	browserEngine := getBrowserEngine(userAgent)

	// Assume JavaScript is enabled for web browsers (most common case)
	jsEnabled := !strings.Contains(strings.ToLower(userAgent), "curl") &&
		!strings.Contains(strings.ToLower(userAgent), "wget")

	return isSecure, connectionType, browserEngine, jsEnabled
}

// getTimestamps generates timestamp information
func getTimestamps(ctx *gin.Context) (string, string, string) {
	now := time.Now()
	currentTime := now.Format(time.RFC3339)

	// Try to get first seen from header/session (placeholder)
	firstSeen := ctx.Request.Header.Get("X-First-Seen")
	if firstSeen == "" {
		firstSeen = currentTime // First time seeing this client
	}

	return firstSeen, currentTime, currentTime
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
