package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	activitypb "github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_activity/pb"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_blog/pb"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/constants"
	"github.com/the-monkeys/the_monkeys/microservices/rabbitmq"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_blog/internal/database"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_blog/internal/models"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_blog/internal/seo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

type BlogService struct {
	osClient   database.ElasticsearchStorage
	seoManager seo.SEOManager
	logger     *zap.SugaredLogger
	config     *config.Config
	qConn      rabbitmq.Conn
	pb.UnimplementedBlogServiceServer
}

func NewBlogService(client database.ElasticsearchStorage, seoManager seo.SEOManager, logger *zap.SugaredLogger, config *config.Config, qConn rabbitmq.Conn) *BlogService {
	return &BlogService{
		osClient:   client,
		seoManager: seoManager,
		logger:     logger,
		config:     config,
		qConn:      qConn,
	}
}

// Helper method to generate session ID
func (blog *BlogService) generateSessionID() string {
	return fmt.Sprintf("session_%d_%s", time.Now().UnixNano(), generateGUID()[:8])
}

// Helper method to generate GUID (simple version)
func generateGUID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// Helper method to extract UTM parameters from referrer URL
func (blog *BlogService) extractUTMFromReferrer(referrer, utmParam string) string {
	if referrer == "" {
		return ""
	}

	// Simple URL parameter extraction
	paramPrefix := utmParam + "="
	if idx := strings.Index(referrer, paramPrefix); idx != -1 {
		start := idx + len(paramPrefix)
		end := strings.IndexAny(referrer[start:], "&# ")
		if end == -1 {
			return referrer[start:]
		}
		return referrer[start : start+end]
	}
	return ""
}

// ComprehensiveClientInfo represents all client information extracted from requests
type ComprehensiveClientInfo struct {
	// Basic client information
	IPAddress string
	Client    string
	SessionID string
	UserAgent string
	Referrer  string
	Platform  pb.Platform

	// Enhanced Browser fingerprinting
	AcceptLanguage   string
	AcceptEncoding   string
	DNT              string
	Timezone         string
	ScreenResolution string
	ColorDepth       string
	DeviceMemory     string
	Languages        []string

	// Location & Geographic hints
	Country        string
	TimezoneOffset string

	// Marketing & UTM tracking
	UTMSource   string
	UTMMedium   string
	UTMCampaign string
	UTMContent  string
	UTMTerm     string

	// Behavioral indicators
	IsBot        bool
	TrustScore   float64
	RequestCount int32

	// Technical environment
	IsSecureContext   bool
	ConnectionType    string
	BrowserEngine     string
	JavaScriptEnabled bool

	// Timestamps
	FirstSeen      string
	LastSeen       string
	CollectedAt    string
	Origin         string
	RealIp         string
	ForwardedFor   string
	ForwardedProto string
	ForwardedHost  string
	ForwardedPort  string
	Os             string
	Browser        string
	Device         string
	Accept         string
	Connection     string
	Referer        string
}

