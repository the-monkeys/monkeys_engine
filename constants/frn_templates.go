package constants

// FRN Template names — registered once in FreeRangeNotify, referenced by name.
const (
	// Social
	FRNTplNewFollowerInApp = "new_follower_inapp"
	FRNTplNewFollowerSSE   = "new_follower_sse"
	FRNTplNewCommentInApp  = "new_comment_inapp"
	FRNTplNewCommentSSE    = "new_comment_sse"
	FRNTplBlogLikedInApp   = "blog_liked_inapp"

	// Collaboration
	FRNTplCoAuthorInviteInApp  = "coauthor_invite_inapp"
	FRNTplCoAuthorInviteSSE    = "coauthor_invite_sse"
	FRNTplCoAuthorInviteEmail  = "coauthor_invite_email"
	FRNTplCoAuthorAcceptInApp  = "coauthor_accept_inapp"
	FRNTplCoAuthorAcceptSSE    = "coauthor_accept_sse"
	FRNTplCoAuthorDeclineInApp = "coauthor_decline_inapp"
	FRNTplCoAuthorRemovedInApp = "coauthor_removed_inapp"
	FRNTplCoAuthorRemovedSSE   = "coauthor_removed_sse"

	// Content
	FRNTplBlogPublishedCoAuthorInApp = "blog_published_coauthor_inapp"
	FRNTplBlogPublishedCoAuthorSSE   = "blog_published_coauthor_sse"

	// Security
	FRNTplPasswordChangedInApp  = "password_changed_inapp"
	FRNTplPasswordChangedEmail  = "password_changed_email"
	FRNTplEmailVerifiedInApp    = "email_verified_inapp"
	FRNTplLoginDetectedInApp    = "login_detected_inapp"
	FRNTplLoginDetectedSSE      = "login_detected_sse"
	FRNTplLoginDetectedEmail    = "login_detected_email"
	FRNTplPasswordResetReqInApp = "password_reset_requested_inapp"
	FRNTplPasswordResetReqEmail = "password_reset_requested_email"
	FRNTplEmailChangedInApp     = "email_changed_inapp"
	FRNTplEmailChangedEmail     = "email_changed_email"
	FRNTplUsernameChangedInApp  = "username_changed_inapp"
)

// FRN Categories
const (
	FRNCategorySocial        = "social"
	FRNCategoryCollaboration = "collaboration"
	FRNCategoryContent       = "content"
	FRNCategorySecurity      = "security"
)
