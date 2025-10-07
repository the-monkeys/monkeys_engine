package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// Core Activity Event Model - Main tracking table for all user activities
type ActivityEvent struct {
	ID         string           `json:"id" db:"id"`
	UserID     string           `json:"user_id" db:"user_id" validate:"required"`
	AccountID  string           `json:"account_id" db:"account_id" validate:"required"`
	SessionID  string           `json:"session_id" db:"session_id"`
	Category   ActivityCategory `json:"category" db:"category" validate:"required"`
	Action     string           `json:"action" db:"action" validate:"required"`
	Resource   string           `json:"resource,omitempty" db:"resource"`       // blog, user, comment, etc.
	ResourceID string           `json:"resource_id,omitempty" db:"resource_id"` // specific ID of the resource
	ClientIP   string           `json:"client_ip" db:"client_ip"`               // User's IP address
	UserAgent  string           `json:"user_agent" db:"user_agent"`             // Browser/device info
	Country    string           `json:"country,omitempty" db:"country"`         // Derived from IP
	Platform   string           `json:"platform" db:"platform"`                 // web, mobile, api
	Referrer   string           `json:"referrer,omitempty" db:"referrer"`       // Where user came from
	Metadata   JSONMap          `json:"metadata,omitempty" db:"metadata"`       // Additional flexible data
	Success    bool             `json:"success" db:"success"`                   // Did action succeed
	Duration   int64            `json:"duration_ms,omitempty" db:"duration_ms"` // Action duration
	Timestamp  time.Time        `json:"timestamp" db:"timestamp"`
	CreatedAt  time.Time        `json:"created_at" db:"created_at"`
}

// Activity Categories for blogging platform (matches comprehensive specification)
type ActivityCategory string

const (
	// Core User Actions
	CategoryAuthentication ActivityCategory = "authentication" // login, logout, session, MFA events
	CategoryAuthorization  ActivityCategory = "authorization"  // permissions, roles, access control
	CategoryContent        ActivityCategory = "content"        // create, edit, delete, publish, collaboration
	CategorySocial         ActivityCategory = "social"         // like, follow, comment, share, reporting
	CategorySearch         ActivityCategory = "search"         // search queries, filters, refinements
	CategoryNavigation     ActivityCategory = "navigation"     // page visits, menu clicks, user journeys

	// AI & Intelligence
	CategoryRecommendation ActivityCategory = "recommendation" // AI recommendations, personalization
	CategoryAnalytics      ActivityCategory = "analytics"      // behavior analysis, engagement metrics

	// System & Operations
	CategorySecurity     ActivityCategory = "security"     // threats, violations, anomalies
	CategorySystem       ActivityCategory = "system"       // admin actions, maintenance, performance
	CategoryNotification ActivityCategory = "notification" // emails, push, in-app notifications
	CategoryCompliance   ActivityCategory = "compliance"   // GDPR, data requests, privacy
	CategoryFinancial    ActivityCategory = "financial"    // payments, subscriptions, billing
	CategoryIntegration  ActivityCategory = "integration"  // APIs, webhooks, third-party services
	CategoryIncident     ActivityCategory = "incident"     // security incidents, data breaches
)

// Severity levels for security events
type Severity string

const (
	SeverityLow      Severity = "low"      // Minor security issues
	SeverityMedium   Severity = "medium"   // Important security events
	SeverityHigh     Severity = "high"     // Critical security events
	SeverityCritical Severity = "critical" // Emergency security situations
)

// User Journey Stages
type JourneyStage string

const (
	StageOnboarding   JourneyStage = "onboarding"   // New user getting started
	StageActivation   JourneyStage = "activation"   // User becoming active
	StageEngagement   JourneyStage = "engagement"   // Regular usage patterns
	StageRetention    JourneyStage = "retention"    // Long-term user retention
	StageChurn        JourneyStage = "churn"        // User becoming inactive
	StageReactivation JourneyStage = "reactivation" // Bringing back churned users
)

// Content Types
type ContentType string

const (
	ContentBlog     ContentType = "blog"
	ContentComment  ContentType = "comment"
	ContentProfile  ContentType = "profile"
	ContentImage    ContentType = "image"
	ContentVideo    ContentType = "video"
	ContentDocument ContentType = "document"
	ContentSeries   ContentType = "series"
	ContentDraft    ContentType = "draft"
)

// Platform Types
type Platform string

const (
	PlatformWeb     Platform = "web"
	PlatformMobile  Platform = "mobile"
	PlatformTablet  Platform = "tablet"
	PlatformAPI     Platform = "api"
	PlatformDesktop Platform = "desktop"
)