// Helper method to extract comprehensive client info from any request type
func (blog *BlogService) extractClientInfo(req interface{}) *ComprehensiveClientInfo {
	var clientInfo *pb.ClientInfo
	var sessionID, ipAddress, userAgent, referrer, client string
	var platform pb.Platform

	// Extract ClientInfo from different request types
	switch r := req.(type) {
	case *pb.BlogReq:
		if r != nil && r.GetClientInfo() != nil {
			clientInfo = r.GetClientInfo()
		} else if r != nil {
			// Fallback to individual fields if ClientInfo not available
			sessionID = r.GetSessionId()
			ipAddress = r.GetIp()
			userAgent = r.GetUserAgent()
			referrer = r.GetReferrer()
			platform = r.GetPlatform()
			client = r.GetClient()
		}
	case *pb.DraftBlogRequest:
		if r != nil && r.GetClientInfo() != nil {
			clientInfo = r.GetClientInfo()
		} else if r != nil {
			// Fallback to individual fields if ClientInfo not available
			sessionID = r.GetSessionId()
			ipAddress = r.GetIp()
			userAgent = r.GetUserAgent()
			referrer = r.GetReferrer()
			platform = r.GetPlatform()
			client = r.GetClient()
		}
	case *pb.PublishBlogReq:
		if r != nil && r.GetClientInfo() != nil {
			clientInfo = r.GetClientInfo()
		} else if r != nil {
			// Fallback to individual fields if ClientInfo not available
			sessionID = r.GetSessionId()
			ipAddress = r.GetIp()
			userAgent = r.GetUserAgent()
			referrer = r.GetReferrer()
			platform = r.GetPlatform()
			client = r.GetClient()
		}
	case *pb.DeleteBlogReq:
		if r != nil && r.GetClientInfo() != nil {
			clientInfo = r.GetClientInfo()
		} else if r != nil {
			// Fallback to individual fields if ClientInfo not available
			sessionID = r.GetSessionId()
			ipAddress = r.GetIp()
			userAgent = r.GetUserAgent()
			referrer = r.GetReferrer()
			platform = r.GetPlatform()
			client = r.GetClient()
		}
	case *pb.BlogListReq:
		if r != nil && r.GetClientInfo() != nil {
			clientInfo = r.GetClientInfo()
		} else if r != nil {
			// Fallback to individual fields if ClientInfo not available
			sessionID = r.GetSessionId()
			ipAddress = r.GetIp()
			userAgent = r.GetUserAgent()
			referrer = r.GetReferrer()
			platform = r.GetPlatform()
			client = r.GetClient()
		}
	case *pb.SearchReq:
		if r != nil && r.GetClientInfo() != nil {
			clientInfo = r.GetClientInfo()
		} else if r != nil {
			// Fallback to individual fields if ClientInfo not available
			sessionID = r.GetSessionId()
			ipAddress = r.GetIp()
			userAgent = r.GetUserAgent()
			referrer = r.GetReferrer()
			platform = r.GetPlatform()
			client = r.GetClient()
		}
	case *pb.BlogByIdReq:
		if r != nil && r.GetClientInfo() != nil {
			clientInfo = r.GetClientInfo()
		} else {
			blog.logger.Debugf("BlogByIdReq doesn't have client tracking fields, using defaults")
		}
	case *pb.ArchiveBlogReq:
		if r != nil && r.GetClientInfo() != nil {
			clientInfo = r.GetClientInfo()
		} else {
			blog.logger.Debugf("ArchiveBlogReq doesn't have client tracking fields, using defaults")
		}
	case *pb.GetBlogsByTagsNameReq:
		if r != nil && r.GetClientInfo() != nil {
			clientInfo = r.GetClientInfo()
		}
	case *pb.GetBlogsReq:
		if r != nil && r.GetClientInfo() != nil {
			clientInfo = r.GetClientInfo()
		}
	case *pb.GetBlogsBySliceReq:
		if r != nil && r.GetClientInfo() != nil {
			clientInfo = r.GetClientInfo()
		}
	default:
		// Fallback for unknown request types
		blog.logger.Debugf("Unknown request type for client tracking: %T", req)
		return &ComprehensiveClientInfo{
			SessionID: blog.generateSessionID(),
			Platform:  pb.Platform_PLATFORM_UNSPECIFIED,
		}
	}

	// If we have comprehensive ClientInfo, use it
	if clientInfo != nil {
		// Generate session ID if not provided
		sessionIDFromClient := clientInfo.GetSessionId()
		if sessionIDFromClient == "" {
			sessionIDFromClient = blog.generateSessionID()
		}

		return &ComprehensiveClientInfo{
			// Basic client information
			IPAddress: clientInfo.GetIpAddress(),
			Client:    clientInfo.GetClient(),
			SessionID: sessionIDFromClient,
			UserAgent: clientInfo.GetUserAgent(),
			Referrer:  clientInfo.GetReferrer(),
			Platform:  clientInfo.GetPlatform(),

			// Enhanced Browser fingerprinting
			AcceptLanguage:   clientInfo.GetAcceptLanguage(),
			AcceptEncoding:   clientInfo.GetAcceptEncoding(),
			DNT:              clientInfo.GetDnt(),
			Timezone:         clientInfo.GetTimezone(),
			ScreenResolution: clientInfo.GetScreenResolution(),
			ColorDepth:       clientInfo.GetColorDepth(),
			DeviceMemory:     clientInfo.GetDeviceMemory(),
			Languages:        clientInfo.GetLanguages(),

			// Location & Geographic hints
			Country:        clientInfo.GetCountry(),
			TimezoneOffset: clientInfo.GetTimezoneOffset(),

			// Marketing & UTM tracking
			UTMSource:   clientInfo.GetUtmSource(),
			UTMMedium:   clientInfo.GetUtmMedium(),
			UTMCampaign: clientInfo.GetUtmCampaign(),
			UTMContent:  clientInfo.GetUtmContent(),
			UTMTerm:     clientInfo.GetUtmTerm(),

			// Behavioral indicators
			IsBot:        clientInfo.GetIsBot(),
			TrustScore:   float64(clientInfo.GetTrustScore()),
			RequestCount: clientInfo.GetRequestCount(),

			// Technical environment
			IsSecureContext:   clientInfo.GetIsSecureContext(),
			ConnectionType:    clientInfo.GetConnectionType(),
			BrowserEngine:     clientInfo.GetBrowserEngine(),
			JavaScriptEnabled: clientInfo.GetJavascriptEnabled(),
			Browser:           clientInfo.GetBrowser(),
			Accept:            clientInfo.GetAccept(),
			Connection:        clientInfo.GetConnection(),
			Origin:            clientInfo.GetOrigin(),
			Referer:           clientInfo.GetReferrer(),
			RealIp:            clientInfo.GetRealIp(),
			ForwardedFor:      clientInfo.GetForwardedFor(),
			ForwardedHost:     clientInfo.GetForwardedHost(),
			ForwardedPort:     clientInfo.GetForwardedPort(),
			ForwardedProto:    clientInfo.GetForwardedProto(),
			Os:                clientInfo.GetOs(),
			Device:            clientInfo.GetDeviceType(),

			// Timestamps
			FirstSeen:   clientInfo.GetFirstSeen(),
			LastSeen:    clientInfo.GetLastSeen(),
			CollectedAt: clientInfo.GetCollectedAt(),
		}
	}

	// Fallback to individual fields if ClientInfo not available
	// Generate session ID if not provided
	if sessionID == "" {
		sessionID = blog.generateSessionID()
	}

	// Extract what we can from UserAgent and other available fields
	var isBot bool
	var browserEngine, acceptLanguage string
	var isSecureContext, jsEnabled bool
	var trustScore float64

	if userAgent != "" {
		userAgentLower := strings.ToLower(userAgent)

		// Detect if it's a bot
		isBot = strings.Contains(userAgentLower, "bot") ||
			strings.Contains(userAgentLower, "crawler") ||
			strings.Contains(userAgentLower, "spider") ||
			strings.Contains(userAgentLower, "scraper")

		// Extract browser engine from user agent
		if strings.Contains(userAgentLower, "webkit") {
			browserEngine = "WebKit"
		} else if strings.Contains(userAgentLower, "gecko") {
			browserEngine = "Gecko"
		} else if strings.Contains(userAgentLower, "trident") {
			browserEngine = "Trident"
		} else if strings.Contains(userAgentLower, "edg") {
			browserEngine = "EdgeHTML"
		}

		// Assume JavaScript is enabled for web browsers (not for bots/API clients)
		jsEnabled = !isBot && (platform == pb.Platform_PLATFORM_WEB ||
			platform == pb.Platform_PLATFORM_MOBILE ||
			platform == pb.Platform_PLATFORM_TABLET)

		// Basic trust score calculation
		trustScore = 0.5 // Base score
		if isBot {
			trustScore = 0.2
		} else if platform == pb.Platform_PLATFORM_API {
			trustScore = 0.7
		} else {
			trustScore = 0.8
		}
	}

	// Detect secure context from referrer or assume HTTPS for modern browsers
	if referrer != "" {
		isSecureContext = strings.HasPrefix(strings.ToLower(referrer), "https://")
	} else {
		isSecureContext = true // Assume secure context by default
	}

	// Extract language from user agent if possible
	var languages []string
	if userAgent != "" && strings.Contains(userAgent, "en-") {
		acceptLanguage = "en-US,en;q=0.9"
		languages = []string{"en-US", "en"}
	} else if userAgent != "" {
		acceptLanguage = "en-US,en;q=0.9" // Default fallback
		languages = []string{"en-US"}
	}

	// Create comprehensive structure with available individual field data and intelligent defaults
	return &ComprehensiveClientInfo{
		// Basic client information (available from individual fields)
		IPAddress: ipAddress,
		Client:    client,
		SessionID: sessionID,
		UserAgent: userAgent,
		Referrer:  referrer,
		Platform:  platform,

		// Enhanced fields (intelligent extraction from available data)
		AcceptLanguage:   acceptLanguage,
		AcceptEncoding:   "gzip, deflate, br", // Common default
		DNT:              "0",                 // Default: tracking allowed
		Timezone:         "UTC",               // Default timezone
		ScreenResolution: "",                  // Cannot determine from server-side
		ColorDepth:       "24",                // Common default
		DeviceMemory:     "",                  // Cannot determine from server-side
		Languages:        languages,

		// Location & Geographic hints (can be enhanced with GeoIP later)
		Country:        "",       // TODO: Add GeoIP lookup for country detection
		TimezoneOffset: "+00:00", // UTC default

		// Marketing & UTM tracking (extract from referrer if possible)
		UTMSource:   blog.extractUTMFromReferrer(referrer, "utm_source"),
		UTMMedium:   blog.extractUTMFromReferrer(referrer, "utm_medium"),
		UTMCampaign: blog.extractUTMFromReferrer(referrer, "utm_campaign"),
		UTMContent:  blog.extractUTMFromReferrer(referrer, "utm_content"),
		UTMTerm:     blog.extractUTMFromReferrer(referrer, "utm_term"),

		// Behavioral indicators (intelligent calculation)
		IsBot:        isBot,
		TrustScore:   trustScore,
		RequestCount: 1, // First request in this context

		// Technical environment (intelligent detection)
		IsSecureContext:   isSecureContext,
		ConnectionType:    "", // Cannot determine from server-side
		BrowserEngine:     browserEngine,
		JavaScriptEnabled: jsEnabled,

		// Timestamps
		FirstSeen:   time.Now().Format(time.RFC3339),
		LastSeen:    time.Now().Format(time.RFC3339),
		CollectedAt: time.Now().Format(time.RFC3339),
	}
}

