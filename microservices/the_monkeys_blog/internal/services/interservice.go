package services

// import (
// 	"context"
// 	"net/http"

// 	"github.com/sirupsen/logrus"
// 	"github.com/the-monkeys/the_monkeys/apis/interservice/blogs/pb"
// 	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_blog/internal/psql"
// )

// type Interservice struct {
// 	osClient osClient
// 	pgClient *psql.PostDBHandler
// 	logger   *logrus.Logger
// 	pb.UnimplementedBlogServiceServer
// }

// func NewInterservice(client osClient, logger *logrus.Logger) *Interservice {
// 	return &Interservice{osClient: client, logger: logger}
// }

// func (blog *Interservice) SetUserDeactivated(ctx context.Context, req *pb.SetUserDeactivatedReq) (*pb.SetUserDeactivatedRes, error) {
// 	blog.logger.Infof("User is deactivated: %s", req.Email)

// 	// TODO: Set all the users status key in the blog as disabled and not show users blog to the portal.

// 	return &pb.SetUserDeactivatedRes{
// 		Status:  http.StatusOK,
// 		Message: "updated successfully",
// 	}, nil
// }
