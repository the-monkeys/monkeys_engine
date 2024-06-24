package constants

const (
	UserPubilc  = "public"
	UserPrivate = "private"
	UserFriends = "friends"
)

const (
	RoleAdmin  = "admin"
	RoleOwner  = "owner"
	RoleEditor = "editor"
	RoleViewer = "viewer"
)

const (
	PermissionRead    = "read"
	PermissionEdit    = "edit"
	PermissionDelete  = "delete"
	PermissionAchieve = "archive"
)

const (
	ClientChrome  = "chrome"
	ClientFirefox = "firefox"
	ClientSafari  = "safari"
	ClientEdge    = "edge"
	ClientOpera   = "opera"
	ClientAndroid = "android"
	ClientIOS     = "ios"
	ClientBrave   = "brave"
	ClientOthers  = "others"
)

const (
	EmailVerificationStatusUnverified       = "unverified"
	EmailVerificationStatusVerificationSent = "verification-link-sent"
	EmailVerificationStatusVerified         = "verified"
)

const (
	AuthTheMonkeys   = "the-monkeys"
	AuthGoogleOauth2 = "google-oauth2"
	AuthInstaOauth2  = "instagram-oauth2"
)

const (
	UserActive   = "active"
	UserInactive = "inactive"
	UserHidden   = "hidden"
)

const (
	BlogStatusDraft     = "draft"
	BlogStatusPublished = "published"
	BlogStatusArchived  = "archived"
)

var Clients = []string{"chrome", "firefox", "safari", "edge", "opera", "android", "ios", "brave", "others"}