// Helper method to detect platform from user agent or request platform
func (blog *BlogService) detectPlatform(userAgent string, reqPlatform pb.Platform) activitypb.Platform {
	// If platform is provided in request, convert it
	switch reqPlatform {
	case pb.Platform_PLATFORM_WEB:
		return activitypb.Platform_PLATFORM_WEB
	case pb.Platform_PLATFORM_MOBILE:
		return activitypb.Platform_PLATFORM_MOBILE
	case pb.Platform_PLATFORM_TABLET:
		return activitypb.Platform_PLATFORM_TABLET
	case pb.Platform_PLATFORM_API:
		return activitypb.Platform_PLATFORM_API
	case pb.Platform_PLATFORM_DESKTOP:
		return activitypb.Platform_PLATFORM_DESKTOP
	default:
		// Detect from user agent if platform not specified
		if userAgent != "" {
			userAgent = strings.ToLower(userAgent)
			if strings.Contains(userAgent, "mobile") || strings.Contains(userAgent, "android") || strings.Contains(userAgent, "iphone") {
				return activitypb.Platform_PLATFORM_MOBILE
			}
			if strings.Contains(userAgent, "tablet") || strings.Contains(userAgent, "ipad") {
				return activitypb.Platform_PLATFORM_TABLET
			}
		}
		return activitypb.Platform_PLATFORM_WEB
	}
}

// Helper method to send activity tracking message to RabbitMQ
func (blog *BlogService) sendActivityTrackingMessage(activityReq *activitypb.TrackActivityRequest) {
	go func() {
		// Create activity tracking message
		activityMsg, err := json.Marshal(activityReq)
		if err != nil {
			blog.logger.Errorf("failed to marshal activity tracking message: %v", err)
			return
		}

		fmt.Println("activityMsg1: ", string(activityMsg))

		// Send to activity tracking queue via RabbitMQ
		err = blog.qConn.PublishMessage(blog.config.RabbitMQ.Exchange, "activity.track", activityMsg)
		if err != nil {
			blog.logger.Errorf("failed to publish activity tracking message: %v", err)
			return
		}

		blog.logger.Debugf("activity tracking message sent for user %s, action %s", activityReq.UserId, activityReq.Action)
	}()
}

