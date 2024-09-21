package user_service

import (
	"context"

	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_user/pb"
)

func (asc *UserServiceClient) GetColabBlogs(username string) (*pb.BlogsByUserNameRes, error) {
	res, err := asc.Client.GetBlogsByUserName(context.Background(), &pb.BlogsByUserNameReq{
		Username: username,
	})

	return res, err
}
