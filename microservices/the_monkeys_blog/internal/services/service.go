package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_blog/pb"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/constants"
	"github.com/the-monkeys/the_monkeys/microservices/rabbitmq"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_blog/internal/database"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_blog/internal/models"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_blog/internal/seo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type BlogService struct {
	osClient   database.ElasticsearchStorage
	seoManager seo.SEOManager
	logger     *zap.SugaredLogger
	config     *config.Config
	qConn      rabbitmq.Conn
	pb.UnimplementedBlogServiceServer
}

func NewBlogService(client database.ElasticsearchStorage, seoManager seo.SEOManager, logger *zap.SugaredLogger, config *config.Config, qConn rabbitmq.Conn) *BlogService {
	return &BlogService{
		osClient:   client,
		seoManager: seoManager,
		logger:     logger,
		config:     config,
		qConn:      qConn,
	}
}

func (blog *BlogService) DraftBlog(ctx context.Context, req *pb.DraftBlogRequest) (*pb.BlogResponse, error) {
	blog.logger.Debugw("draft blog", "blog_id", req.BlogId, "owner", req.OwnerAccountId)
	req.IsDraft = true

	exists, _, _ := blog.osClient.DoesBlogExist(ctx, req.BlogId)
	if exists {
		blog.logger.Infof("updating the blog with id: %s", req.BlogId)
		// owner, _, err := blog.osClient.GetBlogDetailsById(ctx, req.BlogId)
		// if err != nil {
		// 	blog.logger.Errorf("cannot find the blog with id: %s, error: %v", req.BlogId, err)
		// 	return nil, status.Errorf(codes.NotFound, "cannot find the blog with id")
		// }

		// if req.OwnerAccountId != owner {
		// 	blog.logger.Errorf("user %s is trying to take the ownership of the content, original owner is: %s", req.OwnerAccountId, owner)
		// 	return nil, status.Errorf(codes.Unauthenticated, "you don't have permission to change the owner id")
		// }
	} else {
		blog.logger.Infof("creating the blog with id: %s for author: %s", req.BlogId, req.OwnerAccountId)
		bx, err := json.Marshal(models.InterServiceMessage{
			AccountId:  req.OwnerAccountId,
			BlogId:     req.BlogId,
			Action:     constants.BLOG_CREATE,
			BlogStatus: constants.BlogStatusDraft,
			IpAddress:  req.Ip,
			Client:     req.Client,
		})

		if err != nil {
			blog.logger.Errorf("cannot marshal the message for blog: %s, error: %v", req.BlogId, err)
			return nil, status.Errorf(codes.Internal, "Something went wrong while drafting a blog")
		}

		if len(req.Tags) == 0 {
			req.Tags = []string{"untagged"}
		}
		// fmt.Printf("bx: %v\n", string(bx))
		go func() {
			err := blog.qConn.PublishMessage(blog.config.RabbitMQ.Exchange, blog.config.RabbitMQ.RoutingKeys[1], bx)
			if err != nil {
				blog.logger.Errorf("failed to publish blog create message to RabbitMQ: exchange=%s, routing_key=%s, error=%v", blog.config.RabbitMQ.Exchange, blog.config.RabbitMQ.RoutingKeys[1], err)
			}
		}()
	}

	_, err := blog.osClient.DraftABlog(ctx, req)
	if err != nil {
		blog.logger.Errorf("cannot store draft into opensearch: %v", err)
		return nil, err
	}

	return &pb.BlogResponse{
		Blog: req.Blog,
	}, nil
}

func (blog *BlogService) CheckIfBlogsExist(ctx context.Context, req *pb.BlogByIdReq) (*pb.BlogExistsRes, error) {
	exists, blogInfo, err := blog.osClient.DoesBlogExist(ctx, req.BlogId)
	if err != nil {
		blog.logger.Errorf("cannot find the blog with id: %s, error: %v", req.BlogId, err)
		return nil, status.Errorf(codes.NotFound, "cannot find the blog with id")
	}

	isDraft, ok := blogInfo["is_draft"].(bool)
	if !ok {
		blog.logger.Errorf("unexpected type for is_draft field")
		isDraft = true
	}

	return &pb.BlogExistsRes{
		BlogExists: exists,
		IsDraft:    isDraft,
	}, nil
}

func (blog *BlogService) GetDraftBlogsByAccId(ctx context.Context, req *pb.BlogByIdReq) (*pb.GetDraftBlogsRes, error) {
	blog.logger.Debugw("get draft blogs", "owner", req.OwnerAccountId)
	if req.OwnerAccountId == "" {
		blog.logger.Error("account id cannot be empty")
		return nil, status.Errorf(codes.InvalidArgument, "Account id cannot be empty")
	}

	res, err := blog.osClient.GetDraftBlogsByOwnerAccountID(ctx, req.OwnerAccountId)
	if err != nil {
		blog.logger.Errorf("error occurred while getting draft blogs for account id: %s, error: %v", req.OwnerAccountId, err)
		return nil, status.Errorf(codes.Internal, "cannot get the draft blogs for account id: %s", req.OwnerAccountId)
	}

	return res, nil
}

