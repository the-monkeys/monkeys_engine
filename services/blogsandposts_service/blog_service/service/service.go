package service

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/89minutes/the_new_project/common"
	"github.com/89minutes/the_new_project/services/blogsandposts_service/blog_service/models"
	"github.com/89minutes/the_new_project/services/blogsandposts_service/blog_service/pb"
	"github.com/89minutes/the_new_project/services/blogsandposts_service/blog_service/utils"
	"github.com/opensearch-project/opensearch-go/opensearchapi"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TODO: The following
// Get blog by id is throwing error if the blog doesn't exist

type BlogService struct {
	osClient openSearchClient
	logger   *logrus.Logger
	pb.UnimplementedBlogsAndPostServiceServer
}

func NewBlogService(client openSearchClient,
	logger *logrus.Logger) *BlogService {
	return &BlogService{osClient: client, logger: logger}
}

func (blog *BlogService) CreateABlog(ctx context.Context, req *pb.CreateBlogRequest) (*pb.CreateBlogResponse, error) {
	blog.logger.Infof("received a create blog request from user: %v", req.AuthorId)

	// Lower cased tags and trim spaces
	for i, v := range req.Tags {
		req.Tags[i] = strings.ToLower(strings.TrimSpace(v))
	}

	// Trim spaces from fields
	req.Title = strings.TrimSpace(req.Title)
	req.AuthorName = strings.TrimSpace(req.AuthorName)
	req.Content = strings.TrimSpace(req.Content)
	req.AuthorId = strings.TrimSpace(req.AuthorId)

	req.CanEdit = true
	req.Ownership = pb.CreateBlogRequest_THE_USER

	// Assign to models struct
	post := models.Blogs{
		Id:               req.Id,
		Title:            req.Title,
		ContentFormatted: req.Content,
		ContentRaw:       formattedToRawContent(req.Content),
		AuthorName:       req.AuthorName,
		AuthorId:         req.AuthorId,
		Published:        &req.Published,
		Tags:             req.Tags,
		CreateTime:       time.Now().Format("2006-01-02T15:04:05Z07:00"),
		UpdateTime:       time.Now().Format("2006-01-02T15:04:05Z07:00"),
		CanEdit:          &req.CanEdit,
		OwnerShip:        req.Ownership,
		FolderPath:       "",
	}

	// Create the articles
	resp, err := blog.osClient.CreateAnArticle(post)
	if err != nil {
		blog.logger.Errorf("cannot save the blog, error: %+v", err)
		return nil, err
	}

	if resp.StatusCode == http.StatusBadRequest {
		blog.logger.Errorf("cannot save the blog bad request, error: %+v", err)
		return nil, common.BadRequest
	}

	blog.logger.Infof("user %v created a blog successfully: %v", req.GetAuthorId(), req.GetId())

	return &pb.CreateBlogResponse{
		Status: int64(resp.StatusCode),
		Id:     int64(resp.StatusCode),
	}, nil
}

func (blog *BlogService) Get100Blogs(req *emptypb.Empty, stream pb.BlogsAndPostService_Get100BlogsServer) error {
	searchResponse, err := blog.osClient.GetLast100Articles()
	var result map[string]interface{}

	// logrus.Infof("Response: %+v", searchResponse)
	decoder := json.NewDecoder(searchResponse.Body)
	if err := decoder.Decode(&result); err != nil {
		blog.logger.Error("error while decoding, error", err)
	}

	bx, err := json.MarshalIndent(result, "", "    ")
	if err != nil {
		blog.logger.Errorf("cannot marshal map[string]interface{}, error: %+v", err)
		return err
	}

	arts := models.Last100Articles{}
	if err := json.Unmarshal(bx, &arts); err != nil {
		blog.logger.Errorf("cannot unmarshal byte slice, error: %+v", err)
		return err
	}

	articles := parseToStruct(arts)
	for _, article := range articles {
		if err := stream.Send(&article); err != nil {
			blog.logger.Errorf("error while sending stream, error %+v", err)
		}
	}

	return nil
}

