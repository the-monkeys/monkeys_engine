package service

import (
	"context"
	"net/http"

	"github.com/89minutes/the_new_project/apis/interservice/blogs/pb"
	"github.com/89minutes/the_new_project/services/blogsandposts_service/blog_service/psql"
	"github.com/sirupsen/logrus"
)

type Interservice struct {
	osClient openSearchClient
	pgClient *psql.PostDBHandler
	logger   *logrus.Logger
	pb.UnimplementedBlogServiceServer
}

func NewInterservice(client openSearchClient,
	logger *logrus.Logger) *Interservice {
	return &Interservice{osClient: client, logger: logger}
}

func (blog *Interservice) SetUserDeactivated(ctx context.Context, req *pb.SetUserDeactivatedReq) (*pb.SetUserDeactivatedRes, error) {
	blog.logger.Infof("User is deactivated: %v", req.Email)

	// TODO: Set all the users status key in the blog as disabled and not show users blog to the portal.

	return &pb.SetUserDeactivatedRes{
		Status:  http.StatusOK,
		Message: "updated successfully",
	}, nil
}