// Helper method to track blog activities with comprehensive client information
func (blog *BlogService) trackBlogActivity(accountId, action, resource, resourceId string, req interface{}) {
	// Extract comprehensive client information
	clientInfo := blog.extractClientInfo(req)

	fmt.Println("ClientInfo in blog service**>>>> : ", clientInfo)

	toInt32 := func(s string) int32 {
		v, _ := strconv.ParseInt(s, 10, 32)
		return int32(v)
	}

	colorDepth := toInt32(clientInfo.ColorDepth)

	var screenWidth, screenHeight int32
	parts := strings.Split(clientInfo.ScreenResolution, "x")
	if len(parts) == 2 {
		screenWidth = toInt32(parts[0])
		screenHeight = toInt32(parts[1])
	}

	timezoneOffset := toInt32(clientInfo.TimezoneOffset)

	// Create comprehensive ClientInfo for activity tracking
	activityClientInfo := &activitypb.ClientInfo{
		IpAddress:         clientInfo.IPAddress,
		UserAgent:         clientInfo.UserAgent,
		AcceptLanguage:    clientInfo.AcceptLanguage,
		AcceptEncoding:    clientInfo.AcceptEncoding,
		Dnt:               clientInfo.DNT,
		Referer:           clientInfo.Referrer,
		Platform:          blog.detectPlatform(clientInfo.UserAgent, clientInfo.Platform),
		Country:           clientInfo.Country,
		IsBot:             clientInfo.IsBot,
		TrustScore:        clientInfo.TrustScore,
		BrowserEngine:     clientInfo.BrowserEngine,
		UtmSource:         clientInfo.UTMSource,
		UtmMedium:         clientInfo.UTMMedium,
		UtmCampaign:       clientInfo.UTMCampaign,
		UtmTerm:           clientInfo.UTMTerm,
		UtmContent:        clientInfo.UTMContent,
		Timezone:          clientInfo.Timezone,
		Languages:         clientInfo.Languages,
		XClientId:         "", // TODO: Extract if available
		XSessionId:        clientInfo.SessionID,
		ColorDepth:        colorDepth,
		ScreenWidth:       screenWidth,
		ScreenHeight:      screenHeight,
		TimezoneOffset:    timezoneOffset,
		JavascriptEnabled: clientInfo.JavaScriptEnabled,
		RequestCount:      clientInfo.RequestCount,
		// IsSecureContext:   clientInfo.IsSecureContext,
		// Additional fields that can be populated from comprehensive client info
		Connection:      clientInfo.ConnectionType,
		Origin:          clientInfo.Origin,
		XRealIp:         clientInfo.RealIp,
		XForwardedFor:   clientInfo.ForwardedFor,
		XForwardedProto: clientInfo.ForwardedProto,
		XForwardedHost:  clientInfo.ForwardedHost,
		Accept:          clientInfo.Accept,
		Browser:         clientInfo.Browser,
		Os:              clientInfo.Os,
	}

	// Create enhanced activity tracking request with comprehensive client data
	activityReq := &activitypb.TrackActivityRequest{
		UserId:     accountId,
		AccountId:  accountId,
		SessionId:  clientInfo.SessionID,
		Category:   activitypb.ActivityCategory_CATEGORY_CONTENT,
		Action:     action,
		Resource:   resource,
		ResourceId: resourceId,
		ClientInfo: activityClientInfo,
		Success:    true,
		DurationMs: 0, // TODO: Add timing if needed
	}

	// Log comprehensive client tracking information for debugging
	blog.logger.Debugf("Tracking %s activity for user %s - IP: %s, Platform: %s, UserAgent: %s, Country: %s, UTM Source: %s, Trust Score: %f, Browser: %s",
		action, accountId, clientInfo.IPAddress, clientInfo.Platform,
		clientInfo.UserAgent, clientInfo.Country, clientInfo.UTMSource,
		clientInfo.TrustScore, clientInfo.BrowserEngine)

	// Enhanced: Fetch and include detailed metadata for recommendation engine
	var metadata map[string]interface{}

	if resource == "blog" && resourceId != "" {
		// Fetch comprehensive blog metadata
		metadata = blog.fetchBlogMetadataForActivity(context.Background(), resourceId)
	} else if resource == "search" && resourceId != "" {
		// Create search context metadata
		metadata = map[string]interface{}{
			"search_query":       resourceId,
			"search_terms":       strings.Fields(resourceId),
			"search_type":        "blog_search",
			"metadata_source":    "search_context",
			"metadata_timestamp": time.Now().UTC().Format(time.RFC3339),
		}

		// Add search-specific context from request
		if searchReq, ok := req.(*pb.SearchReq); ok {
			if len(searchReq.Tags) > 0 {
				metadata["search_tags"] = searchReq.Tags
			}
			metadata["search_limit"] = searchReq.Limit
			metadata["search_offset"] = searchReq.Offset
		}
	}

	// Convert metadata to protobuf Struct if available
	if metadata != nil {
		if metadataStruct, err := structpb.NewStruct(metadata); err == nil {
			activityReq.Metadata = metadataStruct
		} else {
			blog.logger.Warnf("failed to convert metadata to struct: %v", err)
		}
	}

	fmt.Println("activityReq: ", activityReq)

	// Send activity tracking message
	blog.sendActivityTrackingMessage(activityReq)
}

