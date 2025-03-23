package consumer

import (
	"context"

	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_user/pb"
)

func (asc *UserDbConn) GetBlogsIds(accountId string, blogType string) (*pb.BlogsByUserNameRes, error) {
	res, err := asc.userClient.GetBlogsByUserIds(context.Background(), &pb.BlogsByUserIdsReq{
		AccountId: accountId,
		Type:      blogType,
	})

	return res, err
}

func (asc *UserDbConn) GetUserDetails(username string) (*pb.UserDetailsResp, error) {
	res, err := asc.userClient.GetUserDetails(context.Background(), &pb.UserDetailReq{
		Username: username,
	})

	return res, err
}

func (asc *UserDbConn) GetFollowingAccounts(followersUsername string) (*pb.FollowerFollowingResp, error) {
	resp, err := asc.userClient.GetFollowing(context.Background(), &pb.UserDetailReq{
		Username: followersUsername,
	})

	return resp, err
}

func (asc *UserDbConn) GetNoOfLikeCounts(blogId string) (int32, error) {
	resp, err := asc.userClient.GetLikeCounts(context.Background(), &pb.BookMarkReq{
		BlogId: blogId,
	})

	if err != nil {
		return 0, err
	}
	return resp.Count, nil
}

func (asc *UserDbConn) GetNoOfBookmarkCounts(blogId string) (int32, error) {
	resp, err := asc.userClient.GetBookMarkCounts(context.Background(), &pb.BookMarkReq{
		BlogId: blogId,
	})

	if err != nil {
		return 0, err
	}
	return resp.Count, nil
}

func (asc *UserDbConn) HaveILikedTheBlog(blogId, userName string) (bool, error) {
	res, err := asc.userClient.GetIfBlogLiked(context.Background(), &pb.BookMarkReq{
		Username: userName,
		BlogId:   blogId,
	})

	if err != nil {
		return false, err
	}

	return res.BookMarked, nil
}

func (asc *UserDbConn) HaveIBookmarkedTheBlog(blogId, userName string) (bool, error) {
	res, err := asc.userClient.GetIfBlogBookMarked(context.Background(), &pb.BookMarkReq{
		Username: userName,
		BlogId:   blogId,
	})

	if err != nil {
		return false, err
	}

	return res.BookMarked, nil
}

func (asc *UserDbConn) GetUsersBookmarks(username string) ([]string, error) {
	res, err := asc.userClient.GetBookMarks(context.Background(), &pb.BookMarkReq{
		Username: username,
	})

	return res.BlogIds, err
}
