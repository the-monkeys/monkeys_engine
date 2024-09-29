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
