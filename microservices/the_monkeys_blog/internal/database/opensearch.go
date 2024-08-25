package database

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/opensearch-project/opensearch-go/opensearchapi"
	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_blog/pb"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_blog/internal/constants"
)

type ElasticsearchStorage interface {
	DraftABlog(ctx context.Context, blog *pb.DraftBlogRequest) (*esapi.Response, error)
	GetDraftBlogsByOwnerAccountID(ctx context.Context, ownerAccountID string) (*pb.GetDraftBlogsRes, error)
	DoesBlogExist(ctx context.Context, blogID string) (bool, error)
	PublishBlogById(ctx context.Context, blogId string) (*esapi.Response, error)
	GetPublishedBlogByTagsName(ctx context.Context, tags ...string) (*pb.GetBlogsByTagsNameRes, error)
	GetPublishedBlogById(ctx context.Context, id string) (*pb.GetBlogByIdRes, error)

	// GetBlogById(ctx context.Context, req *pb.GetBlogByIdReq) (*pb.GetBlogByIdRes, error)
	GetBlogDetailsById(ctx context.Context, blogId string) (string, []string, error)
	AchieveBlogById(ctx context.Context, blogId string) (*esapi.Response, error)
}

type elasticsearchStorage struct {
	client *elasticsearch.Client
	log    *logrus.Logger
}

func NewElasticsearchClient(url, username, password string, log *logrus.Logger) (ElasticsearchStorage, error) {
	client, err := NewESClient(url, username, password)
	if err != nil {
		log.Errorf("Failed to connect to Elasticsearch instance, error: %+v", err)
		return nil, err
	}

	return &elasticsearchStorage{
		client: client,
		log:    log,
	}, nil
}

func (es *elasticsearchStorage) DraftABlog(ctx context.Context, blog *pb.DraftBlogRequest) (*esapi.Response, error) {
	bs, err := json.Marshal(blog)
	if err != nil {
		es.log.Errorf("DraftABlog: cannot marshal the blog, error: %v", err)
		return nil, err
	}

	document := strings.NewReader(string(bs))

	req := esapi.IndexRequest{
		Index:      constants.ElasticsearchBlogIndex,
		DocumentID: blog.BlogId,
		Body:       document,
	}

	insertResponse, err := req.Do(ctx, es.client)
	if err != nil {
		es.log.Errorf("DraftABlog: error while indexing blog, error: %+v", err)
		return insertResponse, err
	}

	if insertResponse.IsError() {
		err = fmt.Errorf("DraftABlog: error indexing blog, response: %+v", insertResponse)
		es.log.Error(err)
		return insertResponse, err
	}

	es.log.Infof("DraftABlog: successfully created blog for user: %s, response: %+v", blog.OwnerAccountId, insertResponse)
	return insertResponse, nil
}

