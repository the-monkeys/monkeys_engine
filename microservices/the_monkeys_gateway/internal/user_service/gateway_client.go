package user_service

import (
	"context"

	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_user/pb"
)

func (asc *UserServiceClient) GetBlogsIds(accountId string, blogType string) (*pb.BlogsByUserNameRes, error) {
	res, err := asc.Client.GetBlogsByUserIds(context.Background(), &pb.BlogsByUserIdsReq{
		AccountId: accountId,
		Type:      blogType,
	})

	return res, err
}

func (asc *UserServiceClient) GetUserDetails(username string) (*pb.UserDetailsResp, error) {
	res, err := asc.Client.GetUserDetails(context.Background(), &pb.UserDetailReq{
		Username: username,
	})

	return res, err
}

func (asc *UserServiceClient) GetFollowingAccounts(followersUsername string) (*pb.FollowerFollowingResp, error) {
	resp, err := asc.Client.GetFollowing(context.Background(), &pb.UserDetailReq{
		Username: followersUsername,
	})

	return resp, err
}

func (asc *UserServiceClient) GetNoOfLikeCounts(blogId string) (int32, error) {
	resp, err := asc.Client.GetLikeCounts(context.Background(), &pb.BookMarkReq{
		BlogId: blogId,
	})

	if err != nil {
		return 0, err
	}
	return resp.Count, nil
}

func (asc *UserServiceClient) GetNoOfBookmarkCounts(blogId string) (int32, error) {
	resp, err := asc.Client.GetBookMarkCounts(context.Background(), &pb.BookMarkReq{
		BlogId: blogId,
	})

	if err != nil {
		return 0, err
	}
	return resp.Count, nil
}

func (asc *UserServiceClient) HaveILikedTheBlog(blogId, userName string) (bool, error) {
	res, err := asc.Client.GetIfBlogLiked(context.Background(), &pb.BookMarkReq{
		Username: userName,
		BlogId:   blogId,
	})

	if err != nil {
		return false, err
	}

	return res.BookMarked, nil
}

func (asc *UserServiceClient) HaveIBookmarkedTheBlog(blogId, userName string) (bool, error) {
	res, err := asc.Client.GetIfBlogBookMarked(context.Background(), &pb.BookMarkReq{
		Username: userName,
		BlogId:   blogId,
	})

	if err != nil {
		return false, err
	}

	return res.BookMarked, nil
}

func (asc *UserServiceClient) GetUsersBookmarks(username string) ([]string, error) {
	res, err := asc.Client.GetBookMarks(context.Background(), &pb.BookMarkReq{
		Username: username,
	})

	return res.BlogIds, err
}