// Helper method to fetch comprehensive blog metadata for activity tracking
func (blog *BlogService) fetchBlogMetadataForActivity(ctx context.Context, blogId string) map[string]interface{} {
	defer func() {
		if r := recover(); r != nil {
			blog.logger.Errorf("recovered from panic in fetchBlogMetadataForActivity: %v", r)
		}
	}()

	// Fetch blog data from Elasticsearch
	blogData, err := blog.osClient.GetBlogByBlogId(ctx, blogId, false) // Published blogs
	if err != nil {
		// Try draft blogs if published blog not found
		blogData, err = blog.osClient.GetBlogByBlogId(ctx, blogId, true)
		if err != nil {
			blog.logger.Warnf("could not fetch blog metadata for activity tracking, blogId: %s, error: %v", blogId, err)
			return nil
		}
	}

	// Extract key metadata for recommendation engine
	metadata := make(map[string]interface{})

	// Blog identification and ownership
	if blogId, ok := blogData["blog_id"].(string); ok {
		metadata["blog_id"] = blogId
	}
	if ownerAccountId, ok := blogData["owner_account_id"].(string); ok {
		metadata["blog_author_id"] = ownerAccountId
	}

	// Blog content metadata
	if title, ok := blogData["title"].(string); ok {
		metadata["blog_title"] = title
	}
	if category, ok := blogData["category"].(string); ok {
		metadata["blog_category"] = category
	}

	// Blog tags for content-based recommendations
	if tags, ok := blogData["tags"].([]interface{}); ok {
		stringTags := make([]string, len(tags))
		for i, tag := range tags {
			if tagStr, ok := tag.(string); ok {
				stringTags[i] = tagStr
			}
		}
		metadata["blog_tags"] = stringTags
	} else if tags, ok := blogData["tags"].([]string); ok {
		metadata["blog_tags"] = tags
	}

	// Temporal metadata
	if publishedTime, ok := blogData["published_time"].(string); ok {
		metadata["blog_published_time"] = publishedTime
	}
	if createdTime, ok := blogData["created_time"].(string); ok {
		metadata["blog_created_time"] = createdTime
	}

	// Content type and structure
	if contentType, ok := blogData["content_type"].(string); ok {
		metadata["blog_content_type"] = contentType
	}

	// Blog status and metrics
	if isDraft, ok := blogData["is_draft"].(bool); ok {
		metadata["blog_is_draft"] = isDraft
	}
	if isArchive, ok := blogData["is_archive"].(bool); ok {
		metadata["blog_is_archive"] = isArchive
	}

	// Author information for collaborative filtering
	if authorList, ok := blogData["author_list"].([]interface{}); ok {
		stringAuthors := make([]string, len(authorList))
		for i, author := range authorList {
			if authorStr, ok := author.(string); ok {
				stringAuthors[i] = authorStr
			}
		}
		metadata["blog_authors"] = stringAuthors
	} else if authorList, ok := blogData["author_list"].([]string); ok {
		metadata["blog_authors"] = authorList
	}

	// Add source and confidence for data quality
	metadata["metadata_source"] = "elasticsearch"
	metadata["metadata_timestamp"] = time.Now().UTC().Format(time.RFC3339)

	blog.logger.Debugf("fetched blog metadata for activity tracking: blogId=%s, metadata=%v", blogId, metadata)
	return metadata
}

func (blog *BlogService) DraftBlog(ctx context.Context, req *pb.DraftBlogRequest) (*pb.BlogResponse, error) {
	blog.logger.Debugw("draft blog", "blog_id", req.BlogId, "owner", req.OwnerAccountId)
	req.IsDraft = true

	exists, _, _ := blog.osClient.DoesBlogExist(ctx, req.BlogId)
	if exists {
		blog.logger.Infof("updating the blog with id: %s", req.BlogId)
		// owner, _, err := blog.osClient.GetBlogDetailsById(ctx, req.BlogId)
		// if err != nil {
		// 	blog.logger.Errorf("cannot find the blog with id: %s, error: %v", req.BlogId, err)
		// 	return nil, status.Errorf(codes.NotFound, "cannot find the blog with id")
		// }

		// if req.OwnerAccountId != owner {
		// 	blog.logger.Errorf("user %s is trying to take the ownership of the content, original owner is: %s", req.OwnerAccountId, owner)
		// 	return nil, status.Errorf(codes.Unauthenticated, "you don't have permission to change the owner id")
		// }
	} else {
		blog.logger.Infof("creating the blog with id: %s for author: %s", req.BlogId, req.OwnerAccountId)
		bx, err := json.Marshal(models.InterServiceMessage{
			AccountId:  req.OwnerAccountId,
			BlogId:     req.BlogId,
			Action:     constants.BLOG_CREATE,
			BlogStatus: constants.BlogStatusDraft,
			IpAddress:  req.Ip,
			Client:     req.Client,
		})

		if err != nil {
			blog.logger.Errorf("cannot marshal the message for blog: %s, error: %v", req.BlogId, err)
			return nil, status.Errorf(codes.Internal, "Something went wrong while drafting a blog")
		}

		if len(req.Tags) == 0 {
			req.Tags = []string{"untagged"}
		}
		// fmt.Printf("bx: %v\n", string(bx))
		go func() {
			err := blog.qConn.PublishMessage(blog.config.RabbitMQ.Exchange, blog.config.RabbitMQ.RoutingKeys[1], bx)
			if err != nil {
				blog.logger.Errorf("failed to publish blog create message to RabbitMQ: exchange=%s, routing_key=%s, error=%v", blog.config.RabbitMQ.Exchange, blog.config.RabbitMQ.RoutingKeys[1], err)
			}
		}()
	}

	_, err := blog.osClient.DraftABlog(ctx, req)
	if err != nil {
		blog.logger.Errorf("cannot store draft into opensearch: %v", err)
		return nil, err
	}

	// Track blog activity
	action := "create_draft"
	if exists {
		action = "update_draft"
	}
	blog.trackBlogActivity(req.OwnerAccountId, action, "blog", req.BlogId, req)

	return &pb.BlogResponse{
		Blog: req.Blog,
	}, nil
}

