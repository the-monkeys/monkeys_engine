package database

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/opensearch-project/opensearch-go"
	"github.com/opensearch-project/opensearch-go/opensearchapi"
	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_blog/pb"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_blog/internal/constants"
)

type OpensearchStorage interface {
	DraftABlog(ctx context.Context, blog *pb.DraftBlogRequest) (*opensearchapi.Response, error)
	DoesBlogExist(ctx context.Context, blogID string) (bool, error)
	PublishBlogById(ctx context.Context, blogId string) (*opensearchapi.Response, error)
	GetBlogById(ctx context.Context, req *pb.GetBlogByIdReq) (*pb.GetBlogByIdRes, error)
}

type opensearchStorage struct {
	client *opensearch.Client
	log    *logrus.Logger
}

func NewOpenSearchClient(url, username, password string, log *logrus.Logger) (OpensearchStorage, error) {
	client, err := NewOSClient(url, username, password)
	if err != nil {
		logrus.Errorf("Failed to connect to opensearch instance, error: %+v", err)
		return nil, err
	}

	return &opensearchStorage{
		client: client,
		log:    log,
	}, nil
}

func (os *opensearchStorage) DraftABlog(ctx context.Context, blog *pb.DraftBlogRequest) (*opensearchapi.Response, error) {
	os.log.Infof("DraftABlog: received an article with id: %s", blog.BlogId)

	bs, err := json.Marshal(blog)
	if err != nil {
		os.log.Errorf("DraftABlog: cannot marshal the article, error: %v", err)
		return nil, err
	}

	document := strings.NewReader(string(bs))

	osReq := opensearchapi.IndexRequest{
		Index:      constants.OpensearchArticleIndex,
		DocumentID: blog.BlogId,
		Body:       document,
	}

	insertResponse, err := osReq.Do(ctx, os.client)
	if err != nil {
		os.log.Errorf("DraftABlog: error while creating/drafting article, error: %+v", err)
		return insertResponse, err
	}

	if insertResponse.IsError() {
		err = fmt.Errorf("DraftABlog: error creating an article, insert response: %+v", insertResponse)
		os.log.Error(err)
		return insertResponse, err
	}

	os.log.Infof("DraftABlog: successfully created an article for user: %s, insert response: %+v", blog.OwnerAccountId, insertResponse)
	return insertResponse, nil
}

func (os *opensearchStorage) DoesBlogExist(ctx context.Context, blogID string) (bool, error) {
	os.log.Infof("Checking if a blog with id: %s exists", blogID)

	osReq := opensearchapi.GetRequest{
		Index:      constants.OpensearchArticleIndex,
		DocumentID: blogID,
	}

	getResponse, err := osReq.Do(ctx, os.client)
	if err != nil {
		os.log.Errorf("Error while checking if blog exists, error: %+v", err)
		return false, err
	}

	if getResponse.IsError() {
		if getResponse.StatusCode == http.StatusNotFound {
			os.log.Errorf("Blog with id: %s does not exist", blogID)
			return false, fmt.Errorf("blog with id: %s does not exist", blogID)
		}
		err = fmt.Errorf("error checking if blog exists, get response: %+v", getResponse)
		os.log.Error(err)
		return false, err
	}

	os.log.Infof("Blog with id: %s exists", blogID)
	return true, nil
}
func (os *opensearchStorage) PublishBlogById(ctx context.Context, blogId string) (*opensearchapi.Response, error) {
	os.log.Infof("Publishing blog with id: %s", blogId)

	// Define the update script
	updateScript := `{
		"script" : {
			"source": "ctx._source.is_draft = params.is_draft",
			"lang": "painless",
			"params" : {
				"is_draft" : false
			}
		}
	}`

	osReq := opensearchapi.UpdateRequest{
		Index:      constants.OpensearchArticleIndex,
		DocumentID: blogId,
		Body:       strings.NewReader(updateScript),
	}

	updateResponse, err := osReq.Do(ctx, os.client)
	if err != nil {
		os.log.Errorf("Error while publishing blog, error: %+v", err)
		return updateResponse, err
	}

	if updateResponse.IsError() {
		err = fmt.Errorf("error publishing blog, update response: %+v", updateResponse)
		os.log.Error(err)
		return updateResponse, err
	}

	os.log.Infof("Successfully published blog with id: %s", blogId)
	return updateResponse, nil
}

func (storage *opensearchStorage) GetBlogById(ctx context.Context, req *pb.GetBlogByIdReq) (*pb.GetBlogByIdRes, error) {
	storage.log.Infof("fetching blog with id: %s", req.BlogId)

	osReq := opensearchapi.GetRequest{
		Index:      constants.OpensearchArticleIndex,
		DocumentID: req.BlogId,
	}

	getResponse, err := osReq.Do(ctx, storage.client)
	if err != nil {
		storage.log.Errorf("error while fetching blog, error: %+v", err)
		return nil, err
	}

	if getResponse.IsError() {
		if getResponse.StatusCode == http.StatusNotFound {
			storage.log.Errorf("blog with id: %s does not exist", req.BlogId)
			return nil, fmt.Errorf("blog with id: %s does not exist", req.BlogId)
		}
		err = fmt.Errorf("error fetching blog, get response: %+v", getResponse)
		storage.log.Error(err)
		return nil, err
	}

	// Read the body into a byte slice
	bodyBytes, err := io.ReadAll(getResponse.Body)
	if err != nil {
		storage.log.Errorf("error reading response body, error: %+v", err)
		return nil, err
	}

	var source map[string]interface{}
	err = json.Unmarshal(bodyBytes, &source)
	if err != nil {
		storage.log.Errorf("error unmarshalling blog, error: %+v", err)
		return nil, err
	}

	bx, err := json.MarshalIndent(source["_source"].(map[string]interface{}), "", "\t")
	if err != nil {
		storage.log.Errorf("error marshalling the _source, error: %+v", err)
		return nil, err
	}
	blogRes := &pb.GetBlogByIdRes{}

	if err = json.Unmarshal(bx, blogRes); err != nil {
		storage.log.Errorf("error un-marshalling the bytes into struct, error: %+v", err)
		return nil, err
	}

	storage.log.Infof("successfully fetched blog with id: %s", req.BlogId)
	return blogRes, nil
}
