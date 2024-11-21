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

	DELETE_USERS_ALL_BLOGS = "delete_users_all_blogs"
)

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