func (es *elasticsearchStorage) GetDraftBlogsByOwnerAccountID(ctx context.Context, ownerAccountID string) (*pb.GetDraftBlogsRes, error) {
	// Ensure ownerAccountID is properly set
	if ownerAccountID == "" {
		es.log.Error("GetDraftBlogsByOwnerAccountID: ownerAccountID is empty")
		return nil, fmt.Errorf("ownerAccountID cannot be empty")
	}

	// Build the query to search for draft blogs by owner_account_id
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{
						"term": map[string]interface{}{
							"owner_account_id.keyword": ownerAccountID,
						},
					},
					{
						"term": map[string]interface{}{
							"is_draft": true,
						},
					},
				},
			},
		},
	}

	// Marshal the query to JSON
	bs, err := json.Marshal(query)
	if err != nil {
		es.log.Errorf("GetDraftBlogsByOwnerAccountID: cannot marshal the query, error: %v", err)
		return nil, err
	}

	// Print the query for debugging
	es.log.Infof("Executing query: %s", string(bs))

	// Create a new search request with the query
	req := esapi.SearchRequest{
		Index: []string{constants.ElasticsearchBlogIndex},
		Body:  strings.NewReader(string(bs)),
	}

	// Execute the search request
	res, err := req.Do(ctx, es.client)
	if err != nil {
		es.log.Errorf("GetDraftBlogsByOwnerAccountID: error executing search request, error: %+v", err)
		return nil, err
	}
	defer res.Body.Close()

	// Check if the response indicates an error
	if res.IsError() {
		err = fmt.Errorf("GetDraftBlogsByOwnerAccountID: search query failed, response: %+v", res)
		es.log.Error(err)
		return nil, err
	}

	// Read the response body
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		es.log.Errorf("GetDraftBlogsByOwnerAccountID: error reading response body, error: %v", err)
		return nil, err
	}

	// Print the response body for debugging
	// es.log.Infof("Search response body: %s", string(bodyBytes))

	// Parse the response body
	var esResponse map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &esResponse); err != nil {
		es.log.Errorf("GetDraftBlogsByOwnerAccountID: error decoding response body, error: %v", err)
		return nil, err
	}

	// Extract the hits from the response
	hits, ok := esResponse["hits"].(map[string]interface{})["hits"].([]interface{})
	if !ok {
		err := fmt.Errorf("GetDraftBlogsByOwnerAccountID: failed to parse hits from response")
		es.log.Error(err)
		return nil, err
	}

	// Convert the hits to a slice of DraftBlogRequest
	var blogs = &pb.GetDraftBlogsRes{
		Blogs: make([]*pb.GetBlogs, 0, len(hits)),
	}
	for _, hit := range hits {
		hitSource := hit.(map[string]interface{})["_source"]
		hitBytes, err := json.Marshal(hitSource)
		if err != nil {
			es.log.Errorf("GetDraftBlogsByOwnerAccountID: error marshaling hit source, error: %v", err)
			continue
		}

		var blog pb.GetBlogs
		if err := json.Unmarshal(hitBytes, &blog); err != nil {
			es.log.Errorf("GetDraftBlogsByOwnerAccountID: error unmarshaling hit to DraftBlogRequest, error: %v", err)
			continue
		}
		blogs.Blogs = append(blogs.Blogs, &blog)
	}

	es.log.Infof("GetDraftBlogsByOwnerAccountID: successfully fetched %d draft blogs for owner_account_id: %s", len(blogs.Blogs), ownerAccountID)
	return blogs, nil
}

func (es *elasticsearchStorage) DoesBlogExist(ctx context.Context, blogID string) (bool, error) {
	// Ensure blogID is not empty
	if blogID == "" {
		es.log.Error("DoesBlogExist: blogID is empty")
		return false, fmt.Errorf("blogID cannot be empty")
	}

	// Create a Get request to check if the document exists
	req := esapi.GetRequest{
		Index:      constants.ElasticsearchBlogIndex,
		DocumentID: blogID,
	}

	// Execute the Get request
	getResponse, err := req.Do(ctx, es.client)
	if err != nil {
		es.log.Errorf("DoesBlogExist: error executing Get request, error: %+v", err)
		return false, err
	}
	defer getResponse.Body.Close()

	// Check if the response indicates the document exists
	if getResponse.StatusCode == http.StatusOK {
		es.log.Infof("DoesBlogExist: blog with id %s exists", blogID)
		return true, nil
	} else if getResponse.StatusCode == http.StatusNotFound {
		es.log.Infof("DoesBlogExist: blog with id %s does not exist", blogID)
		return false, nil
	}

	// If the response is something else, log it as an error
	err = fmt.Errorf("DoesBlogExist: unexpected status code %d", getResponse.StatusCode)
	es.log.Error(err)
	return false, err
}

