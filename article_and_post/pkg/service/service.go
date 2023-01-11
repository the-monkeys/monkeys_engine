package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/89minutes/the_new_project/article_and_post/pkg/database"
	"github.com/89minutes/the_new_project/article_and_post/pkg/models"
	"github.com/89minutes/the_new_project/article_and_post/pkg/pb"
	"github.com/89minutes/the_new_project/article_and_post/pkg/utils"
	"github.com/google/uuid"
	"github.com/opensearch-project/opensearch-go"
	"github.com/opensearch-project/opensearch-go/opensearchapi"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ArticleServer struct {
	osClient *opensearch.Client
	pb.UnimplementedArticleServiceServer
}

func NewArticleServer(url, username, password string) (*ArticleServer, error) {
	client, err := database.NewOSClient(url, username, password)
	if err != nil {
		logrus.Errorf("Failed to connect to opensearch instance, error: %+v", err)
		return nil, err
	}

	return &ArticleServer{
		osClient: client,
	}, nil
}

func (srv *ArticleServer) CreateArticle(ctx context.Context, req *pb.CreateArticleRequest) (*pb.CreateArticleResponse, error) {
	var article models.Article

	if req.GetId() == "" {
		req.Id = uuid.New().String()
	}

	req.CanEdit = true
	req.ContentOwnerShip = pb.CreateArticleRequest_THE_USER

	// Store into the opensearch db
	document := strings.NewReader(ArticleToString(req))

	osReq := opensearchapi.IndexRequest{
		Index:      utils.OpensearchArticleIndex,
		DocumentID: req.Id,
		Body:       document,
	}

	insertResponse, err := osReq.Do(context.Background(), srv.osClient)
	if err != nil {
		logrus.Errorf("cannot create a new document for user: %s, error: %v", req.GetAuthor(), err)
		return nil, err
	}

	if insertResponse.IsError() {
		logrus.Errorf("opensearch apt failed to create a new document for user: %s, error: %v",
			req.GetAuthor(), insertResponse.Status())
		return nil, err
	}

	logrus.Infof("successfully created an article for user: %s, insert response: %+v",
		req.Author, insertResponse)

	return &pb.CreateArticleResponse{
		Status: http.StatusCreated,
		Id:     article.Id,
	}, nil
}

func ArticleToString(ip *pb.CreateArticleRequest) string {
	return fmt.Sprintf(`{
		"id":         			"%s",
		"title":      			"%s",
		"content":     			"%s",
		"author":   			"%s",
		"is_draft":    			"%v",
		"tags": 				"%v",
		"create_time": 			"%v",
		"update_time": 			"%v",
		"quick_read": 			"%v",
		"content_ownership": 	"%v",
		"can_edit": 			"%v",
		"viewed_by":			"%v",
		"comments":				"%v"
	}`, ip.Id, ip.Title, ip.Content, ip.Author, ip.IsDraft,
		ip.Tags, ip.CreateTime, ip.UpdateTime, ip.QuickRead, ip.ContentOwnerShip,
		ip.CanEdit, ip.ViewBy, ip.Comment)
}

func (srv *ArticleServer) GetArticles(req *pb.GetArticlesRequest, stream pb.ArticleService_GetArticlesServer) error {

	// Search for the document.
	content := strings.NewReader(`{
		"query": {
			"match": {
				"is_draft": "false"
			}
		},
		"_source": {
			"includes": [
				"id",
				"title",
				"author",
				"create_time",
				"quick_read",
				"viewed_by"
			],
			"excludes": [
				"content"
			]
		}
	}`)

	search := opensearchapi.SearchRequest{
		Index: []string{utils.OpensearchArticleIndex},
		Body:  content,
	}

	searchResponse, err := search.Do(context.Background(), srv.osClient)
	if err != nil {
		fmt.Println("failed to search document ", err)
		os.Exit(1)
	}

	var result map[string]interface{}

	decoder := json.NewDecoder(searchResponse.Body)
	if err := decoder.Decode(&result); err != nil {
		logrus.Error("Error while decoding, error", err)
	}

	bx, err := json.MarshalIndent(result, "", "    ")
	if err != nil {
		logrus.Errorf("cannot marshal map[string]interface{}, error: %+v", err)
		return err
	}

	arts := models.ArticlesForTheMainPage{}
	if err := json.Unmarshal(bx, &arts); err != nil {
		logrus.Errorf("cannot unmarshal byte slice, error: %+v", err)
		return err
	}

	articles := ParseToStruct(arts)
	for _, article := range articles {
		log.Printf("Article %+v", article)

		if err := stream.Send(&article); err != nil {
			logrus.Errorf("error while sending stream, error %+v", err)
		}
	}

	return nil
}

func ParseToStruct(result models.ArticlesForTheMainPage) []pb.GetArticlesResponse {
	var resp []pb.GetArticlesResponse

	for _, val := range result.Hits.Hits {
		qRead := false
		if val.Source.QuickRead == "true" {
			qRead = true
		}

		tStamp, err := SplitSecondsAndNanos(val.Source.CreateTime)
		if err != nil {
			logrus.Errorf("cannot parse string timestamp to timestamp, error %v", err)
		}

		res := pb.GetArticlesResponse{
			Id:         val.Source.ID,
			Title:      val.Source.Title,
			Author:     val.Source.Author,
			CreateTime: &tStamp,
			QuickRead:  qRead,
			// ViewBy:    instance.ViewedBy,
		}
		resp = append(resp, res)
	}

	return resp
}

func SplitSecondsAndNanos(tStamp string) (timestamppb.Timestamp, error) {
	secAndNano := strings.Split(tStamp, " ")
	first := strings.Split(secAndNano[0], ":")
	second := strings.Split(secAndNano[1], ":")

	seconds, err := strconv.ParseInt(first[1], 10, 64)
	if err != nil {
		return timestamppb.Timestamp{}, err
	}

	nanos, err := strconv.ParseInt(second[1], 10, 64)
	if err != nil {
		return timestamppb.Timestamp{}, err

	}

	return timestamppb.Timestamp{
		Seconds: seconds,
		Nanos:   int32(nanos),
	}, nil
}