// Device Types
type DeviceType string

const (
	DeviceDesktop DeviceType = "desktop"
	DeviceMobile  DeviceType = "mobile"
	DeviceTablet  DeviceType = "tablet"
	DeviceBot     DeviceType = "bot"
	DeviceUnknown DeviceType = "unknown"
)

// Notification Channels
type NotificationChannel string

const (
	ChannelEmail   NotificationChannel = "email"
	ChannelPush    NotificationChannel = "push"
	ChannelInApp   NotificationChannel = "in_app"
	ChannelSMS     NotificationChannel = "sms"
	ChannelWebhook NotificationChannel = "webhook"
)

// Notification Types
type NotificationType string

const (
	NotifyMarketing     NotificationType = "marketing"
	NotifyTransactional NotificationType = "transactional"
	NotifySystem        NotificationType = "system"
	NotifyAlert         NotificationType = "alert"
	NotifySecurity      NotificationType = "security"
)

// Common Actions by Category for blogging platform
type ActionType string

// Authentication Actions
const (
	ActionLogin          ActionType = "login"
	ActionLogout         ActionType = "logout"
	ActionRegister       ActionType = "register"
	ActionSessionExpire  ActionType = "session_expire"
	ActionPasswordReset  ActionType = "password_reset"
	ActionPasswordChange ActionType = "password_change"
	ActionEmailVerify    ActionType = "email_verify"
	ActionMFASetup       ActionType = "mfa_setup"
	ActionMFAVerify      ActionType = "mfa_verify"
	ActionOAuthLogin     ActionType = "oauth_login"
	ActionAccountLockout ActionType = "account_lockout"
	ActionSessionHijack  ActionType = "session_hijack"
	ActionTokenRefresh   ActionType = "token_refresh"
)

// Navigation Actions
const (
	ActionPageView    ActionType = "page_view"
	ActionMenuClick   ActionType = "menu_click"
	ActionLinkClick   ActionType = "link_click"
	ActionButtonClick ActionType = "button_click"
	ActionFormSubmit  ActionType = "form_submit"
)

// Content Actions
const (
	ActionCreate         ActionType = "create"
	ActionEdit           ActionType = "edit"
	ActionDelete         ActionType = "delete"
	ActionPublish        ActionType = "publish"
	ActionUnpublish      ActionType = "unpublish"
	ActionDraft          ActionType = "draft"
	ActionView           ActionType = "view"
	ActionDownload       ActionType = "download"
	ActionUpload         ActionType = "upload"
	ActionArchive        ActionType = "archive"
	ActionTag            ActionType = "tag"
	ActionCategorize     ActionType = "categorize"
	ActionCollaborate    ActionType = "collaborate"
	ActionRevision       ActionType = "revision"
	ActionSchedule       ActionType = "schedule"
	ActionPromote        ActionType = "promote"
	ActionCrossReference ActionType = "cross_reference"
	ActionSeries         ActionType = "series"
)

// Social Actions
const (
	ActionLike     ActionType = "like"
	ActionUnlike   ActionType = "unlike"
	ActionFollow   ActionType = "follow"
	ActionUnfollow ActionType = "unfollow"
	ActionComment  ActionType = "comment"
	ActionShare    ActionType = "share"
	ActionBookmark ActionType = "bookmark"
	ActionReport   ActionType = "report"
)

// Search Actions
const (
	ActionSearch           ActionType = "search"
	ActionFilter           ActionType = "filter"
	ActionSort             ActionType = "sort"
	ActionPaginate         ActionType = "paginate"
	ActionSearchRefinement ActionType = "search_refinement"
	ActionNoResults        ActionType = "no_results"
	ActionSearchSuggestion ActionType = "search_suggestion"
	ActionAdvancedSearch   ActionType = "advanced_search"
)

// Analytics & Behavioral Actions
const (
	ActionPageDwell       ActionType = "page_dwell"
	ActionScrollDepth     ActionType = "scroll_depth"
	ActionReadingSpeed    ActionType = "reading_speed"
	ActionContentComplete ActionType = "content_complete"
	ActionBounce          ActionType = "bounce"
	ActionReturnVisit     ActionType = "return_visit"
	ActionSessionStart    ActionType = "session_start"
	ActionSessionEnd      ActionType = "session_end"
	ActionEngagement      ActionType = "engagement"
	ActionConversion      ActionType = "conversion"
	ActionDropOff         ActionType = "drop_off"
)