func (blog *BlogService) GetBlogById(ctx context.Context, req *pb.GetBlogByIdRequest) (*pb.GetBlogByIdResponse, error) {
	blog.logger.Infof("the blog %v has been requested", req.GetId())

	searchResponse, err := blog.osClient.GetArticleById(ctx, req.GetId())
	if err != nil {
		blog.logger.Errorf("failed to find document, error: %+v", err)
		return nil, status.Errorf(codes.Internal, "failed to find the document, error: %v", err)
	}

	if searchResponse.IsError() {
		blog.logger.Errorf("error fetching the article, %v, search response: %+v", req.Id, searchResponse)
		return nil, err
	}

	var result map[string]interface{}

	// logrus.Infof("Response: %+v", searchResponse)

	decoder := json.NewDecoder(searchResponse.Body)
	if err := decoder.Decode(&result); err != nil {
		blog.logger.Error("error while decoding result, error", err)
		return nil, status.Errorf(codes.Internal, "cannot decode opensearch response: %v", err)
	}

	bx, err := json.MarshalIndent(result, "", "    ")
	if err != nil {
		blog.logger.Errorf("cannot marshal map[string]interface{}, error: %+v", err)
		return nil, status.Errorf(codes.Internal, "cannot marshal opensearch response: %v", err)
	}

	art := models.GetArticleById{}
	if err := json.Unmarshal(bx, &art); err != nil {
		blog.logger.Errorf("cannot unmarshal byte slice, error: %+v", err)
		return nil, status.Errorf(codes.Internal, "cannot unmarshal opensearch response: %v", err)
	}

	// if len(art.Hits.Hits) == 0 {
	// 	blog.logger.Errorf("cannot find the blog : %v", req.GetId())
	// 	return nil, status.Errorf(codes.NotFound, "cannot find the document")
	// }

	return &pb.GetBlogByIdResponse{
		Id:         art.Hits.Hits[0].Source.ID,
		Title:      art.Hits.Hits[0].Source.Title,
		AuthorName: art.Hits.Hits[0].Source.AuthorName,
		AuthorId:   art.Hits.Hits[0].Source.AuthorID,
		Content:    art.Hits.Hits[0].Source.ContentFormatted,
		CreateTime: timestamppb.New(art.Hits.Hits[0].Source.CreateTime),
		Tags:       art.Hits.Hits[0].Source.Tags,
	}, nil
}

func (blog *BlogService) EditBlogById(ctx context.Context, req *pb.EditBlogRequest) (*pb.EditBlogResponse, error) {
	blog.logger.Info("the user has requested to edit the post %v", req.GetId())

	// Lower cased tags and trim spaces
	for i, v := range req.Tags {
		req.Tags[i] = strings.ToLower(strings.TrimSpace(v))
	}

	// Trim spaces from fields
	req.Title = strings.TrimSpace(req.Title)
	req.Content = strings.TrimSpace(req.Content)

	// Get the document from opensearch
	existingArticle, err := blog.GetBlogById(ctx, &pb.GetBlogByIdRequest{Id: req.GetId()})
	if err != nil {
		blog.logger.Errorf("cannot get the existing article, error: %+v", err)
		return nil, status.Errorf(codes.Internal, "cannot get the existing article, error: %v", err)
	}

	// Check if partial then fill a new struct
	toBeUpdated := partialOrAllUpdate(req.IsPartial, existingArticle, req)
	logrus.Infof("Article to be updated: %+v", toBeUpdated.Id)

	document := strings.NewReader(updateArticleById(toBeUpdated.Id, toBeUpdated.Title, toBeUpdated.Content, toBeUpdated.Tags))

	updateReq := opensearchapi.UpdateByQueryRequest{
		Index: []string{utils.OpensearchArticleIndex},
		Body:  document,
	}

	updateRes, err := updateReq.Do(ctx, blog.osClient.client)
	if err != nil {
		blog.logger.Errorf("failed to update the document, error: %+v", err)
		return nil, status.Errorf(codes.Internal, "cannot update the document, error: %v", err)
	}

	if updateRes.IsError() {
		blog.logger.Errorf("cannot update the document, error: %+v", updateRes)
		return nil, status.Errorf(codes.Internal, "cannot update the document, error: %v", err)
	}

	if updateRes.StatusCode == http.StatusBadRequest {
		blog.logger.Errorf("cannot update the document, bad request, error: %+v", updateRes)
		return nil, status.Errorf(codes.Internal, "cannot update the document, error: %v", err)
	}

	logrus.Infof("Updated the article %s", req.Id)

	if updateRes.IsError() {
		blog.logger.Errorf("failed to update the document, bad request, error: %+v", err)
		return nil, status.Errorf(codes.InvalidArgument, "cannot update the document, error: %v", err)
	}

	return &pb.EditBlogResponse{
		Status: http.StatusCreated,
		Id:     toBeUpdated.Id,
	}, nil
}

func (blog *BlogService) DeleteBlogById(ctx context.Context, req *pb.DeleteBlogByIdRequest) (*pb.DeleteBlogByIdResponse, error) {
	blog.logger.Infof("user has requested to delete blog %v", req.Id)

	delete := opensearchapi.DeleteRequest{
		Index:      utils.OpensearchArticleIndex,
		DocumentID: req.Id,
	}

	deleteResponse, err := delete.Do(context.Background(), blog.osClient.client)
	if err != nil {
		blog.logger.Error("cannot delete the blog %s, error: %v", req.Id, err)
		return nil, err
	}

	if deleteResponse.StatusCode == http.StatusNotFound {
		blog.logger.Error("cannot find the blog %s, error: %v", req.Id, err)
		return nil, err
	}

	return &pb.DeleteBlogByIdResponse{Status: int64(deleteResponse.StatusCode)}, nil
}