func (blog *BlogService) GetPublishedBlogsByAccID(ctx context.Context, req *pb.BlogByIdReq) (*pb.GetPublishedBlogsRes, error) {
	blog.logger.Debugw("get published blogs", "owner", req.OwnerAccountId)
	if req.OwnerAccountId == "" {
		blog.logger.Error("account id cannot be empty")
		return nil, status.Errorf(codes.InvalidArgument, "Account id cannot be empty")
	}

	res, err := blog.osClient.GetPublishedBlogsByOwnerAccountID(ctx, req.OwnerAccountId)
	if err != nil {
		blog.logger.Errorf("error occurred while getting published blogs for account id: %s, error: %v", req.OwnerAccountId, err)
		return nil, status.Errorf(codes.Internal, "cannot get the published blogs for account id: %s", req.OwnerAccountId)
	}

	return res, nil
}

func (blog *BlogService) GetDraftBlogById(ctx context.Context, req *pb.BlogByIdReq) (*pb.BlogByIdRes, error) {
	blog.logger.Debugw("get draft blog", "blog_id", req.BlogId)

	res, err := blog.osClient.GetDraftedBlogByIdAndOwner(ctx, req.BlogId, req.OwnerAccountId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "couldn't found the blog with blogId: %s and ownerAccountId: %s", req.BlogId, req.OwnerAccountId)
	}

	// Check if the response is nil, which indicates no blog was found
	if res == nil {
		return nil, status.Errorf(codes.NotFound, "no blog found with blogId: %s and ownerAccountId: %s", req.BlogId, req.OwnerAccountId)
	}

	return res, nil
}

func (blog *BlogService) GetPublishedBlogByIdAndOwnerId(ctx context.Context, req *pb.BlogByIdReq) (*pb.BlogByIdRes, error) {
	blog.logger.Debugw("get published blog", "blog_id", req.BlogId)

	// Fetch the published blog by blog_id and owner_account_id
	res, err := blog.osClient.GetPublishedBlogByIdAndOwner(ctx, req.BlogId, req.OwnerAccountId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "couldn't fetch the blog with blogId: %s and ownerAccountId: %s", req.BlogId, req.OwnerAccountId)
	}

	// Check if the response is nil, which indicates no blog was found
	if res == nil {
		return nil, status.Errorf(codes.NotFound, "no blog found with blogId: %s and ownerAccountId: %s", req.BlogId, req.OwnerAccountId)
	}

	// Return the found blog
	return res, nil
}

func (blog *BlogService) PublishBlog(ctx context.Context, req *pb.PublishBlogReq) (*pb.PublishBlogResp, error) {
	blog.logger.Infof("The user has requested to publish the blog: %s", req.BlogId)

	// TODO: Check if blog exists and published
	exists, _, err := blog.osClient.DoesBlogExist(ctx, req.BlogId)
	if err != nil {
		blog.logger.Errorf("Error checking blog existence: %v", err)
		return nil, status.Errorf(codes.Internal, "cannot get the blog for id: %s", req.BlogId)
	}

	if !exists {
		blog.logger.Errorf("The blog with ID: %s doesn't exist", req.BlogId)
		return nil, status.Errorf(codes.NotFound, "cannot find the blog for id: %s", req.BlogId)
	}

	_, err = blog.osClient.PublishBlogById(ctx, req.BlogId)
	if err != nil {
		blog.logger.Errorf("Error Publishing the blog: %s, error: %v", req.BlogId, err)
		return nil, status.Errorf(codes.Internal, "cannot find the blog for id: %s", req.BlogId)
	}

	// TODO: Add Tags to the db if not already added

	bx, err := json.Marshal(models.InterServiceMessage{
		AccountId:  req.AccountId,
		BlogId:     req.BlogId,
		Action:     constants.BLOG_PUBLISH,
		BlogStatus: constants.BlogStatusPublished,
		IpAddress:  req.Ip,
		Client:     req.Client,
		Tags:       req.Tags,
	})

	if err != nil {
		blog.logger.Errorf("failed to marshal message for blog publish: user_id=%s, blog_id=%s, error=%v", req.AccountId, req.BlogId, err)
		return nil, status.Errorf(codes.Internal, "published the blog with some error: %s", req.BlogId)
	}

	go func() {
		// Enqueue publish message to user service asynchronously
		err := blog.qConn.PublishMessage(blog.config.RabbitMQ.Exchange, blog.config.RabbitMQ.RoutingKeys[1], bx)
		if err != nil {
			blog.logger.Errorf(`failed to publish blog publish message to RabbitMQ: 
			 exchange=%s, routing_key=%s, error=%v`, blog.config.RabbitMQ.Exchange,
				blog.config.RabbitMQ.RoutingKeys[1], err)
		}

	}()

	go func() {
		// Get the blog slug and do the google search engine optimization
		slug := req.Slug
		if slug == "" {
			blog.logger.Warnf("slug is empty for blog id: %s, generating a new slug", req.BlogId)
			slug = fmt.Sprintf("blog-%s", req.BlogId)
		}

		// A slug looks like: proxmox-virtual-environment-the-practical-guide-for-smart-virtualization-78li3
		// Add https://monkeys.com.co host and append /blog/ with host and then followed by slug
		// The complete slug should look like: https://monkeys.com.co/blog/proxmox-virtual-environment-the-practical-guide-for-smart-virtualization-78li3

		// Call a function to handle SEO asynchronously
		err := blog.seoManager.HandleSEOForBlog(ctx, req.BlogId, slug)
		if err != nil {
			blog.logger.Errorf("failed to handle SEO for blog: user_id=%s, blog_id=%s, error=%v", req.AccountId, req.BlogId, err)
		}

	}()

	return &pb.PublishBlogResp{
		Message: fmt.Sprintf("the blog %s has been published!", req.BlogId),
	}, nil
}