// Recommendation Actions
const (
	ActionRecommendView    ActionType = "recommend_view"
	ActionRecommendClick   ActionType = "recommend_click"
	ActionRecommendDismiss ActionType = "recommend_dismiss"
	ActionFeedback         ActionType = "feedback"
	ActionRecommendRequest ActionType = "recommend_request"
	ActionPersonalize      ActionType = "personalize"
	ActionColdStart        ActionType = "cold_start"
	ActionModelRetrain     ActionType = "model_retrain"
	ActionABTest           ActionType = "ab_test"
	ActionReengagement     ActionType = "reengage"
	ActionTrending         ActionType = "trending"
	ActionExploration      ActionType = "exploration"
	ActionSerendipity      ActionType = "serendipity"
	ActionDiversify        ActionType = "diversify"
)

// Authorization Actions
const (
	ActionGrantPermission   ActionType = "grant_permission"
	ActionRevokePermission  ActionType = "revoke_permission"
	ActionAccessDenied      ActionType = "access_denied"
	ActionRoleAssign        ActionType = "role_assign"
	ActionRoleRevoke        ActionType = "role_revoke"
	ActionInviteSent        ActionType = "invite_sent"
	ActionInviteAccepted    ActionType = "invite_accepted"
	ActionInviteDeclined    ActionType = "invite_declined"
	ActionTokenGenerate     ActionType = "token_generate"
	ActionTokenRevoke       ActionType = "token_revoke"
	ActionPrivilegeEscalate ActionType = "privilege_escalate"
)

// System & Admin Actions
const (
	ActionMaintenance   ActionType = "maintenance"
	ActionMigration     ActionType = "migration"
	ActionBackup        ActionType = "backup"
	ActionRestore       ActionType = "restore"
	ActionConfigChange  ActionType = "config_change"
	ActionBan           ActionType = "ban"
	ActionUnban         ActionType = "unban"
	ActionModerate      ActionType = "moderate"
	ActionFlag          ActionType = "flag"
	ActionDataRetention ActionType = "data_retention"
)

// Communication Actions
const (
	ActionEmailSent       ActionType = "email_sent"
	ActionEmailOpened     ActionType = "email_opened"
	ActionEmailClicked    ActionType = "email_clicked"
	ActionEmailBounced    ActionType = "email_bounced"
	ActionNotifyDelivered ActionType = "notify_delivered"
	ActionNotifyRead      ActionType = "notify_read"
	ActionNotifyDismissed ActionType = "notify_dismissed"
	ActionSubscribe       ActionType = "subscribe"
	ActionUnsubscribe     ActionType = "unsubscribe"
	ActionPushSent        ActionType = "push_sent"
)

// Financial Actions
const (
	ActionPaymentAttempt        ActionType = "payment_attempt"
	ActionPaymentSuccess        ActionType = "payment_success"
	ActionPaymentFailed         ActionType = "payment_failed"
	ActionSubscriptionUpgrade   ActionType = "subscription_upgrade"
	ActionSubscriptionDowngrade ActionType = "subscription_downgrade"
	ActionSubscriptionCancel    ActionType = "subscription_cancel"
	ActionRefundRequest         ActionType = "refund_request"
	ActionRefundProcessed       ActionType = "refund_processed"
	ActionTrialStart            ActionType = "trial_start"
	ActionTrialExpire           ActionType = "trial_expire"
	ActionBillingUpdate         ActionType = "billing_update"
)

// Integration Actions
const (
	ActionAPICall        ActionType = "api_call"
	ActionWebhookSent    ActionType = "webhook_sent"
	ActionWebhookFailed  ActionType = "webhook_failed"
	ActionOAuthAuthorize ActionType = "oauth_authorize"
	ActionRateLimit      ActionType = "rate_limit"
	ActionAPIKeyRotate   ActionType = "api_key_rotate"
	ActionThirdPartySync ActionType = "third_party_sync"
)

// Incident Actions
const (
	ActionSecurityIncident  ActionType = "security_incident"
	ActionDataBreach        ActionType = "data_breach"
	ActionServiceDisruption ActionType = "service_disruption"
	ActionEmergencyLock     ActionType = "emergency_lock"
	ActionForensicCollect   ActionType = "forensic_collect"
	ActionIncidentResolve   ActionType = "incident_resolve"
)

// Security Event Model
type SecurityEvent struct {
	ID          string          `json:"id" db:"id"`
	UserID      string          `json:"user_id" db:"user_id"`
	AccountID   string          `json:"account_id" db:"account_id"`
	EventType   string          `json:"event_type" db:"event_type"` // failed_login, suspicious_activity, etc.
	Severity    Severity        `json:"severity" db:"severity"`
	Description string          `json:"description" db:"description"`
	Context     SecurityContext `json:"context" db:"context"`
	RiskScore   int             `json:"risk_score" db:"risk_score"` // 0-100 risk assessment
	Resolved    bool            `json:"resolved" db:"resolved"`
	ResolvedBy  *string         `json:"resolved_by,omitempty" db:"resolved_by"`
	ResolvedAt  *time.Time      `json:"resolved_at,omitempty" db:"resolved_at"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at" db:"updated_at"`
}

