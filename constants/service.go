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
	EventUpdateUsername    = "event-update-username"
	EventUpdateEmail       = "event-update-email"
	EventFollowTopics      = "event-follow-topics"
	EventUnFollowTopics    = "event-un-follow-topics"
	EventCreateTopics      = "event-create-topics"
	EventBookMarkBlog      = "event-bookmark-blog"
	EventRemoveBookMark    = "event-remove-bookmark"
	EventFollowUser        = "event-follow-user"
	EventUnFollowUser      = "event-un-follow-user"
	EventBlogLike          = "event-blog-like"
	EventBlogUnlike        = "event-blog-unlike"

	EventCreatedBlog    = "event-created-blog"
	EventDraftedBlog    = "event-drafted-blog"
	EventPublishedBlog  = "event-published-blog"
	EventDeleteBlog     = "event-delete-blog"
	EventInviteCoAuthor = "event-invite-co-author"
	EventAcceptCoAuthor = "event-accept-co-author"
	EventRemoveCoAuthor = "event-remove-co-author"
)

const (
	ServiceGateway     = "the-monkeys-gateway"
	ServiceAuth        = "the-monkeys-authz"
	ServiceUser        = "the-monkeys-user"
	ServiceBlog        = "the-monkeys-blog"
	ServiceFileStorage = "the-monkeys-file-storage"
	ServiceStream      = "the-monkeys-stream"
)
