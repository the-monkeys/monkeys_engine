package database

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	// GetBlogById(ctx context.Context, req *pb.GetBlogByIdReq) (*pb.GetBlogByIdRes, error)
	GetBlogDetailsById(ctx context.Context, blogId string) (string, []string, error)
	ArchieveBlogById(ctx context.Context, blogId string) (*opensearchapi.Response, error)
	GetPublishedBlogById(ctx context.Context, id string) (*pb.GetBlogByIdRes, error)
	GetPublishedBlogByTagsName(ctx context.Context, id ...string) (*pb.GetBlogsByTagsNameRes, error)
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

func (storage *opensearchStorage) GetPublishedBlogById(ctx context.Context, id string) (*pb.GetBlogByIdRes, error) {
	res, err := storage.client.Search(
		storage.client.Search.WithContext(context.Background()),
		storage.client.Search.WithIndex(constants.OpensearchArticleIndex),
		storage.client.Search.WithBody(strings.NewReader(fmt.Sprintf(`{
			"query": {
				"bool": {
					"must": [
						{ "term": { "blog_id": "%s" } },
						{ "term": { "is_draft": false } }
					],
					"should": [
						{ "bool": { "must_not": { "exists": { "field": "is_archive" } } } },
						{ "term": { "is_archive": false } }
					],
					"minimum_should_match": 1
				}
			}
		}`, id))),
		storage.client.Search.WithPretty(),
	)

	// storage.log.Infof("Response: %+v", res)
	if err != nil {
		log.Fatalf("fetching the blog: %s", err)
		return nil, err
	}
	defer res.Body.Close()

	bodyBytes, err := io.ReadAll(res.Body)
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

	if len(source["hits"].(map[string]interface{})["hits"].([]interface{})) == 0 {
		storage.log.Errorf("no blog found with id: %s", id)
		return nil, fmt.Errorf("no blog found with id: %s", id)
	}

	firstHit := source["hits"].(map[string]interface{})["hits"].([]interface{})[0]
	firstHitMap, ok := firstHit.(map[string]interface{})
	if !ok {
		log.Fatalf("error converting first hit to map[string]interface{}")
		return nil, fmt.Errorf("error converting first hit to map[string]interface{}")
	}

	bx, err := json.MarshalIndent(firstHitMap["_source"], "", "\t")
	if err != nil {
		storage.log.Errorf("error marshalling the _source, error: %+v", err)
		return nil, err
	}
	blogRes := &pb.GetBlogByIdRes{}

	if err = json.Unmarshal(bx, blogRes); err != nil {
		storage.log.Errorf("error un-marshalling the bytes into struct, error: %+v", err)
		return nil, err
	}

	storage.log.Infof("successfully fetched blog with id: %s", id)
	return blogRes, nil
}