// Security Context for security events
type SecurityContext struct {
	IPAddress      string            `json:"ip_address"`
	UserAgent      string            `json:"user_agent"`
	Country        string            `json:"country,omitempty"`
	FailedAttempts int               `json:"failed_attempts,omitempty"`
	LastSuccess    *time.Time        `json:"last_success,omitempty"`
	Patterns       map[string]string `json:"patterns,omitempty"` // Detected suspicious patterns
}

// User Behavior Analytics Model
type UserBehaviorAnalytics struct {
	UserID             string             `json:"user_id" db:"user_id"`
	AccountID          string             `json:"account_id" db:"account_id"`
	TimeRange          string             `json:"time_range" db:"time_range"`
	TotalActivities    int                `json:"total_activities" db:"total_activities"`
	UniqueActions      int                `json:"unique_actions" db:"unique_actions"`
	SessionCount       int                `json:"session_count" db:"session_count"`
	AvgSessionDuration int64              `json:"avg_session_duration_ms" db:"avg_session_duration_ms"`
	CategoryBreakdown  map[string]int     `json:"category_breakdown" db:"category_breakdown"`
	ActionFrequency    map[string]int     `json:"action_frequency" db:"action_frequency"`
	HourlyPattern      []HourlyActivity   `json:"hourly_pattern" db:"hourly_pattern"`
	DeviceUsage        map[string]int     `json:"device_usage" db:"device_usage"`
	LocationPattern    []LocationActivity `json:"location_pattern" db:"location_pattern"`
	EngagementScore    float64            `json:"engagement_score" db:"engagement_score"`
	RiskScore          int                `json:"risk_score" db:"risk_score"`
	LastActivity       time.Time          `json:"last_activity" db:"last_activity"`
	CreatedAt          time.Time          `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at" db:"updated_at"`
}

// Hourly activity pattern
type HourlyActivity struct {
	Hour  int `json:"hour"` // 0-23
	Count int `json:"count"`
}

// Location-based activity
type LocationActivity struct {
	Country string `json:"country"`
	Count   int    `json:"count"`
}

// Recommendation Interaction Model
type RecommendationInteraction struct {
	ID                 string    `json:"id" db:"id"`
	UserID             string    `json:"user_id" db:"user_id"`
	AccountID          string    `json:"account_id" db:"account_id"`
	SessionID          string    `json:"session_id" db:"session_id"`
	RecommendationType string    `json:"recommendation_type" db:"recommendation_type"` // content, user, topic
	ItemID             string    `json:"item_id" db:"item_id"`                         // recommended item
	Position           int       `json:"position" db:"position"`                       // position in recommendation list
	Action             string    `json:"action" db:"action"`                           // view, click, dismiss, like
	Context            JSONMap   `json:"context" db:"context"`                         // algorithm, score, etc.
	Timestamp          time.Time `json:"timestamp" db:"timestamp"`
	CreatedAt          time.Time `json:"created_at" db:"created_at"`
}

// Content Interaction Model
type ContentInteraction struct {
	ID              string    `json:"id" db:"id"`
	UserID          string    `json:"user_id" db:"user_id"`
	AccountID       string    `json:"account_id" db:"account_id"`
	ContentID       string    `json:"content_id" db:"content_id"`
	ContentType     string    `json:"content_type" db:"content_type"`         // blog, comment, profile
	InteractionType string    `json:"interaction_type" db:"interaction_type"` // view, like, share, comment
	Duration        int64     `json:"duration_ms,omitempty" db:"duration_ms"`
	ScrollDepth     float64   `json:"scroll_depth,omitempty" db:"scroll_depth"`       // 0-1
	ReadingSpeed    float64   `json:"reading_speed,omitempty" db:"reading_speed"`     // words per minute
	CompletionRate  float64   `json:"completion_rate,omitempty" db:"completion_rate"` // 0-1
	Context         JSONMap   `json:"context" db:"context"`
	Timestamp       time.Time `json:"timestamp" db:"timestamp"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
}

// Search Activity Model
type SearchActivity struct {
	ID             string    `json:"id" db:"id"`
	UserID         string    `json:"user_id" db:"user_id"`
	AccountID      string    `json:"account_id" db:"account_id"`
	SessionID      string    `json:"session_id" db:"session_id"`
	Query          string    `json:"query" db:"query"`
	Filters        JSONMap   `json:"filters,omitempty" db:"filters"`
	ResultCount    int       `json:"result_count" db:"result_count"`
	ClickedResults []string  `json:"clicked_results,omitempty" db:"clicked_results"`
	NoResults      bool      `json:"no_results" db:"no_results"`
	Context        JSONMap   `json:"context" db:"context"`
	Timestamp      time.Time `json:"timestamp" db:"timestamp"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
}