func (blog *BlogService) CheckIfBlogsExist(ctx context.Context, req *pb.BlogByIdReq) (*pb.BlogExistsRes, error) {
	exists, blogInfo, err := blog.osClient.DoesBlogExist(ctx, req.BlogId)
	if err != nil {
		blog.logger.Errorf("cannot find the blog with id: %s, error: %v", req.BlogId, err)
		return nil, status.Errorf(codes.NotFound, "cannot find the blog with id")
	}

	isDraft, ok := blogInfo["is_draft"].(bool)
	if !ok {
		blog.logger.Errorf("unexpected type for is_draft field")
		isDraft = true
	}

	// Track blog activity
	// blog.trackBlogActivity(req.OwnerAccountId, "check_blog_existence", "blog", req.BlogId, req)

	return &pb.BlogExistsRes{
		BlogExists: exists,
		IsDraft:    isDraft,
	}, nil
}

func (blog *BlogService) GetDraftBlogsByAccId(ctx context.Context, req *pb.BlogByIdReq) (*pb.GetDraftBlogsRes, error) {
	blog.logger.Debugw("get draft blogs", "owner", req.OwnerAccountId)
	if req.OwnerAccountId == "" {
		blog.logger.Error("account id cannot be empty")
		return nil, status.Errorf(codes.InvalidArgument, "Account id cannot be empty")
	}

	res, err := blog.osClient.GetDraftBlogsByOwnerAccountID(ctx, req.OwnerAccountId)
	if err != nil {
		blog.logger.Errorf("error occurred while getting draft blogs for account id: %s, error: %v", req.OwnerAccountId, err)
		return nil, status.Errorf(codes.Internal, "cannot get the draft blogs for account id: %s", req.OwnerAccountId)
	}

	// Track blog activity
	blog.trackBlogActivity(req.OwnerAccountId, "get_draft_blogs", "user", req.OwnerAccountId, req)

	return res, nil
}

func (blog *BlogService) GetPublishedBlogsByAccID(ctx context.Context, req *pb.BlogByIdReq) (*pb.GetPublishedBlogsRes, error) {
	blog.logger.Debugw("get published blogs", "owner", req.OwnerAccountId)
	if req.OwnerAccountId == "" {
		blog.logger.Error("account id cannot be empty")
		return nil, status.Errorf(codes.InvalidArgument, "Account id cannot be empty")
	}

	res, err := blog.osClient.GetPublishedBlogsByOwnerAccountID(ctx, req.OwnerAccountId)
	if err != nil {
		blog.logger.Errorf("error occurred while getting published blogs for account id: %s, error: %v", req.OwnerAccountId, err)
		return nil, status.Errorf(codes.Internal, "cannot get the published blogs for account id: %s", req.OwnerAccountId)
	}

	// Track blog activity
	blog.trackBlogActivity(req.OwnerAccountId, "get_published_blogs", "user", req.OwnerAccountId, req)

	return res, nil
}

func (blog *BlogService) GetDraftBlogById(ctx context.Context, req *pb.BlogByIdReq) (*pb.BlogByIdRes, error) {
	blog.logger.Debugw("get draft blog", "blog_id", req.BlogId)

	res, err := blog.osClient.GetDraftedBlogByIdAndOwner(ctx, req.BlogId, req.OwnerAccountId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "couldn't found the blog with blogId: %s and ownerAccountId: %s", req.BlogId, req.OwnerAccountId)
	}

	// Check if the response is nil, which indicates no blog was found
	if res == nil {
		return nil, status.Errorf(codes.NotFound, "no blog found with blogId: %s and ownerAccountId: %s", req.BlogId, req.OwnerAccountId)
	}

	// Track blog activity
	blog.trackBlogActivity(req.OwnerAccountId, "get_draft_blog", "blog", req.BlogId, req)

	return res, nil
}

func (blog *BlogService) GetPublishedBlogByIdAndOwnerId(ctx context.Context, req *pb.BlogByIdReq) (*pb.BlogByIdRes, error) {
	blog.logger.Debugw("get published blog", "blog_id", req.BlogId)

	// Fetch the published blog by blog_id and owner_account_id
	res, err := blog.osClient.GetPublishedBlogByIdAndOwner(ctx, req.BlogId, req.OwnerAccountId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "couldn't fetch the blog with blogId: %s and ownerAccountId: %s", req.BlogId, req.OwnerAccountId)
	}

	// Check if the response is nil, which indicates no blog was found
	if res == nil {
		return nil, status.Errorf(codes.NotFound, "no blog found with blogId: %s and ownerAccountId: %s", req.BlogId, req.OwnerAccountId)
	}

	// Track blog activity
	blog.trackBlogActivity(req.OwnerAccountId, "get_published_blog", "blog", req.BlogId, req)

	// Return the found blog
	return res, nil
}

