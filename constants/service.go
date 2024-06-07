package constants

const (
	EventRegister                    = "event-register"
	EventPasswordReset               = "event-password-reset"
	EventLogin                       = "event-login"
	EventForgotPassword              = "event-forgot-password"
	EventVerifiedEmailForPassChange  = "event-verified-email-for-pass-change"
	EventUpdatedPassword             = "event-updated-password"
	EventRequestForEmailVerification = "event-request-for-email-verification"
	EventVerifiedEmail               = "event-verified-email"

	EventUpdateProfileInfo = "event-update-profile-info"
)

const (
	ServiceGateway     = "the-monkeys-gateway"
	ServiceAuth        = "the-monkeys-authz"
	ServiceUser        = "the-monkeys-user"
	ServiceBlog        = "the-monkeys-blog"
	ServiceFileStorage = "the-monkeys-file-storage"
	ServiceStream      = "the-monkeys-stream"
)