// Compliance Event Model (GDPR, data requests, etc.)
type ComplianceEvent struct {
	ID           string     `json:"id" db:"id"`
	UserID       string     `json:"user_id" db:"user_id"`
	AccountID    string     `json:"account_id" db:"account_id"`
	EventType    string     `json:"event_type" db:"event_type"` // data_export, data_deletion, consent_change
	Status       string     `json:"status" db:"status"`         // requested, processing, completed, failed
	RequestData  JSONMap    `json:"request_data" db:"request_data"`
	ResponseData JSONMap    `json:"response_data,omitempty" db:"response_data"`
	ProcessedBy  *string    `json:"processed_by,omitempty" db:"processed_by"`
	CompletedAt  *time.Time `json:"completed_at,omitempty" db:"completed_at"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`
}

// System Performance Event Model
type PerformanceEvent struct {
	ID         string    `json:"id" db:"id"`
	Service    string    `json:"service" db:"service"`     // which microservice
	Operation  string    `json:"operation" db:"operation"` // API endpoint or operation
	UserID     *string   `json:"user_id,omitempty" db:"user_id"`
	Duration   int64     `json:"duration_ms" db:"duration_ms"`
	StatusCode int       `json:"status_code" db:"status_code"`
	ErrorMsg   string    `json:"error_message,omitempty" db:"error_message"`
	Context    JSONMap   `json:"context" db:"context"`
	Timestamp  time.Time `json:"timestamp" db:"timestamp"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

// JSONMap for handling JSON data in PostgreSQL
type JSONMap map[string]interface{}

// Value implements the driver.Valuer interface for PostgreSQL
func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements the sql.Scanner interface for PostgreSQL
func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, j)
	case string:
		return json.Unmarshal([]byte(v), j)
	default:
		return fmt.Errorf("cannot scan %T into JSONMap", value)
	}
}

// Helper functions for creating activity events
func NewActivityEvent(userID, accountID, sessionID string, category ActivityCategory, action string) *ActivityEvent {
	return &ActivityEvent{
		UserID:    userID,
		AccountID: accountID,
		SessionID: sessionID,
		Category:  category,
		Action:    action,
		Platform:  "web",
		Success:   true,
		Timestamp: time.Now(),
		Metadata:  make(JSONMap),
	}
}

func NewSecurityEvent(userID, accountID, eventType, description string, severity Severity) *SecurityEvent {
	return &SecurityEvent{
		UserID:      userID,
		AccountID:   accountID,
		EventType:   eventType,
		Severity:    severity,
		Description: description,
		RiskScore:   0,
		Resolved:    false,
	}
}

// Reading Behavior Analytics Model
type ReadingBehaviorAnalytics struct {
	ID               string    `json:"id" db:"id"`
	UserID           string    `json:"user_id" db:"user_id"`
	AccountID        string    `json:"account_id" db:"account_id"`
	ContentID        string    `json:"content_id" db:"content_id"`
	SessionID        string    `json:"session_id" db:"session_id"`
	ScrollDepth      float64   `json:"scroll_depth" db:"scroll_depth"`           // 0-1 (percentage)
	ReadingSpeed     float64   `json:"reading_speed" db:"reading_speed"`         // words per minute
	DwellTime        int64     `json:"dwell_time_ms" db:"dwell_time_ms"`         // time spent on content
	CompletionRate   float64   `json:"completion_rate" db:"completion_rate"`     // 0-1
	ReturnVisits     int       `json:"return_visits" db:"return_visits"`         // how many times re-read
	WordCount        int       `json:"word_count" db:"word_count"`               // content length
	ReadabilityScore float64   `json:"readability_score" db:"readability_score"` // flesch-kincaid, etc.
	DeviceType       string    `json:"device_type" db:"device_type"`             // mobile, desktop, tablet
	TimeOfDay        int       `json:"time_of_day" db:"time_of_day"`             // hour 0-23
	IsWeekend        bool      `json:"is_weekend" db:"is_weekend"`
	ContentFormat    string    `json:"content_format" db:"content_format"` // long-form, short-form
	HasImages        bool      `json:"has_images" db:"has_images"`
	HasLinks         bool      `json:"has_links" db:"has_links"`
	LinkClicks       int       `json:"link_clicks" db:"link_clicks"`
	ImageViews       int       `json:"image_views" db:"image_views"`
	Timestamp        time.Time `json:"timestamp" db:"timestamp"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
}

