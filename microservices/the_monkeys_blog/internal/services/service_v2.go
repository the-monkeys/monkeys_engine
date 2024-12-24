package services

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_blog/pb"
	"github.com/the-monkeys/the_monkeys/constants"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_blog/internal/models"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

func (blog *BlogService) DraftBlogV2(stream grpc.BidiStreamingServer[anypb.Any, anypb.Any]) error {
	for {
		// Receive a message from the client
		reqAny, err := stream.Recv()
		if err == io.EOF {
			// Client has closed the stream
			blog.logger.Debugf("Client closed the stream")
			// Send message to the user service to update the blog status
			return nil
		}
		if err != nil {
			blog.logger.Errorf("Error receiving message from stream: %v", err)
			return status.Errorf(codes.Internal, "Error receiving message: %v", err)
		}

		// Unmarshal the incoming Any message into a struct
		reqStruct := &structpb.Struct{}
		if err := anypb.UnmarshalTo(reqAny, reqStruct, proto.UnmarshalOptions{}); err != nil {
			blog.logger.Errorf("Error unmarshaling message: %v", err)
			return status.Errorf(codes.InvalidArgument, "Invalid message format: %v", err)
		}

		// Convert the struct to a map for further processing
		req := reqStruct.AsMap()

		blog.logger.Infof("Received a blog containing id: %v", req["blog_id"])
		req["is_draft"] = true

		blogId := req["blog_id"].(string)
		ownerAccountId := req["owner_account_id"].(string)
		ip := req["Ip"].(string)
		client := req["Client"].(string)
		tagsInterface, ok := req["tags"].([]interface{})
		if !ok {
			blog.logger.Errorf("Tags field is not of type []interface{}")
			tagsInterface = []interface{}{"untagged"}
		}
		tags := make([]string, len(tagsInterface))
		for i, v := range tagsInterface {
			tags[i], ok = v.(string)
			if !ok {
				blog.logger.Errorf("Tag value is not of type string")
				return status.Errorf(codes.InvalidArgument, "Tag value is not of type string")
			}
		}

		exists, _, _ := blog.osClient.DoesBlogExist(stream.Context(), req["blog_id"].(string))
		if exists {
			blog.logger.Infof("Updating the blog with id: %s", blogId)
			// Additional logic for existing blog handling
		} else {
			blog.logger.Infof("Creating the blog with id: %s for author: %s", blogId, ownerAccountId)
			bx, err := json.Marshal(models.InterServiceMessage{
				AccountId:  ownerAccountId,
				BlogId:     blogId,
				Action:     constants.BLOG_CREATE,
				BlogStatus: constants.BlogStatusDraft,
				IpAddress:  ip,
				Client:     client,
			})
			if err != nil {
				blog.logger.Errorf("Cannot marshal the message for blog: %s, error: %v", blogId, err)
				return status.Errorf(codes.Internal, "Something went wrong while drafting a blog")
			}
			if len(tags) == 0 {
				req["Tags"] = []string{"untagged"}
			}
			go blog.qConn.PublishMessage(blog.config.RabbitMQ.Exchange, blog.config.RabbitMQ.RoutingKeys[1], bx)
		}

		_, err = blog.osClient.SaveBlog(stream.Context(), req)
		if err != nil {
			blog.logger.Errorf("Cannot store draft into opensearch: %v", err)
			return status.Errorf(codes.Internal, "Failed to store draft: %v", err)
		}

		// // Respond back to the client
		// resp := &pb.BlogResponse{
		// 	Blog: req.Blog,
		// }
		// respAny, err := anypb.New(resp)
		// if err != nil {
		// 	blog.logger.Errorf("Error marshalling response: %v", err)
		// 	return status.Errorf(codes.Internal, "Failed to create response: %v", err)
		// }

		// TODO: Change the return data to the actual response
		anyMsg, err := anypb.New(reqStruct)
		if err != nil {
			logrus.Errorf("Error wrapping structpb.Struct in anypb.Any: %v", err)
			return status.Errorf(codes.Internal, "Failed to wrap struct in Any: %v", err)
		}

		if err := stream.Send(anyMsg); err != nil {
			blog.logger.Errorf("Error sending response: %v", err)
			return status.Errorf(codes.Internal, "Failed to send response: %v", err)
		}
	}
}