func (es *elasticsearchStorage) PublishBlogById(ctx context.Context, blogId string) (*esapi.Response, error) {
	// Ensure blogId is not empty
	if blogId == "" {
		es.log.Error("PublishBlogById: blogId is empty")
		return nil, fmt.Errorf("blogId cannot be empty")
	}

	// Build the update query to set is_draft to false
	updateScript := map[string]interface{}{
		"script": map[string]interface{}{
			"source": "ctx._source.is_draft = false",
		},
	}

	// Marshal the update script to JSON
	bs, err := json.Marshal(updateScript)
	if err != nil {
		es.log.Errorf("PublishBlogById: cannot marshal the update script, error: %v", err)
		return nil, err
	}

	// Create an update request
	req := esapi.UpdateRequest{
		Index:      constants.ElasticsearchBlogIndex,
		DocumentID: blogId,
		Body:       strings.NewReader(string(bs)),
	}

	// Execute the update request
	updateResponse, err := req.Do(ctx, es.client)
	if err != nil {
		es.log.Errorf("PublishBlogById: error executing update request, error: %+v", err)
		return updateResponse, err
	}
	defer updateResponse.Body.Close()

	// Check if the response indicates an error
	if updateResponse.IsError() {
		err = fmt.Errorf("PublishBlogById: update query failed, response: %+v", updateResponse)
		es.log.Error(err)
		return updateResponse, err
	}

	es.log.Infof("PublishBlogById: successfully published blog with id: %s", blogId)
	return updateResponse, nil
}

func (es *elasticsearchStorage) GetPublishedBlogByTagsName(ctx context.Context, tags ...string) (*pb.GetBlogsByTagsNameRes, error) {
	// Ensure at least one tag is provided
	if len(tags) == 0 {
		es.log.Error("GetPublishedBlogByTagsName: no tags provided")
		return nil, fmt.Errorf("at least one tag must be provided")
	}

	// Build the query to search for published blogs by tags
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{
						"terms": map[string]interface{}{
							"tags.keyword": tags,
						},
					},
					{
						"term": map[string]interface{}{
							"is_draft": false,
						},
					},
				},
			},
		},
	}

	// Marshal the query to JSON
	bs, err := json.Marshal(query)
	if err != nil {
		es.log.Errorf("GetPublishedBlogByTagsName: cannot marshal the query, error: %v", err)
		return nil, err
	}

	// Print the query for debugging
	es.log.Infof("Executing query: %s", string(bs))

	// Create a new search request with the query
	req := esapi.SearchRequest{
		Index: []string{constants.ElasticsearchBlogIndex},
		Body:  strings.NewReader(string(bs)),
	}

	// Execute the search request
	res, err := req.Do(ctx, es.client)
	if err != nil {
		es.log.Errorf("GetPublishedBlogByTagsName: error executing search request, error: %+v", err)
		return nil, err
	}
	defer res.Body.Close()

	// Check if the response indicates an error
	if res.IsError() {
		err = fmt.Errorf("GetPublishedBlogByTagsName: search query failed, response: %+v", res)
		es.log.Error(err)
		return nil, err
	}

	// Read the response body
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		es.log.Errorf("GetPublishedBlogByTagsName: error reading response body, error: %v", err)
		return nil, err
	}

	// Parse the response body
	var esResponse map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &esResponse); err != nil {
		es.log.Errorf("GetPublishedBlogByTagsName: error decoding response body, error: %v", err)
		return nil, err
	}

	// Extract the hits from the response
	hits, ok := esResponse["hits"].(map[string]interface{})["hits"].([]interface{})
	if !ok {
		err := fmt.Errorf("GetPublishedBlogByTagsName: failed to parse hits from response")
		es.log.Error(err)
		return nil, err
	}

	// Convert the hits to a slice of GetBlogs
	var blogs = &pb.GetBlogsByTagsNameRes{
		TheBlogs: make([]*pb.GetBlogsByTags, 0, len(hits)),
	}
	for _, hit := range hits {
		hitSource := hit.(map[string]interface{})["_source"]
		hitBytes, err := json.Marshal(hitSource)
		if err != nil {
			es.log.Errorf("GetPublishedBlogByTagsName: error marshaling hit source, error: %v", err)
			continue
		}

		var blog pb.GetBlogsByTags
		if err := json.Unmarshal(hitBytes, &blog); err != nil {
			es.log.Errorf("GetPublishedBlogByTagsName: error unmarshaling hit to GetBlogs, error: %v", err)
			continue
		}
		blogs.TheBlogs = append(blogs.TheBlogs, &blog)
	}

	es.log.Infof("GetPublishedBlogByTagsName: successfully fetched %d published blogs for tags: %v", len(blogs.TheBlogs), tags)
	return blogs, nil
}