// Notification Event Model
type NotificationEvent struct {
	ID              string     `json:"id" db:"id"`
	UserID          string     `json:"user_id" db:"user_id"`
	AccountID       string     `json:"account_id" db:"account_id"`
	Type            string     `json:"type" db:"type"`                 // email, push, in-app, sms
	Channel         string     `json:"channel" db:"channel"`           // marketing, transactional, system
	Template        string     `json:"template" db:"template"`         // which template used
	Subject         string     `json:"subject,omitempty" db:"subject"` // email subject or notification title
	Status          string     `json:"status" db:"status"`             // sent, delivered, opened, clicked, bounced
	DeliveryTime    int64      `json:"delivery_time_ms" db:"delivery_time_ms"`
	OpenTime        *time.Time `json:"open_time,omitempty" db:"open_time"`
	ClickTime       *time.Time `json:"click_time,omitempty" db:"click_time"`
	ClickedLinks    []string   `json:"clicked_links,omitempty" db:"clicked_links"`
	DeviceInfo      JSONMap    `json:"device_info,omitempty" db:"device_info"`
	LocationInfo    JSONMap    `json:"location_info,omitempty" db:"location_info"`
	BounceReason    string     `json:"bounce_reason,omitempty" db:"bounce_reason"`
	UnsubscribeTime *time.Time `json:"unsubscribe_time,omitempty" db:"unsubscribe_time"`
	Context         JSONMap    `json:"context" db:"context"`
	Timestamp       time.Time  `json:"timestamp" db:"timestamp"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`
}

// Financial Transaction Model
type FinancialEvent struct {
	ID              string     `json:"id" db:"id"`
	UserID          string     `json:"user_id" db:"user_id"`
	AccountID       string     `json:"account_id" db:"account_id"`
	TransactionType string     `json:"transaction_type" db:"transaction_type"` // payment, refund, subscription
	Amount          float64    `json:"amount" db:"amount"`
	Currency        string     `json:"currency" db:"currency"`
	Status          string     `json:"status" db:"status"`                 // pending, completed, failed, cancelled
	PaymentMethod   string     `json:"payment_method" db:"payment_method"` // credit_card, paypal, stripe
	TransactionID   string     `json:"transaction_id" db:"transaction_id"` // external payment ID
	SubscriptionID  *string    `json:"subscription_id,omitempty" db:"subscription_id"`
	PlanID          *string    `json:"plan_id,omitempty" db:"plan_id"`
	BillingCycle    *string    `json:"billing_cycle,omitempty" db:"billing_cycle"` // monthly, yearly
	TrialDays       *int       `json:"trial_days,omitempty" db:"trial_days"`
	DiscountCode    *string    `json:"discount_code,omitempty" db:"discount_code"`
	DiscountAmount  *float64   `json:"discount_amount,omitempty" db:"discount_amount"`
	TaxAmount       *float64   `json:"tax_amount,omitempty" db:"tax_amount"`
	ProcessingFee   *float64   `json:"processing_fee,omitempty" db:"processing_fee"`
	FailureReason   *string    `json:"failure_reason,omitempty" db:"failure_reason"`
	RefundReason    *string    `json:"refund_reason,omitempty" db:"refund_reason"`
	Context         JSONMap    `json:"context" db:"context"`
	ProcessedAt     *time.Time `json:"processed_at,omitempty" db:"processed_at"`
	Timestamp       time.Time  `json:"timestamp" db:"timestamp"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`
}

