package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_blog/pb"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/constants"
	"github.com/the-monkeys/the_monkeys/microservices/rabbitmq"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_blog/internal/database"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_blog/internal/models"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type BlogService struct {
	osClient database.ElasticsearchStorage
	logger   *logrus.Logger
	config   *config.Config
	qConn    rabbitmq.Conn
	pb.UnimplementedBlogServiceServer
}

func NewBlogService(client database.ElasticsearchStorage, logger *logrus.Logger, config *config.Config, qConn rabbitmq.Conn) *BlogService {
	return &BlogService{
		osClient: client,
		logger:   logger,
		config:   config,
		qConn:    qConn,
	}
}

func (blog *BlogService) DraftBlog(ctx context.Context, req *pb.DraftBlogRequest) (*pb.BlogResponse, error) {
	blog.logger.Infof("Content: %+v", req)
	blog.logger.Infof("received a blog containing id: %s", req.BlogId)
	req.IsDraft = true

	exists, _ := blog.osClient.DoesBlogExist(ctx, req.BlogId)
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
		bx, err := json.Marshal(models.MessageToUserSvc{
			UserAccountId: req.OwnerAccountId,
			BlogId:        req.BlogId,
			Action:        constants.BLOG_CREATE,
			Status:        constants.BlogStatusDraft,
		})
		if err != nil {
			blog.logger.Errorf("cannot marshal the message for blog: %s, error: %v", req.BlogId, err)
			return nil, status.Errorf(codes.Internal, "Something went wrong while drafting a blog")
		}
		if len(req.Tags) == 0 {
			req.Tags = []string{"untagged"}
		}
		go blog.qConn.PublishMessage(blog.config.RabbitMQ.Exchange, blog.config.RabbitMQ.RoutingKeys[1], bx)
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

func (blog *BlogService) GetDraftBlogs(ctx context.Context, req *pb.GetDraftBlogsReq) (*pb.GetDraftBlogsRes, error) {
	blog.logger.Infof("fetching draft blogs for account id %s", req.AccountId)
	if req.AccountId == "" {
		logrus.Error("account id cannot be empty")
		return nil, status.Errorf(codes.InvalidArgument, "Account id cannot be empty")
	}

	res, err := blog.osClient.GetDraftBlogsByOwnerAccountID(ctx, req.AccountId)
	if err != nil {
		logrus.Errorf("error occurred while getting draft blogs for account id: %s, error: %v", req.AccountId, err)
		return nil, status.Errorf(codes.Internal, "cannot get the draft blogs for account id: %s", req.AccountId)
	}

	return res, nil
}

func (blog *BlogService) GetDraftBlogById(ctx context.Context, req *pb.GetBlogByIdReq) (*pb.GetBlogByIdRes, error) {
	blog.logger.Infof("fetching blog with id: %s", req.BlogId)
	return blog.osClient.GetDraftedBlogByIdAndOwner(ctx, req.BlogId, req.OwnerAccountId)
}

func (blog *BlogService) GetPublishedBlogByIdAndOwnerId(ctx context.Context, req *pb.GetBlogByIdReq) (*pb.GetBlogByIdRes, error) {
	blog.logger.Infof("fetching blog with id: %s", req.BlogId)
	return blog.osClient.GetPublishedBlogByIdAndOwner(ctx, req.BlogId, req.OwnerAccountId)
}

func (blog *BlogService) PublishBlog(ctx context.Context, req *pb.PublishBlogReq) (*pb.PublishBlogResp, error) {
	blog.logger.Infof("The user has requested to publish the blog: %s", req.BlogId)

	exists, err := blog.osClient.DoesBlogExist(ctx, req.BlogId)
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

	return &pb.PublishBlogResp{
		Message: fmt.Sprintf("the blog %s has been published!", req.BlogId),
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

func (blog *BlogService) GetPublishedBlogById(ctx context.Context, req *pb.GetBlogByIdReq) (*pb.GetBlogByIdRes, error) {
	blog.logger.Infof("fetching blog with id: %s", req.BlogId)
	return blog.osClient.GetPublishedBlogById(ctx, req.BlogId)
}

func (blog *BlogService) ArchiveBlogById(ctx context.Context, req *pb.ArchiveBlogReq) (*pb.ArchiveBlogResp, error) {
	blog.logger.Infof("Archiving blog %s", req.BlogId)

	exists, err := blog.osClient.DoesBlogExist(ctx, req.BlogId)
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
		blog.logger.Errorf("failed to archive blog with ID: %s, error: %v", req.BlogId, err)
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

// ********************************************************  Below function need to be re-written ********************************************************

// func (blog *BlogService) Get100Blogs(req *emptypb.Empty, stream pb.BlogsAndPostService_Get100BlogsServer) error {
// 	searchResponse, err := blog.osClient.GetLast100Articles()
// 	if err != nil {
// 		blog.logger.Errorf("cannot get the blogs, error: %v", err)
// 		return err
// 	}
// 	var result map[string]interface{}

// 	// logrus.Infof("Response: %+v", searchResponse)
// 	decoder := json.NewDecoder(searchResponse.Body)
// 	if err := decoder.Decode(&result); err != nil {
// 		blog.logger.Error("error while decoding, error", err)
// 	}

// 	bx, err := json.MarshalIndent(result, "", "    ")
// 	if err != nil {
// 		blog.logger.Errorf("cannot marshal map[string]interface{}, error: %+v", err)
// 		return err
// 	}

// 	arts := models.Last100Articles{}
// 	if err := json.Unmarshal(bx, &arts); err != nil {
// 		blog.logger.Errorf("cannot unmarshal byte slice, error: %+v", err)
// 		return err
// 	}

// 	articles := parseToStruct(arts)
// 	for _, article := range articles {
// 		if err := stream.Send(&article); err != nil {
// 			blog.logger.Errorf("error while sending stream, error %+v", err)
// 		}
// 	}

// 	return nil
// }

// func (blog *BlogService) GetBlogById(ctx context.Context, req *pb.GetBlogByIdRequest) (*pb.GetBlogByIdResponse, error) {
// 	blog.logger.Infof("the blog %v has been requested", req.GetId())

// 	searchResponse, err := blog.osClient.GetArticleById(ctx, req.GetId())
// 	if err != nil {
// 		blog.logger.Errorf("failed to find document, error: %+v", err)
// 		return nil, status.Errorf(codes.Internal, "failed to find the document, error: %v", err)
// 	}

// 	if searchResponse.IsError() {
// 		blog.logger.Errorf("error fetching the article, %v, search response: %+v", req.Id, searchResponse)
// 		return nil, err
// 	}

// 	var result map[string]interface{}

// 	// logrus.Infof("Response: %+v", searchResponse)

// 	decoder := json.NewDecoder(searchResponse.Body)
// 	if err := decoder.Decode(&result); err != nil {
// 		blog.logger.Error("error while decoding result, error", err)
// 		return nil, status.Errorf(codes.Internal, "cannot decode opensearch response: %v", err)
// 	}

// 	bx, err := json.MarshalIndent(result, "", "    ")
// 	if err != nil {
// 		blog.logger.Errorf("cannot marshal map[string]interface{}, error: %+v", err)
// 		return nil, status.Errorf(codes.Internal, "cannot marshal opensearch response: %v", err)
// 	}

// 	art := models.GetArticleById{}
// 	if err := json.Unmarshal(bx, &art); err != nil {
// 		blog.logger.Errorf("cannot unmarshal byte slice, error: %+v", err)
// 		return nil, status.Errorf(codes.Internal, "cannot unmarshal opensearch response: %v", err)
// 	}

// 	if len(art.Hits.Hits) == 0 {
// 		blog.logger.Errorf("cannot find the blog : %v", req.GetId())
// 		return nil, status.Errorf(codes.NotFound, "cannot find the document")
// 	}

// 	return &pb.GetBlogByIdResponse{
// 		Id:         art.Hits.Hits[0].Source.ID,
// 		Title:      art.Hits.Hits[0].Source.Title,
// 		AuthorName: art.Hits.Hits[0].Source.AuthorName,
// 		AuthorId:   art.Hits.Hits[0].Source.AuthorID,
// 		Content:    art.Hits.Hits[0].Source.ContentFormatted,
// 		CreateTime: timestamppb.New(art.Hits.Hits[0].Source.CreateTime),
// 		Tags:       art.Hits.Hits[0].Source.Tags,
// 	}, nil
// }

// func (blog *BlogService) EditBlogById(ctx context.Context, req *pb.EditBlogRequest) (*pb.EditBlogResponse, error) {
// 	blog.logger.Infof("the user has requested to edit the post %v", req.GetId())

// 	// Lower cased tags and trim spaces
// 	for i, v := range req.Tags {
// 		req.Tags[i] = strings.ToLower(strings.TrimSpace(v))
// 	}

// 	// Trim spaces from fields
// 	req.Title = strings.TrimSpace(req.Title)
// 	req.Content = strings.TrimSpace(req.Content)

// 	// Get the document from opensearch
// 	existingArticle, err := blog.GetBlogById(ctx, &pb.GetBlogByIdRequest{Id: req.GetId()})
// 	if err != nil {
// 		blog.logger.Errorf("cannot get the existing article, error: %+v", err)
// 		return nil, status.Errorf(codes.Internal, "cannot get the existing article, error: %v", err)
// 	}

// 	// Check if partial then fill a new struct
// 	toBeUpdated := partialOrAllUpdate(req.IsPartial, existingArticle, req)
// 	logrus.Infof("Article to be updated: %+v", toBeUpdated.Id)

// 	document := strings.NewReader(updateArticleById(toBeUpdated.Id, toBeUpdated.Title, toBeUpdated.Content, toBeUpdated.Tags))

// 	updateReq := opensearchapi.UpdateByQueryRequest{
// 		Index: []string{utils.OpensearchArticleIndex},
// 		Body:  document,
// 	}

// 	updateRes, err := updateReq.Do(ctx, blog.osClient.client)
// 	if err != nil {
// 		blog.logger.Errorf("failed to update the document, error: %+v", err)
// 		return nil, status.Errorf(codes.Internal, "cannot update the document, error: %v", err)
// 	}

// 	if updateRes.IsError() {
// 		blog.logger.Errorf("cannot update the document, error: %+v", updateRes)
// 		return nil, status.Errorf(codes.Internal, "cannot update the document, error: %v", err)
// 	}

// 	if updateRes.StatusCode == http.StatusBadRequest {
// 		blog.logger.Errorf("cannot update the document, bad request, error: %+v", updateRes)
// 		return nil, status.Errorf(codes.Internal, "cannot update the document, error: %v", err)
// 	}

// 	logrus.Infof("Updated the article %s", req.Id)

// 	if updateRes.IsError() {
// 		blog.logger.Errorf("failed to update the document, bad request, error: %+v", err)
// 		return nil, status.Errorf(codes.InvalidArgument, "cannot update the document, error: %v", err)
// 	}

// 	return &pb.EditBlogResponse{
// 		Status: http.StatusCreated,
// 		Id:     toBeUpdated.Id,
// 	}, nil
// }

// func (blog *BlogService) DeleteBlogById(ctx context.Context, req *pb.DeleteBlogByIdRequest) (*pb.DeleteBlogByIdResponse, error) {
// 	blog.logger.Infof("user has requested to delete blog %v", req.Id)

// 	delete := opensearchapi.DeleteRequest{
// 		Index:      utils.OpensearchArticleIndex,
// 		DocumentID: req.Id,
// 	}

// 	deleteResponse, err := delete.Do(context.Background(), blog.osClient.client)
// 	if err != nil {
// 		blog.logger.Errorf("cannot delete the blog %s, error: %v", req.Id, err)
// 		return nil, err
// 	}

// 	if deleteResponse.StatusCode == http.StatusNotFound {
// 		blog.logger.Errorf("cannot find the blog %s, error: %v", req.Id, err)
// 		return nil, err
// 	}

// 	return &pb.DeleteBlogByIdResponse{Status: int64(deleteResponse.StatusCode)}, nil
// }