func (es *elasticsearchStorage) GetPublishedBlogById(ctx context.Context, id string) (*pb.GetBlogByIdRes, error) {
	// Ensure id is not empty
	if id == "" {
		es.log.Error("GetPublishedBlogById: id is empty")
		return nil, fmt.Errorf("blog id cannot be empty")
	}

	// Build the query to search for a published blog by id
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{
						"term": map[string]interface{}{
							"blog_id.keyword": id,
						},
					},
					{
						"term": map[string]interface{}{
							"is_draft": false,
						},
					},
				},
			},
		},
	}

	// Marshal the query to JSON
	bs, err := json.Marshal(query)
	if err != nil {
		es.log.Errorf("GetPublishedBlogById: cannot marshal the query, error: %v", err)
		return nil, err
	}

	// Print the query for debugging
	es.log.Infof("Executing query: %s", string(bs))

	// Create a new search request with the query
	req := esapi.SearchRequest{
		Index: []string{constants.ElasticsearchBlogIndex},
		Body:  strings.NewReader(string(bs)),
	}

	// Execute the search request
	res, err := req.Do(ctx, es.client)
	if err != nil {
		es.log.Errorf("GetPublishedBlogById: error executing search request, error: %+v", err)
		return nil, err
	}
	defer res.Body.Close()

	// Check if the response indicates an error
	if res.IsError() {
		err = fmt.Errorf("GetPublishedBlogById: search query failed, response: %+v", res)
		es.log.Error(err)
		return nil, err
	}

	// Read the response body
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		es.log.Errorf("GetPublishedBlogById: error reading response body, error: %v", err)
		return nil, err
	}

	// Parse the response body
	var esResponse map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &esResponse); err != nil {
		es.log.Errorf("GetPublishedBlogById: error decoding response body, error: %v", err)
		return nil, err
	}

	// Extract the hits from the response
	hits, ok := esResponse["hits"].(map[string]interface{})["hits"].([]interface{})
	if !ok || len(hits) == 0 {
		es.log.Infof("GetPublishedBlogById: no published blog found with id: %s", id)
		return nil, nil
	}

	// Convert the first hit to GetBlogByIdRes
	hitSource := hits[0].(map[string]interface{})["_source"]
	hitBytes, err := json.Marshal(hitSource)
	if err != nil {
		es.log.Errorf("GetPublishedBlogById: error marshaling hit source, error: %v", err)
		return nil, err
	}

	var blog pb.GetBlogByIdRes
	if err := json.Unmarshal(hitBytes, &blog); err != nil {
		es.log.Errorf("GetPublishedBlogById: error unmarshaling hit to GetBlogByIdRes, error: %v", err)
		return nil, err
	}

	es.log.Infof("GetPublishedBlogById: successfully fetched published blog with id: %s", id)
	return &blog, nil
}

// ********************************************************  Below function need to be re-written ********************************************************
// func (storage *elasticsearchStorage) GetPublishedBlogById(ctx context.Context, id string) (*pb.GetBlogByIdRes, error) {
// 	res, err := storage.client.Search(
// 		storage.client.Search.WithContext(context.Background()),
// 		storage.client.Search.WithIndex(constants.ElasticsearchBlogIndex),
// 		storage.client.Search.WithBody(strings.NewReader(fmt.Sprintf(`{
// 			"query": {
// 				"bool": {
// 					"must": [
// 						{ "term": { "blog_id": "%s" } },
// 						{ "term": { "is_draft": false } }
// 					],
// 					"should": [
// 						{ "bool": { "must_not": { "exists": { "field": "is_archive" } } } },
// 						{ "term": { "is_archive": false } }
// 					],
// 					"minimum_should_match": 1
// 				}
// 			}
// 		}`, id))),
// 		storage.client.Search.WithPretty(),
// 	)