// API Integration Event Model
type IntegrationEvent struct {
	ID              string    `json:"id" db:"id"`
	UserID          *string   `json:"user_id,omitempty" db:"user_id"`
	AccountID       *string   `json:"account_id,omitempty" db:"account_id"`
	IntegrationType string    `json:"integration_type" db:"integration_type"` // webhook, api, oauth
	Service         string    `json:"service" db:"service"`                   // external service name
	Endpoint        string    `json:"endpoint" db:"endpoint"`                 // API endpoint or webhook URL
	Method          string    `json:"method" db:"method"`                     // GET, POST, PUT, DELETE
	StatusCode      int       `json:"status_code" db:"status_code"`
	ResponseTime    int64     `json:"response_time_ms" db:"response_time_ms"`
	PayloadSize     int64     `json:"payload_size_bytes" db:"payload_size_bytes"`
	APIKey          *string   `json:"api_key_hash,omitempty" db:"api_key_hash"`         // hashed API key
	OAuthToken      *string   `json:"oauth_token_hash,omitempty" db:"oauth_token_hash"` // hashed token
	RetryCount      int       `json:"retry_count" db:"retry_count"`
	Success         bool      `json:"success" db:"success"`
	ErrorMessage    *string   `json:"error_message,omitempty" db:"error_message"`
	RateLimited     bool      `json:"rate_limited" db:"rate_limited"`
	Context         JSONMap   `json:"context" db:"context"`
	Timestamp       time.Time `json:"timestamp" db:"timestamp"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
}

// Incident Response Event Model
type IncidentEvent struct {
	ID                 string     `json:"id" db:"id"`
	IncidentID         string     `json:"incident_id" db:"incident_id"`     // unique incident identifier
	IncidentType       string     `json:"incident_type" db:"incident_type"` // security, data_breach, service_outage
	Severity           Severity   `json:"severity" db:"severity"`
	Status             string     `json:"status" db:"status"` // detected, investigating, resolved
	AffectedUsers      []string   `json:"affected_users,omitempty" db:"affected_users"`
	DataBreach         bool       `json:"data_breach" db:"data_breach"`
	DataTypes          []string   `json:"data_types,omitempty" db:"data_types"`   // email, password, personal_info
	DetectionMethod    string     `json:"detection_method" db:"detection_method"` // automated, manual, external_report
	Description        string     `json:"description" db:"description"`
	ImpactScope        string     `json:"impact_scope" db:"impact_scope"` // user_data, service, financial
	ResponseTeam       []string   `json:"response_team,omitempty" db:"response_team"`
	Actions            []string   `json:"actions,omitempty" db:"actions"` // actions taken
	NotificationsSent  bool       `json:"notifications_sent" db:"notifications_sent"`
	RegulatorsNotified bool       `json:"regulators_notified" db:"regulators_notified"`
	ForensicData       JSONMap    `json:"forensic_data,omitempty" db:"forensic_data"`
	ResolutionTime     int64      `json:"resolution_time_ms,omitempty" db:"resolution_time_ms"`
	ResolvedBy         *string    `json:"resolved_by,omitempty" db:"resolved_by"`
	Context            JSONMap    `json:"context" db:"context"`
	DetectedAt         time.Time  `json:"detected_at" db:"detected_at"`
	ResolvedAt         *time.Time `json:"resolved_at,omitempty" db:"resolved_at"`
	CreatedAt          time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at" db:"updated_at"`
}