func (blog *BlogService) MoveBlogToDraftStatus(ctx context.Context, req *pb.BlogReq) (*pb.BlogResp, error) {
	blog.logger.Infof("The user has requested to publish the blog: %s", req.BlogId)

	// TODO: Check if blog exists and published
	exists, _, err := blog.osClient.DoesBlogExist(ctx, req.BlogId)
	if err != nil {
		blog.logger.Errorf("Error checking blog existence: %v", err)
		return nil, status.Errorf(codes.Internal, "cannot get the blog for id: %s", req.BlogId)
	}

	if !exists {
		blog.logger.Errorf("The blog with ID: %s doesn't exist", req.BlogId)
		return nil, status.Errorf(codes.NotFound, "cannot find the blog for id: %s", req.BlogId)
	}

	_, err = blog.osClient.MoveBlogToDraft(ctx, req.BlogId)
	if err != nil {
		blog.logger.Errorf("Error Publishing the blog: %s, error: %v", req.BlogId, err)
		return nil, status.Errorf(codes.Internal, "cannot find the blog for id: %s", req.BlogId)
	}

	bx, err := json.Marshal(models.InterServiceMessage{
		AccountId:  req.AccountId,
		BlogId:     req.BlogId,
		Action:     constants.BLOG_UPDATE,
		BlogStatus: constants.BlogStatusDraft,
		IpAddress:  req.Ip,
		Client:     req.Client,
	})

	if err != nil {
		blog.logger.Errorf("failed to marshal message for blog publish: user_id=%s, blog_id=%s, error=%v", req.AccountId, req.BlogId, err)
		return nil, status.Errorf(codes.Internal, "published the blog with some error: %s", req.BlogId)
	}

	// Enqueue publish message to user service asynchronously
	go func() {
		err := blog.qConn.PublishMessage(blog.config.RabbitMQ.Exchange, blog.config.RabbitMQ.RoutingKeys[1], bx)
		if err != nil {
			blog.logger.Errorf("failed to publish blog publish message to RabbitMQ: exchange=%s, routing_key=%s, error=%v", blog.config.RabbitMQ.Exchange, blog.config.RabbitMQ.RoutingKeys[1], err)
		}
	}()

	return &pb.BlogResp{
		Message: fmt.Sprintf("the blog %s has been moved to draft.", req.BlogId),
	}, nil
}

// TODO: Fetch a finite no of blogs like 100 latest blogs based on the tag names
func (blog *BlogService) GetPublishedBlogsByTagsName(ctx context.Context, req *pb.GetBlogsByTagsNameReq) (*pb.GetBlogsByTagsNameRes, error) {
	blog.logger.Infof("fetching blogs with the tags: %s", req.TagNames)

	for i := 0; i < len(req.TagNames); i++ {
		req.TagNames[i] = strings.TrimSpace(req.TagNames[i])
	}

	return blog.osClient.GetPublishedBlogByTagsName(ctx, req.TagNames...)
}

func (blog *BlogService) GetPublishedBlogById(ctx context.Context, req *pb.BlogByIdReq) (*pb.BlogByIdRes, error) {
	blog.logger.Infof("fetching blog with id: %s", req.BlogId)
	return blog.osClient.GetPublishedBlogById(ctx, req.BlogId)
}