// 	// storage.log.Infof("Response: %+v", res)
// 	if err != nil {
// 		log.Fatalf("fetching the blog: %s", err)
// 		return nil, err
// 	}
// 	defer res.Body.Close()

// 	bodyBytes, err := io.ReadAll(res.Body)
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

// 	if len(source["hits"].(map[string]interface{})["hits"].([]interface{})) == 0 {
// 		storage.log.Errorf("no blog found with id: %s", id)
// 		return nil, fmt.Errorf("no blog found with id: %s", id)
// 	}

// 	firstHit := source["hits"].(map[string]interface{})["hits"].([]interface{})[0]
// 	firstHitMap, ok := firstHit.(map[string]interface{})
// 	if !ok {
// 		log.Fatalf("error converting first hit to map[string]interface{}")
// 		return nil, fmt.Errorf("error converting first hit to map[string]interface{}")
// 	}

// 	bx, err := json.MarshalIndent(firstHitMap["_source"], "", "\t")
// 	if err != nil {
// 		storage.log.Errorf("error marshalling the _source, error: %+v", err)
// 		return nil, err
// 	}
// 	blogRes := &pb.GetBlogByIdRes{}

// 	if err = json.Unmarshal(bx, blogRes); err != nil {
// 		storage.log.Errorf("error un-marshalling the bytes into struct, error: %+v", err)
// 		return nil, err
// 	}

// 	storage.log.Infof("successfully fetched blog with id: %s", id)
// 	return blogRes, nil
// }

func (os *elasticsearchStorage) AchieveBlogById(ctx context.Context, blogId string) (*esapi.Response, error) {
	os.log.Infof("archiving blog with id: %s", blogId)

	// // Define the update script
	// updateScript := `{
	// 	"script" : {
	// 		"source": "ctx._source.is_archive = params.is_archive",
	// 		"lang": "painless",
	// 		"params" : {
	// 			"is_archive" : true
	// 		}
	// 	}
	// }`

	// osReq := opensearchapi.UpdateRequest{
	// 	Index:      constants.OpensearchArticleIndex,
	// 	DocumentID: blogId,
	// 	Body:       strings.NewReader(updateScript),
	// }

	// updateResponse, err := osReq.Do(ctx, os.client)
	// if err != nil {
	// 	os.log.Errorf("Error while archiving blog, error: %+v", err)
	// 	return updateResponse, err
	// }

	// if updateResponse.IsError() {
	// 	err = fmt.Errorf("error archiving blog, update response: %+v", updateResponse)
	// 	os.log.Error(err)
	// 	return updateResponse, err
	// }

	// os.log.Infof("Successfully archiving blog with id: %s", blogId)
	// return updateResponse, nil
	return nil, nil
}

func (os *elasticsearchStorage) GetBlogDetailsById(ctx context.Context, blogId string) (string, []string, error) {
	os.log.Infof("Fetching blog with id: %s", blogId)

	// Define the search request
	searchRequest := fmt.Sprintf(`{
		"query": {
			"term": {
				"blog_id": {
					"value": "%s"
				}
			}
		}
	}`, blogId)

	osReq := opensearchapi.SearchRequest{
		Index: []string{constants.ElasticsearchBlogIndex},
		Body:  strings.NewReader(searchRequest),
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

	// Log the entire response for debugging
	os.log.Infof("Search response: %+v", r)

	hitsData, ok := r["hits"].(map[string]interface{})
	if !ok {
		return "", nil, fmt.Errorf("No matching blog found: missing hits in response")
	}

	hits, ok := hitsData["hits"].([]interface{})
	if !ok || len(hits) == 0 {
		return "", nil, fmt.Errorf("No matching blog found: empty hits array")
	}

	hit := hits[0].(map[string]interface{})
	source := hit["_source"].(map[string]interface{})
	ownerAccountId := source["owner_account_id"].(string)
	tagsInterface := source["tags"].([]interface{})
	tags := make([]string, len(tagsInterface))
	for i, tag := range tagsInterface {
		tags[i] = tag.(string)
	}

	return ownerAccountId, tags, nil
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