func (os *opensearchStorage) ArchieveBlogById(ctx context.Context, blogId string) (*opensearchapi.Response, error) {
	os.log.Infof("archiving blog with id: %s", blogId)

	// Define the update script
	updateScript := `{
		"script" : {
			"source": "ctx._source.is_archive = params.is_archive",
			"lang": "painless",
			"params" : {
				"is_archive" : true
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
		os.log.Errorf("Error while archiving blog, error: %+v", err)
		return updateResponse, err
	}

	if updateResponse.IsError() {
		err = fmt.Errorf("error archiving blog, update response: %+v", updateResponse)
		os.log.Error(err)
		return updateResponse, err
	}

	os.log.Infof("Successfully archiving blog with id: %s", blogId)
	return updateResponse, nil
}

func (os *opensearchStorage) GetBlogDetailsById(ctx context.Context, blogId string) (string, []string, error) {
	os.log.Infof("Fetching blog with id: %s", blogId)

	// Define the search request
	searchRequest := `{
		"query": {
			"term": {
				"blog_id": "%s"
			}
		}
	}`

	osReq := opensearchapi.SearchRequest{
		Index: []string{constants.OpensearchArticleIndex},
		Body:  strings.NewReader(fmt.Sprintf(searchRequest, blogId)),
	}

	searchResponse, err := osReq.Do(ctx, os.client)
	if err != nil {
		os.log.Errorf("Error while fetching blog, error: %+v", err)
		return "", nil, err
	}

	if searchResponse.IsError() {
		err = fmt.Errorf("error fetching blog, search response: %+v", searchResponse)
		os.log.Error(err)
		return "", nil, err
	}

	var r map[string]interface{}
	if err := json.NewDecoder(searchResponse.Body).Decode(&r); err != nil {
		return "", nil, err
	}

	for _, hit := range r["hits"].(map[string]interface{})["hits"].([]interface{}) {
		source := hit.(map[string]interface{})["_source"].(map[string]interface{})
		ownerAccountId := source["owner_account_id"].(string)
		tagsInterface := source["tags"].([]interface{})
		tags := make([]string, len(tagsInterface))
		for i, tag := range tagsInterface {
			tags[i] = tag.(string)
		}
		return ownerAccountId, tags, nil
	}

	return "", nil, fmt.Errorf("No matching blog found")
}

func (os *opensearchStorage) GetPublishedBlogByTagsName(ctx context.Context, tags ...string) (*pb.GetBlogsByTagsNameRes, error) {
	// Convert the tags slice to a JSON array
	tagsJson, err := json.Marshal(tags)
	if err != nil {
		return nil, err
	}

	// Construct the query
	query := fmt.Sprintf(`{
        "query": {
            "bool": {
                "must": [
                    { "terms": { "tags": %s } },
                    { "term": { "is_draft": false } }
                ],
                "should": [
                    { "bool": { "must_not": { "exists": { "field": "is_archive" } } } },
                    { "term": { "is_archive": false } }
                ],
                "minimum_should_match": 1
            }
        }
    }`, string(tagsJson))

	// Send the search request
	res, err := os.client.Search(
		os.client.Search.WithContext(context.Background()),
		os.client.Search.WithIndex(constants.OpensearchArticleIndex),
		os.client.Search.WithBody(strings.NewReader(query)),
		os.client.Search.WithPretty(),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	// Read and unmarshal the response body
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var source map[string]interface{}
	err = json.Unmarshal(bodyBytes, &source)
	if err != nil {
		return nil, err
	}

	// Extract the hits
	hits := source["hits"].(map[string]interface{})["hits"].([]interface{})
	if len(hits) == 0 {
		return nil, fmt.Errorf("no blogs found with tags: %v", tags)
	}

	// Unmarshal each hit into a GetBlogByTagNameRes struct
	blogsRes := &pb.GetBlogsByTagsNameRes{}
	for _, hit := range hits {
		hitMap, ok := hit.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("error converting hit to map[string]interface{}")
		}
		bx, err := json.MarshalIndent(hitMap["_source"], "", "\t")
		if err != nil {
			return nil, err
		}
		blogRes := &pb.GetBlogsByTags{}
		if err = json.Unmarshal(bx, blogRes); err != nil {
			return nil, err
		}
		blogsRes.TheBlogs = append(blogsRes.TheBlogs, blogRes)
	}

	return blogsRes, nil
}

// func (storage *opensearchStorage) GetBlogById(ctx context.Context, req *pb.GetBlogByIdReq) (*pb.GetBlogByIdRes, error) {
// 	storage.log.Infof("fetching blog with id: %s", req.BlogId)

// 	osReq := opensearchapi.GetRequest{
// 		Index:      constants.OpensearchArticleIndex,
// 		DocumentID: req.BlogId,
// 	}

// 	getResponse, err := osReq.Do(ctx, storage.client)
// 	if err != nil {
// 		storage.log.Errorf("error while fetching blog, error: %+v", err)
// 		return nil, err
// 	}

// 	if getResponse.IsError() {
// 		if getResponse.StatusCode == http.StatusNotFound {
// 			storage.log.Errorf("blog with id: %s does not exist", req.BlogId)
// 			return nil, fmt.Errorf("blog with id: %s does not exist", req.BlogId)
// 		}
// 		err = fmt.Errorf("error fetching blog, get response: %+v", getResponse)
// 		storage.log.Error(err)
// 		return nil, err
// 	}

// 	// Read the body into a byte slice
// 	bodyBytes, err := io.ReadAll(getResponse.Body)
// 	if err != nil {
// 		storage.log.Errorf("error reading response body, error: %+v", err)
// 		return nil, err
// 	}

// 	var source map[string]interface{}
// 	err = json.Unmarshal(bodyBytes, &source)
// 	if err != nil {
// 		storage.log.Errorf("error unmarshalling blog, error: %+v", err)
// 		return nil, err
// 	}

// 	bx, err := json.MarshalIndent(source["_source"].(map[string]interface{}), "", "\t")
// 	if err != nil {
// 		storage.log.Errorf("error marshalling the _source, error: %+v", err)
// 		return nil, err
// 	}
// 	blogRes := &pb.GetBlogByIdRes{}

// 	if err = json.Unmarshal(bx, blogRes); err != nil {
// 		storage.log.Errorf("error un-marshalling the bytes into struct, error: %+v", err)
// 		return nil, err
// 	}

// 	storage.log.Infof("successfully fetched blog with id: %s", req.BlogId)
// 	return blogRes, nil
// }
