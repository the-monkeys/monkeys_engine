package constants

const (
	UserPubilc  = "public"
	UserPrivate = "private"
	UserFriends = "friends"
)

const (
	RoleAdmin  = "admin"
	RoleOwner  = "Owner"
	RoleEditor = "Editor"
	RoleViewer = "viewer"
)

const (
	PermissionRead    = "read"
	PermissionEdit    = "Edit"
	PermissionDelete  = "delete"
	PermissionAchieve = "archive"
	PermissionCreate  = "Create"
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

var Clients = []string{"Chrome", "Firefox", "Safari", "Edge", "Opera", "Android", "iOS", "Brave", "Others"}