func (blog *BlogService) ArchiveBlogById(ctx context.Context, req *pb.ArchiveBlogReq) (*pb.ArchiveBlogResp, error) {
	blog.logger.Infof("Archiving blog %s", req.BlogId)

	exists, _, err := blog.osClient.DoesBlogExist(ctx, req.BlogId)
	if err != nil {
		blog.logger.Errorf("Error checking blog existence: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to check existence for blog with ID: %s", req.BlogId)
	}

	if !exists {
		blog.logger.Errorf("Blog with ID %s does not exist", req.BlogId)
		return nil, status.Errorf(codes.NotFound, "blog with ID %s does not exist", req.BlogId)
	}

	updateResp, err := blog.osClient.AchieveAPublishedBlogById(ctx, req.BlogId)
	if err != nil {
		blog.logger.Errorf("failed to archive the blog with ID: %s, error: %v", req.BlogId, err)
		return nil, status.Errorf(codes.Internal, "failed to archive blog with ID: %s", req.BlogId)
	}

	blog.logger.Infof("Blog with ID: %s archived successfully, status code: %v", req.BlogId, updateResp.StatusCode)
	return &pb.ArchiveBlogResp{
		Message: fmt.Sprintf("Blog %s has been archived!", req.BlogId),
	}, nil
}

func (blog *BlogService) GetLatest100Blogs(ctx context.Context, req *pb.GetBlogsByTagsNameReq) (*pb.GetBlogsByTagsNameRes, error) {
	return blog.osClient.GetLast100BlogsLatestFirst(ctx)
}

// TODO: Incase of blog doesn't exists, do return 404
func (blog *BlogService) DeleteABlogByBlogId(ctx context.Context, req *pb.DeleteBlogReq) (*pb.DeleteBlogResp, error) {
	_, err := blog.osClient.DeleteABlogById(ctx, req.BlogId)
	if err != nil {
		blog.logger.Errorf("failed to delete the blog with ID: %s, error: %v", req.BlogId, err)
		return nil, status.Errorf(codes.Internal, "failed to delete the blog with ID: %s", req.BlogId)
	}

	bx, err := json.Marshal(models.InterServiceMessage{
		AccountId:  req.OwnerAccountId,
		BlogId:     req.BlogId,
		Action:     constants.BLOG_DELETE,
		BlogStatus: constants.BlogDeleted,
		IpAddress:  req.Ip,
		Client:     req.Client,
	})

	if err != nil {
		blog.logger.Errorf("failed to marshal message for blog publish: user_id=%s, blog_id=%s, error=%v", req.OwnerAccountId, req.BlogId, err)
		return nil, status.Errorf(codes.Internal, "published the blog with some error: %s", req.BlogId)
	}

	// Enqueue delete message to user service asynchronously
	go func() {
		err := blog.qConn.PublishMessage(blog.config.RabbitMQ.Exchange, blog.config.RabbitMQ.RoutingKeys[1], bx)
		if err != nil {
			blog.logger.Errorf("failed to publish blog publish message to RabbitMQ: exchange=%s, routing_key=%s, error=%v", blog.config.RabbitMQ.Exchange, blog.config.RabbitMQ.RoutingKeys[1], err)
		}
	}()

	// Enqueue delete message to storage service asynchronously
	go func() {
		err := blog.qConn.PublishMessage(blog.config.RabbitMQ.Exchange, blog.config.RabbitMQ.RoutingKeys[2], bx)
		if err != nil {
			blog.logger.Errorf("failed to publish blog publish message to RabbitMQ: exchange=%s, routing_key=%s, error=%v", blog.config.RabbitMQ.Exchange, blog.config.RabbitMQ.RoutingKeys[2], err)
		}
	}()

	// fmt.Printf("resp.StatusCode: %v\n", resp.StatusCode)
	return &pb.DeleteBlogResp{
		Message: fmt.Sprintf("Blog with id %s has been successfully deleted", req.BlogId),
	}, nil
}

func (blog *BlogService) GetDraftBlogByBlogId(ctx context.Context, req *pb.BlogByIdReq) (*pb.BlogByIdRes, error) {
	blog.logger.Infof("fetching blog with id: %s", req.BlogId)
	return blog.osClient.GetDraftBlogByBlogId(ctx, req.BlogId)
}

func (blog *BlogService) GetAllBlogsByBlogIds(ctc context.Context, req *pb.GetBlogsByBlogIds) (*pb.GetBlogsRes, error) {
	if len(req.BlogIds) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "blog ids cannot be empty")
	}

	return blog.osClient.GetBlogsByBlogIds(ctc, req.BlogIds)
}
