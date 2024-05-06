package database

import (
	"context"
	"encoding/json"
	"fmt"
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
	os.log.Infof("DraftABlog: received an article with id: %s", blog.ID)

	bs, err := json.Marshal(blog)
	if err != nil {
		os.log.Errorf("DraftABlog: cannot marshal the article, error: %v", err)
		return nil, err
	}

	document := strings.NewReader(string(bs))

	osReq := opensearchapi.IndexRequest{
		Index:      constants.OpensearchArticleIndex,
		DocumentID: blog.ID,
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
			os.log.Infof("Blog with id: %s does not exist", blogID)
			return false, nil
		}
		err = fmt.Errorf("error checking if blog exists, get response: %+v", getResponse)
		os.log.Error(err)
		return false, err
	}

	os.log.Infof("Blog with id: %s exists", blogID)
	return true, nil
}
