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