func (blog *BlogService) PublishBlog(ctx context.Context, req *pb.PublishBlogReq) (*pb.PublishBlogResp, error) {
	blog.logger.Infof("The user has requested to publish the blog: %s", req.BlogId)

	fmt.Println("clientinfo in publishblog", req.ClientInfo)

	// Check if the blog exists
	exists, _, err := blog.osClient.DoesBlogExist(ctx, req.BlogId)
	if err != nil {
		blog.logger.Errorf("Error checking blog existence: %v", err)
		return nil, status.Errorf(codes.Internal, "cannot get the blog for id: %s", req.BlogId)
	}

	if !exists {
		blog.logger.Errorf("The blog with ID: %s doesn't exist", req.BlogId)
		return nil, status.Errorf(codes.NotFound, "cannot find the blog for id: %s", req.BlogId)
	}

	_, err = blog.osClient.PublishBlogById(ctx, req.BlogId)
	if err != nil {
		blog.logger.Errorf("Error Publishing the blog: %s, error: %v", req.BlogId, err)
		return nil, status.Errorf(codes.Internal, "cannot find the blog for id: %s", req.BlogId)
	}

	// TODO: Add Tags to the db if not already added

	bx, err := json.Marshal(models.InterServiceMessage{
		AccountId:  req.AccountId,
		BlogId:     req.BlogId,
		Action:     constants.BLOG_PUBLISH,
		BlogStatus: constants.BlogStatusPublished,
		IpAddress:  req.Ip,
		Client:     req.Client,
		Tags:       req.Tags,
	})

	if err != nil {
		blog.logger.Errorf("failed to marshal message for blog publish: user_id=%s, blog_id=%s, error=%v", req.AccountId, req.BlogId, err)
		return nil, status.Errorf(codes.Internal, "published the blog with some error: %s", req.BlogId)
	}

	go func() {
		// Enqueue publish message to user service asynchronously
		err := blog.qConn.PublishMessage(blog.config.RabbitMQ.Exchange, blog.config.RabbitMQ.RoutingKeys[1], bx)
		if err != nil {
			blog.logger.Errorf(`failed to publish blog publish message to RabbitMQ: 
			 exchange=%s, routing_key=%s, error=%v`, blog.config.RabbitMQ.Exchange,
				blog.config.RabbitMQ.RoutingKeys[1], err)
		}

	}()

	go func() {
		// Get the blog slug and do the google search engine optimization
		slug := req.Slug
		if slug == "" {
			blog.logger.Warnf("slug is empty for blog id: %s, generating a new slug", req.BlogId)
			slug = fmt.Sprintf("blog-%s", req.BlogId)
		}

		// A slug looks like: proxmox-virtual-environment-the-practical-guide-for-smart-virtualization-78li3
		// Add https://monkeys.com.co host and append /blog/ with host and then followed by slug
		// The complete slug should look like: https://monkeys.com.co/blog/proxmox-virtual-environment-the-practical-guide-for-smart-virtualization-78li3

		// Call a function to handle SEO asynchronously
		err := blog.seoManager.HandleSEOForBlog(ctx, req.BlogId, slug)
		if err != nil {
			blog.logger.Errorf("failed to handle SEO for blog: user_id=%s, blog_id=%s, error=%v", req.AccountId, req.BlogId, err)
		}

	}()

	// Track blog activity
	blog.trackBlogActivity(req.AccountId, "publish_blog", "blog", req.BlogId, req)

	return &pb.PublishBlogResp{
		Message: fmt.Sprintf("the blog %s has been published!", req.BlogId),
	}, nil
}

func (blog *BlogService) MoveBlogToDraftStatus(ctx context.Context, req *pb.BlogReq) (*pb.BlogResp, error) {
	blog.logger.Infof("The user has requested to publish the blog: %s", req.BlogId)

	// TODO: Check if blog exists and published
	exists, _, err := blog.osClient.DoesBlogExist(ctx, req.BlogId)
	if err != nil {
		blog.logger.Errorf("Error checking blog existence: %v", err)
		return nil, status.Errorf(codes.Internal, "cannot get the blog for id: %s", req.BlogId)
	}

	if !exists {
		blog.logger.Errorf("The blog with ID: %s doesn't exist", req.BlogId)
		return nil, status.Errorf(codes.NotFound, "cannot find the blog for id: %s", req.BlogId)
	}

	_, err = blog.osClient.MoveBlogToDraft(ctx, req.BlogId)
	if err != nil {
		blog.logger.Errorf("Error Publishing the blog: %s, error: %v", req.BlogId, err)
		return nil, status.Errorf(codes.Internal, "cannot find the blog for id: %s", req.BlogId)
	}

	bx, err := json.Marshal(models.InterServiceMessage{
		AccountId:  req.AccountId,
		BlogId:     req.BlogId,
		Action:     constants.BLOG_UPDATE,
		BlogStatus: constants.BlogStatusDraft,
		IpAddress:  req.Ip,
		Client:     req.Client,
	})

	if err != nil {
		blog.logger.Errorf("failed to marshal message for blog publish: user_id=%s, blog_id=%s, error=%v", req.AccountId, req.BlogId, err)
		return nil, status.Errorf(codes.Internal, "published the blog with some error: %s", req.BlogId)
	}

	// Enqueue publish message to user service asynchronously
	go func() {
		err := blog.qConn.PublishMessage(blog.config.RabbitMQ.Exchange, blog.config.RabbitMQ.RoutingKeys[1], bx)
		if err != nil {
			blog.logger.Errorf("failed to publish blog publish message to RabbitMQ: exchange=%s, routing_key=%s, error=%v", blog.config.RabbitMQ.Exchange, blog.config.RabbitMQ.RoutingKeys[1], err)
		}
	}()

	// Track blog activity
	blog.trackBlogActivity(req.AccountId, "move_to_draft", "blog", req.BlogId, req)

	return &pb.BlogResp{
		Message: fmt.Sprintf("the blog %s has been moved to draft.", req.BlogId),
	}, nil
}

// TODO: Fetch a finite no of blogs like 100 latest blogs based on the tag names
func (blog *BlogService) GetPublishedBlogsByTagsName(ctx context.Context, req *pb.GetBlogsByTagsNameReq) (*pb.GetBlogsByTagsNameRes, error) {
	blog.logger.Infof("fetching blogs with the tags: %s", req.TagNames)

	for i := 0; i < len(req.TagNames); i++ {
		req.TagNames[i] = strings.TrimSpace(req.TagNames[i])
	}

	res, err := blog.osClient.GetPublishedBlogByTagsName(ctx, req.TagNames...)
	if err != nil {
		return nil, err
	}

	// Track blog activity (call once after the loop, not inside it)
	blog.trackBlogActivity("", "get_blogs_by_tags", "search", strings.Join(req.TagNames, ","), req)

	return res, nil
}