// User Journey Tracking Model
type UserJourneyEvent struct {
	ID            string    `json:"id" db:"id"`
	UserID        string    `json:"user_id" db:"user_id"`
	AccountID     string    `json:"account_id" db:"account_id"`
	SessionID     string    `json:"session_id" db:"session_id"`
	JourneyStage  string    `json:"journey_stage" db:"journey_stage"`     // onboarding, active, retention, churn
	Funnel        string    `json:"funnel" db:"funnel"`                   // registration, content_creation, engagement
	Step          int       `json:"step" db:"step"`                       // step number in funnel
	StepName      string    `json:"step_name" db:"step_name"`             // descriptive step name
	Conversion    bool      `json:"conversion" db:"conversion"`           // did user convert at this step
	DropOff       bool      `json:"drop_off" db:"drop_off"`               // did user drop off
	TimeToStep    int64     `json:"time_to_step_ms" db:"time_to_step_ms"` // time from previous step
	ABTestVariant *string   `json:"ab_test_variant,omitempty" db:"ab_test_variant"`
	Referrer      string    `json:"referrer,omitempty" db:"referrer"`
	UTMSource     *string   `json:"utm_source,omitempty" db:"utm_source"`
	UTMMedium     *string   `json:"utm_medium,omitempty" db:"utm_medium"`
	UTMCampaign   *string   `json:"utm_campaign,omitempty" db:"utm_campaign"`
	Context       JSONMap   `json:"context" db:"context"`
	Timestamp     time.Time `json:"timestamp" db:"timestamp"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

// Content Analytics Model (detailed content performance)
type ContentAnalytics struct {
	ID                        string            `json:"id" db:"id"`
	ContentID                 string            `json:"content_id" db:"content_id"`
	AuthorID                  string            `json:"author_id" db:"author_id"`
	ContentType               string            `json:"content_type" db:"content_type"`
	Title                     string            `json:"title" db:"title"`
	WordCount                 int               `json:"word_count" db:"word_count"`
	ReadabilityScore          float64           `json:"readability_score" db:"readability_score"`
	TopicTags                 []string          `json:"topic_tags,omitempty" db:"topic_tags"`
	Views                     int64             `json:"views" db:"views"`
	UniqueViews               int64             `json:"unique_views" db:"unique_views"`
	AvgReadingTime            int64             `json:"avg_reading_time_ms" db:"avg_reading_time_ms"`
	CompletionRate            float64           `json:"completion_rate" db:"completion_rate"`
	BounceRate                float64           `json:"bounce_rate" db:"bounce_rate"`
	ShareCount                int               `json:"share_count" db:"share_count"`
	LikeCount                 int               `json:"like_count" db:"like_count"`
	CommentCount              int               `json:"comment_count" db:"comment_count"`
	BookmarkCount             int               `json:"bookmark_count" db:"bookmark_count"`
	EngagementScore           float64           `json:"engagement_score" db:"engagement_score"`
	ViralityScore             float64           `json:"virality_score" db:"virality_score"`
	RecommendationImpressions int64             `json:"recommendation_impressions" db:"recommendation_impressions"`
	SearchImpressions         int64             `json:"search_impressions" db:"search_impressions"`
	DirectAccess              int64             `json:"direct_access" db:"direct_access"`
	ReferralSources           map[string]int64  `json:"referral_sources" db:"referral_sources"`
	GeographicReach           map[string]int64  `json:"geographic_reach" db:"geographic_reach"`
	DeviceBreakdown           map[string]int64  `json:"device_breakdown" db:"device_breakdown"`
	TimeSpentDistribution     []TimeSpentBucket `json:"time_spent_distribution" db:"time_spent_distribution"`
	PeakViewingHours          []int             `json:"peak_viewing_hours" db:"peak_viewing_hours"`
	UpdatedAt                 time.Time         `json:"updated_at" db:"updated_at"`
	CreatedAt                 time.Time         `json:"created_at" db:"created_at"`
}

// Time spent bucket for content analytics
type TimeSpentBucket struct {
	MinSeconds int `json:"min_seconds"`
	MaxSeconds int `json:"max_seconds"`
	Count      int `json:"count"`
}

// Helper functions for new models
func NewNotificationEvent(userID, accountID, notificationType, channel string) *NotificationEvent {
	return &NotificationEvent{
		UserID:    userID,
		AccountID: accountID,
		Type:      notificationType,
		Channel:   channel,
		Status:    "sent",
		Context:   make(JSONMap),
		Timestamp: time.Now(),
	}
}

func NewFinancialEvent(userID, accountID, transactionType string, amount float64, currency string) *FinancialEvent {
	return &FinancialEvent{
		UserID:          userID,
		AccountID:       accountID,
		TransactionType: transactionType,
		Amount:          amount,
		Currency:        currency,
		Status:          "pending",
		Context:         make(JSONMap),
		Timestamp:       time.Now(),
	}
}

func NewIncidentEvent(incidentType, description string, severity Severity) *IncidentEvent {
	return &IncidentEvent{
		IncidentType:    incidentType,
		Severity:        severity,
		Status:          "detected",
		Description:     description,
		DataBreach:      false,
		DetectionMethod: "automated",
		Context:         make(JSONMap),
		DetectedAt:      time.Now(),
	}
}

func NewUserJourneyEvent(userID, accountID, sessionID, stage, funnel string, step int) *UserJourneyEvent {
	return &UserJourneyEvent{
		UserID:       userID,
		AccountID:    accountID,
		SessionID:    sessionID,
		JourneyStage: stage,
		Funnel:       funnel,
		Step:         step,
		Conversion:   false,
		DropOff:      false,
		Context:      make(JSONMap),
		Timestamp:    time.Now(),
	}
}

// Validation helpers
func (ae *ActivityEvent) IsValid() error {
	if ae.UserID == "" {
		return fmt.Errorf("user_id is required")
	}
	if ae.AccountID == "" {
		return fmt.Errorf("account_id is required")
	}
	if ae.Category == "" {
		return fmt.Errorf("category is required")
	}
	if ae.Action == "" {
		return fmt.Errorf("action is required")
	}
	return nil
}

func (se *SecurityEvent) IsValid() error {
	if se.UserID == "" {
		return fmt.Errorf("user_id is required")
	}
	if se.EventType == "" {
		return fmt.Errorf("event_type is required")
	}
	if se.Severity == "" {
		return fmt.Errorf("severity is required")
	}
	return nil
}

func (ne *NotificationEvent) IsValid() error {
	if ne.UserID == "" {
		return fmt.Errorf("user_id is required")
	}
	if ne.Type == "" {
		return fmt.Errorf("type is required")
	}
	if ne.Channel == "" {
		return fmt.Errorf("channel is required")
	}
	return nil
}