func (blog *BlogService) BlogsOfFollowingAccounts(req *pb.FollowingAccounts, stream pb.BlogService_BlogsOfFollowingAccountsServer) error {
	blog.logger.Debugf("Received request for blogs of following accounts: %v", req.AccountIds)

	if len(req.AccountIds) == 0 {
		return status.Errorf(codes.InvalidArgument, "No account ids provided")
	}

	blogs, err := blog.osClient.GetBlogsOfUsersByAccountIds(stream.Context(), req.AccountIds, req.Limit, req.Offset)
	if err != nil {
		blog.logger.Errorf("Error fetching blogs of following accounts: %v", err)
		return status.Errorf(codes.Internal, "Error fetching blogs of following accounts: %v", err)
	}

	// TODO: remove a key from here blogs blogs = []map[string]interface{}
	removeKeyFromBlogs(blogs, "action")
	removeKeyFromBlogs(blogs, "Ip")
	removeKeyFromBlogs(blogs, "Client")

	blogBytes, err := json.Marshal(blogs)
	if err != nil {
		blog.logger.Errorf("Error marshalling blogs: %v", err)
		return status.Errorf(codes.Internal, "Error marshalling blogs: %v", err)
	}

	fmt.Printf("blogBytes: %v\n", string(blogBytes))

	// Send the packed message over the stream
	if err := stream.Send(&anypb.Any{
		TypeUrl: "the-monkeys/the-monkeys/apis/serviceconn/gateway_blog/pb.BlogResponse",
		Value:   blogBytes,
	}); err != nil {
		return err
	}

	return nil
}

func removeKeyFromBlogs(blogs []map[string]interface{}, key string) {
	for _, blog := range blogs {
		delete(blog, key)
	}
}

func (blog *BlogService) GetBlogs(req *pb.GetBlogsReq, stream pb.BlogService_GetBlogsServer) error {
	blog.logger.Debugf("Received request for blogs: %v", req)

	var blogs []map[string]interface{}
	var err error

	// Check if specific tags are requested
	if len(req.Tags) > 0 {
		if req.IsDraft {
			blog.logger.Debug("Fetching draft blogs by tags")
		} else {
			blog.logger.Debug("Fetching published blogs by tags")
			blogs, err = blog.osClient.GetBlogsByTags(stream.Context(), req.Tags, false, req.Limit, req.Offset)
			if err != nil {
				blog.logger.Errorf("Error fetching blogs by tags: %v", err)
				return status.Errorf(codes.Internal, "Error fetching blogs by tags: %v", err)
			}
		}

		// TODO: remove a key from here blogs blogs = []map[string]interface{}
		removeKeyFromBlogs(blogs, "action")
		removeKeyFromBlogs(blogs, "Ip")
		removeKeyFromBlogs(blogs, "Client")

		blogBytes, err := json.Marshal(blogs)
		if err != nil {
			blog.logger.Errorf("Error marshalling blogs: %v", err)
			return status.Errorf(codes.Internal, "Error marshalling blogs: %v", err)
		}

		fmt.Printf("blogBytes: %v\n", string(blogBytes))

		// Send the packed message over the stream
		if err := stream.Send(&anypb.Any{
			TypeUrl: "the-monkeys/the-monkeys/apis/serviceconn/gateway_blog/pb.BlogResponse",
			Value:   blogBytes,
		}); err != nil {
			return err
		}

		return nil
	}

	// No tags provided, handle based on draft or published status
	if req.IsDraft {
		blog.logger.Debug("Fetching draft blogs by account ID")
		blogs, err = blog.osClient.GetBlogsByAccountId(stream.Context(), req.AccountId, true, req.Limit, req.Offset)
		if err != nil {
			blog.logger.Errorf("Error fetching draft blogs by account ID: %v", err)
			return status.Errorf(codes.Internal, "Error fetching draft blogs by account ID: %v", err)
		}
	} else {
		blog.logger.Debug("Fetching published blogs by account ID")
		blogs, err = blog.osClient.GetBlogsByAccountId(stream.Context(), req.AccountId, false, req.Limit, req.Offset)
		if err != nil {
			blog.logger.Errorf("Error fetching published blogs by account ID: %v", err)
			return status.Errorf(codes.Internal, "Error fetching published blogs by account ID: %v", err)
		}
	}

	// TODO: remove a key from here blogs blogs = []map[string]interface{}
	removeKeyFromBlogs(blogs, "action")
	removeKeyFromBlogs(blogs, "Ip")
	removeKeyFromBlogs(blogs, "Client")

	blogBytes, err := json.Marshal(blogs)
	if err != nil {
		blog.logger.Errorf("Error marshalling blogs: %v", err)
		return status.Errorf(codes.Internal, "Error marshalling blogs: %v", err)
	}

	fmt.Printf("blogBytes: %v\n", string(blogBytes))

	// Send the packed message over the stream
	if err := stream.Send(&anypb.Any{
		TypeUrl: "the-monkeys/the-monkeys/apis/serviceconn/gateway_blog/pb.BlogResponse",
		Value:   blogBytes,
	}); err != nil {
		return err
	}

	return nil
}