func (blog *BlogService) GetPublishedBlogById(ctx context.Context, req *pb.BlogByIdReq) (*pb.BlogByIdRes, error) {
	blog.logger.Infof("fetching blog with id: %s", req.BlogId)

	res, err := blog.osClient.GetPublishedBlogById(ctx, req.BlogId)
	if err != nil {
		return nil, err
	}

	// Track blog activity
	blog.trackBlogActivity(req.OwnerAccountId, "get_published_blog_by_id", "blog", req.BlogId, req)

	return res, nil
}

func (blog *BlogService) ArchiveBlogById(ctx context.Context, req *pb.ArchiveBlogReq) (*pb.ArchiveBlogResp, error) {
	blog.logger.Infof("Archiving blog %s", req.BlogId)

	exists, _, err := blog.osClient.DoesBlogExist(ctx, req.BlogId)
	if err != nil {
		blog.logger.Errorf("Error checking blog existence: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to check existence for blog with ID: %s", req.BlogId)
	}

	if !exists {
		blog.logger.Errorf("Blog with ID %s does not exist", req.BlogId)
		return nil, status.Errorf(codes.NotFound, "blog with ID %s does not exist", req.BlogId)
	}

	updateResp, err := blog.osClient.AchieveAPublishedBlogById(ctx, req.BlogId)
	if err != nil {
		blog.logger.Errorf("failed to archive the blog with ID: %s, error: %v", req.BlogId, err)
		return nil, status.Errorf(codes.Internal, "failed to archive blog with ID: %s", req.BlogId)
	}

	blog.logger.Infof("Blog with ID: %s archived successfully, status code: %v", req.BlogId, updateResp.StatusCode)

	// Track blog activity
	blog.trackBlogActivity("", "archive_blog", "blog", req.BlogId, req)

	return &pb.ArchiveBlogResp{
		Message: fmt.Sprintf("Blog %s has been archived!", req.BlogId),
	}, nil
}

func (blog *BlogService) GetLatest100Blogs(ctx context.Context, req *pb.GetBlogsByTagsNameReq) (*pb.GetBlogsByTagsNameRes, error) {
	res, err := blog.osClient.GetLast100BlogsLatestFirst(ctx)
	if err != nil {
		return nil, err
	}

	// Track blog activity
	blog.trackBlogActivity("", "get_latest_blogs", "search", "latest_100", req)

	return res, nil
}

// TODO: Incase of blog doesn't exists, do return 404
func (blog *BlogService) DeleteABlogByBlogId(ctx context.Context, req *pb.DeleteBlogReq) (*pb.DeleteBlogResp, error) {
	_, err := blog.osClient.DeleteABlogById(ctx, req.BlogId)
	if err != nil {
		blog.logger.Errorf("failed to delete the blog with ID: %s, error: %v", req.BlogId, err)
		return nil, status.Errorf(codes.Internal, "failed to delete the blog with ID: %s", req.BlogId)
	}

	bx, err := json.Marshal(models.InterServiceMessage{
		AccountId:  req.OwnerAccountId,
		BlogId:     req.BlogId,
		Action:     constants.BLOG_DELETE,
		BlogStatus: constants.BlogDeleted,
		IpAddress:  req.Ip,
		Client:     req.Client,
	})

	if err != nil {
		blog.logger.Errorf("failed to marshal message for blog publish: user_id=%s, blog_id=%s, error=%v", req.OwnerAccountId, req.BlogId, err)
		return nil, status.Errorf(codes.Internal, "published the blog with some error: %s", req.BlogId)
	}

	// Enqueue delete message to user service asynchronously
	go func() {
		err := blog.qConn.PublishMessage(blog.config.RabbitMQ.Exchange, blog.config.RabbitMQ.RoutingKeys[1], bx)
		if err != nil {
			blog.logger.Errorf("failed to publish blog publish message to RabbitMQ: exchange=%s, routing_key=%s, error=%v", blog.config.RabbitMQ.Exchange, blog.config.RabbitMQ.RoutingKeys[1], err)
		}
	}()

	// Enqueue delete message to storage service asynchronously
	go func() {
		err := blog.qConn.PublishMessage(blog.config.RabbitMQ.Exchange, blog.config.RabbitMQ.RoutingKeys[2], bx)
		if err != nil {
			blog.logger.Errorf("failed to publish blog publish message to RabbitMQ: exchange=%s, routing_key=%s, error=%v", blog.config.RabbitMQ.Exchange, blog.config.RabbitMQ.RoutingKeys[2], err)
		}
	}()

	// Track blog activity
	blog.trackBlogActivity(req.OwnerAccountId, "delete_blog", "blog", req.BlogId, req)

	// fmt.Printf("resp.StatusCode: %v\n", resp.StatusCode)
	return &pb.DeleteBlogResp{
		Message: fmt.Sprintf("Blog with id %s has been successfully deleted", req.BlogId),
	}, nil
}

func (blog *BlogService) GetDraftBlogByBlogId(ctx context.Context, req *pb.BlogByIdReq) (*pb.BlogByIdRes, error) {
	blog.logger.Infof("fetching blog with id: %s", req.BlogId)

	res, err := blog.osClient.GetDraftBlogByBlogId(ctx, req.BlogId)
	if err != nil {
		return nil, err
	}

	// Track blog activity
	blog.trackBlogActivity(req.OwnerAccountId, "get_draft_blog_by_id", "blog", req.BlogId, req)

	return res, nil
}

func (blog *BlogService) GetAllBlogsByBlogIds(ctc context.Context, req *pb.GetBlogsByBlogIds) (*pb.GetBlogsRes, error) {
	if len(req.BlogIds) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "blog ids cannot be empty")
	}

	res, err := blog.osClient.GetBlogsByBlogIds(ctc, req.BlogIds)
	if err != nil {
		return nil, err
	}

	// Track blog activity (call once for the batch, not in any loop)
	blog.trackBlogActivity("", "get_blogs_by_ids", "search", strings.Join(req.BlogIds, ","), req)

	return res, nil
}
