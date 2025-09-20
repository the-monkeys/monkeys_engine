package constants

const (
	USER_REGISTER       = "user_register"
	USERNAME_UPDATE     = "username_update"
	USER_ACCOUNT_DELETE = "user_profile_directory_delete"
	BLOG_CREATE         = "blog_create"
	BLOG_UPDATE         = "blog_update"
	BLOG_PUBLISH        = "published"
	BLOG_DELETE         = "delete"
	BLOG_LIKE           = "blog_like"
	USER_FOLLOWED       = "user_followed"

	DELETE_USERS_ALL_BLOGS = "delete_users_all_blogs"

	PROFILE_UPDATE = "profile_update"
)

// RestrictedUsernames contains usernames that are not allowed for user registration
var RestrictedUsernames = []string{
	"admin", "administrator", "root", "superuser", "support", "help",
	"contact", "info", "user", "username", "null", "undefined",
	"system", "api", "www", "mail", "ftp", "blog", "news", "forum",
	"test", "demo", "guest", "public", "private", "staff", "moderator",
	"team", "teams", "feed", "feedback", "themonkeys", "the_monkeys", "themonkeysadmin", "themonkeysadministrator",
	"about", "access", "account", "accounts", "activity", "add", "address", "addresses",
	"all", "analytics", "and", "api", "apps", "archive", "archives", "area", "areas",
	"auth", "authentication", "authorize", "auto", "backup", "backups", "base", "billing",
}

const (
	NotificationRegister = `Subject: Welcome! Complete Your Registration

Thank you for signing up with The Monkeys!

To complete your registration and activate your account, please click the link sent to your email or copy and paste it into your browser:

We're excited to have you on board!

Best regards,
The Monkeys Team
`

	NotificationBlogLicked = `Subject: Blog Liked! 
		%s liked your blog %s
	`
)
