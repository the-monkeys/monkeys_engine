package database

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_blog/pb"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_blog/internal/constants"
)

type ElasticsearchStorage interface {
	DraftABlog(ctx context.Context, blog *pb.DraftBlogRequest) (*esapi.Response, error)
	GetDraftBlogsByOwnerAccountID(ctx context.Context, ownerAccountID string) (*pb.GetDraftBlogsRes, error)
	GetDraftBlogByBlogId(ctx context.Context, blogId string) (*pb.BlogByIdRes, error)
	DoesBlogExist(ctx context.Context, blogID string) (bool, map[string]interface{}, error)
	PublishBlogById(ctx context.Context, blogId string) (*esapi.Response, error)
	MoveBlogToDraft(ctx context.Context, blogId string) (*esapi.Response, error)
	GetPublishedBlogByTagsName(ctx context.Context, tags ...string) (*pb.GetBlogsByTagsNameRes, error)
	GetPublishedBlogById(ctx context.Context, id string) (*pb.BlogByIdRes, error)
	AchieveAPublishedBlogById(ctx context.Context, blogId string) (*esapi.Response, error)
	DeleteABlogById(ctx context.Context, blogId string) (*esapi.Response, error)
	GetLast100BlogsLatestFirst(ctx context.Context) (*pb.GetBlogsByTagsNameRes, error)
	GetDraftedBlogByIdAndOwner(ctx context.Context, blogId, ownerAccountId string) (*pb.BlogByIdRes, error)
	GetPublishedBlogByIdAndOwner(ctx context.Context, blogId, ownerAccountId string) (*pb.BlogByIdRes, error)
	GetPublishedBlogsByOwnerAccountID(ctx context.Context, ownerAccountID string) (*pb.GetPublishedBlogsRes, error)
	GetBlogsByBlogIds(ctx context.Context, blogIds []string) (*pb.GetBlogsRes, error)
	DeleteBlogsByOwnerAccountID(ctx context.Context, ownerAccountId string) (*esapi.Response, error)

	// -------------------------------------------------------------------------------- V2 --------------------------------------------------------------------------------
	SaveBlog(ctx context.Context, blog map[string]interface{}) (*esapi.Response, error)
	GetBlogsOfUsersByAccountIds(ctx context.Context, accountIds []string, limit, offset int32) ([]map[string]interface{}, error)
	GetBlogsByTagsAccId(ctx context.Context, accountId string, tags []string, isDraft bool, limit, offset int32) ([]map[string]interface{}, error)
	GetBlogsByAccountId(ctx context.Context, accountId string, isDraft bool, limit, offset int32) ([]map[string]interface{}, error)
	GetBlogByBlogId(ctx context.Context, blogId string, isDraft bool) (map[string]interface{}, error)
	GetABlogByBlogIdAccId(ctx context.Context, blogId, accountId string, isDraft bool) (map[string]interface{}, error)
	GetBlogsByTags(ctx context.Context, tags []string, isDraft bool, limit, offset int32) ([]map[string]interface{}, error)
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

	// Build the query to search for draft blogs by owner_account_id, sorted by time in descending order
	query := map[string]interface{}{
		"sort": []map[string]interface{}{
			{
				"blog.time": map[string]string{
					"order": "desc",
				},
			},
		},
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

	// Convert the hits to a slice of GetDraftBlogsRes
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
			es.log.Errorf("GetDraftBlogsByOwnerAccountID: error unmarshaling hit to GetBlogs, error: %v", err)
			continue
		}
		blogs.Blogs = append(blogs.Blogs, &blog)
	}

	es.log.Infof("GetDraftBlogsByOwnerAccountID: successfully fetched %d draft blogs for owner_account_id: %s", len(blogs.Blogs), ownerAccountID)
	return blogs, nil
}

func (es *elasticsearchStorage) GetPublishedBlogsByOwnerAccountID(ctx context.Context, ownerAccountID string) (*pb.GetPublishedBlogsRes, error) {
	// Ensure ownerAccountID is properly set
	if ownerAccountID == "" {
		es.log.Error("GetPublishedBlogsByOwnerAccountID: ownerAccountID is empty")
		return nil, fmt.Errorf("ownerAccountID cannot be empty")
	}

	// Build the query to search for published blogs by owner_account_id, sorted by time in descending order
	query := map[string]interface{}{
		"sort": []map[string]interface{}{
			{
				"blog.time": map[string]string{
					"order": "desc",
				},
			},
		},
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
							"is_draft": false,
						},
					},
				},
				"must_not": []map[string]interface{}{
					{
						"term": map[string]interface{}{
							"is_archived": true,
						},
					},
				},
				"should": []map[string]interface{}{
					{
						"term": map[string]interface{}{
							"is_archived": false,
						},
					},
					{
						"bool": map[string]interface{}{
							"must_not": map[string]interface{}{
								"exists": map[string]interface{}{
									"field": "is_archived",
								},
							},
						},
					},
				},
				"minimum_should_match": 1,
			},
		},
	}

	// Marshal the query to JSON
	bs, err := json.Marshal(query)
	if err != nil {
		es.log.Errorf("GetPublishedBlogsByOwnerAccountID: cannot marshal the query, error: %v", err)
		return nil, err
	}

	// Create a new search request with the query
	req := esapi.SearchRequest{
		Index: []string{constants.ElasticsearchBlogIndex},
		Body:  strings.NewReader(string(bs)),
	}

	// Execute the search request
	res, err := req.Do(ctx, es.client)
	if err != nil {
		es.log.Errorf("GetPublishedBlogsByOwnerAccountID: error executing search request, error: %+v", err)
		return nil, err
	}
	defer res.Body.Close()

	// Check if the response indicates an error
	if res.IsError() {
		err = fmt.Errorf("GetPublishedBlogsByOwnerAccountID: search query failed, response: %+v", res)
		es.log.Error(err)
		return nil, err
	}

	// Read the response body
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		es.log.Errorf("GetPublishedBlogsByOwnerAccountID: error reading response body, error: %v", err)
		return nil, err
	}

	// Parse the response body
	var esResponse map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &esResponse); err != nil {
		es.log.Errorf("GetPublishedBlogsByOwnerAccountID: error decoding response body, error: %v", err)
		return nil, err
	}

	// Extract the hits from the response
	hits, ok := esResponse["hits"].(map[string]interface{})["hits"].([]interface{})
	if !ok {
		err := fmt.Errorf("GetPublishedBlogsByOwnerAccountID: failed to parse hits from response")
		es.log.Error(err)
		return nil, err
	}

	// Convert the hits to a slice of GetDraftBlogsRes
	var blogs = &pb.GetPublishedBlogsRes{
		Blogs: make([]*pb.GetBlogs, 0, len(hits)),
	}
	for _, hit := range hits {
		hitSource := hit.(map[string]interface{})["_source"]
		hitBytes, err := json.Marshal(hitSource)
		if err != nil {
			es.log.Errorf("GetPublishedBlogsByOwnerAccountID: error marshaling hit source, error: %v", err)
			continue
		}

		var blog pb.GetBlogs
		if err := json.Unmarshal(hitBytes, &blog); err != nil {
			es.log.Errorf("GetPublishedBlogsByOwnerAccountID: error unmarshaling hit to GetBlogs, error: %v", err)
			continue
		}
		blogs.Blogs = append(blogs.Blogs, &blog)
	}

	es.log.Infof("GetPublishedBlogsByOwnerAccountID: successfully fetched %d published blogs for owner_account_id: %s", len(blogs.Blogs), ownerAccountID)
	return blogs, nil
}

func (es *elasticsearchStorage) DoesBlogExist(ctx context.Context, blogID string) (bool, map[string]interface{}, error) {
	// Ensure blogID is not empty
	if blogID == "" {
		es.log.Error("DoesBlogExist: blogID is empty")
		return false, nil, fmt.Errorf("blogID cannot be empty")
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
		return false, nil, err
	}
	defer getResponse.Body.Close()

	// Check if the response indicates the document exists
	if getResponse.StatusCode == http.StatusOK {
		// Parse the response body to extract blog details
		bodyBytes, err := io.ReadAll(getResponse.Body)
		if err != nil {
			es.log.Errorf("DoesBlogExist: error reading response body, error: %v", err)
			return false, nil, err
		}

		var response map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &response); err != nil {
			es.log.Errorf("DoesBlogExist: error decoding response body, error: %v", err)
			return false, nil, err
		}

		// Extract the _source field which contains the blog details
		blogDetails, ok := response["_source"].(map[string]interface{})
		if !ok {
			es.log.Errorf("DoesBlogExist: failed to parse _source from response")
			return false, nil, fmt.Errorf("failed to parse blog details from response")
		}

		es.log.Infof("DoesBlogExist: blog with id %s exists", blogID)
		return true, blogDetails, nil
	} else if getResponse.StatusCode == http.StatusNotFound {
		es.log.Infof("DoesBlogExist: blog with id %s does not exist", blogID)
		return false, nil, nil
	}

	// If the response is something else, log it as an error
	err = fmt.Errorf("DoesBlogExist: unexpected status code %d", getResponse.StatusCode)
	es.log.Error(err)
	return false, nil, err
}

func (es *elasticsearchStorage) PublishBlogById(ctx context.Context, blogId string) (*esapi.Response, error) {
	// Ensure blogId is not empty
	if blogId == "" {
		es.log.Error("PublishBlogById: blogId is empty")
		return nil, fmt.Errorf("blogId cannot be empty")
	}

	// Build the update query to set is_draft to false and add published_time
	updateScript := map[string]interface{}{
		"script": map[string]interface{}{
			"source": "ctx._source.is_draft = false; ctx._source.published_time = params.published_time;",
			"params": map[string]interface{}{
				"published_time": time.Now().Format(time.RFC3339),
			},
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

func (es *elasticsearchStorage) MoveBlogToDraft(ctx context.Context, blogId string) (*esapi.Response, error) {
	// Ensure blogId is not empty
	if blogId == "" {
		es.log.Error("MoveBlogToDraft: blogId is empty")
		return nil, fmt.Errorf("blogId cannot be empty")
	}

	// Build the update query to set is_draft to false and add published_time
	updateScript := map[string]interface{}{
		"script": map[string]interface{}{
			"source": "ctx._source.is_draft = true; ctx._source.published_time = params.published_time;",
			"params": map[string]interface{}{
				"published_time": time.Now().Format(time.RFC3339),
			},
		},
	}

	// Marshal the update script to JSON
	bs, err := json.Marshal(updateScript)
	if err != nil {
		es.log.Errorf("MoveBlogToDraft: cannot marshal the update script, error: %v", err)
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
		es.log.Errorf("MoveBlogToDraft: error executing update request, error: %+v", err)
		return updateResponse, err
	}
	defer updateResponse.Body.Close()

	// Check if the response indicates an error
	if updateResponse.IsError() {
		err = fmt.Errorf("MoveBlogToDraft: update query failed, response: %+v", updateResponse)
		es.log.Error(err)
		return updateResponse, err
	}

	es.log.Infof("MoveBlogToDraft: successfully published blog with id: %s", blogId)
	return updateResponse, nil
}

func (es *elasticsearchStorage) GetPublishedBlogByTagsName(ctx context.Context, tags ...string) (*pb.GetBlogsByTagsNameRes, error) {
	// Ensure at least one tag is provided
	if len(tags) == 0 {
		es.log.Error("GetPublishedBlogByTagsName: no tags provided")
		return nil, fmt.Errorf("at least one tag must be provided")
	}

	// Build the query to search for published blogs by tags with the `is_archived` filtering
	query := map[string]interface{}{
		"sort": []map[string]interface{}{
			{
				"blog.time": map[string]string{
					"order": "desc",
				},
			},
		},
		"size": 100,
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
				"must_not": []map[string]interface{}{
					{
						"term": map[string]interface{}{
							"is_archived": true,
						},
					},
				},
				"should": []map[string]interface{}{
					{
						"term": map[string]interface{}{
							"is_archived": false,
						},
					},
					{
						"bool": map[string]interface{}{
							"must_not": map[string]interface{}{
								"exists": map[string]interface{}{
									"field": "is_archived",
								},
							},
						},
					},
				},
				"minimum_should_match": 1,
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

	// Convert the hits to a slice of GetBlogsByTagsNameRes
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
			es.log.Errorf("GetPublishedBlogByTagsName: error unmarshaling hit to GetBlogsByTags, error: %v", err)
			continue
		}
		blogs.TheBlogs = append(blogs.TheBlogs, &blog)
	}

	es.log.Infof("GetPublishedBlogByTagsName: successfully fetched %d published blogs for tags: %v", len(blogs.TheBlogs), tags)
	return blogs, nil
}

func (es *elasticsearchStorage) GetPublishedBlogById(ctx context.Context, id string) (*pb.BlogByIdRes, error) {
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

	var blog pb.BlogByIdRes
	if err := json.Unmarshal(hitBytes, &blog); err != nil {
		es.log.Errorf("GetPublishedBlogById: error unmarshaling hit to GetBlogByIdRes, error: %v", err)
		return nil, err
	}

	es.log.Infof("GetPublishedBlogById: successfully fetched published blog with id: %s", id)
	return &blog, nil
}

// AchieveAPublishedBlogById archives a published blog by setting an "is_archived" field to true
func (es *elasticsearchStorage) AchieveAPublishedBlogById(ctx context.Context, blogId string) (*esapi.Response, error) {
	// Ensure blogId is not empty
	if blogId == "" {
		es.log.Error("AchieveAPublishedBlogById: blogId is empty")
		return nil, fmt.Errorf("blogId cannot be empty")
	}

	// Build the update query to set is_archived to true
	updateScript := map[string]interface{}{
		"script": map[string]interface{}{
			"source": "ctx._source.is_archived = true",
		},
	}

	// Marshal the update script to JSON
	bs, err := json.Marshal(updateScript)
	if err != nil {
		es.log.Errorf("AchieveAPublishedBlogById: cannot marshal the update script, error: %v", err)
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
		es.log.Errorf("AchieveAPublishedBlogById: error executing update request, error: %+v", err)
		return updateResponse, err
	}
	defer updateResponse.Body.Close()

	// Check if the response indicates an error
	if updateResponse.IsError() {
		err = fmt.Errorf("AchieveAPublishedBlogById: update query failed, response: %+v", updateResponse)
		es.log.Error(err)
		return updateResponse, err
	}

	es.log.Infof("AchieveAPublishedBlogById: successfully archived blog with id: %s", blogId)
	return updateResponse, nil
}

// DeleteABlogById deletes a blog by its ID
func (es *elasticsearchStorage) DeleteABlogById(ctx context.Context, blogId string) (*esapi.Response, error) {
	// Ensure blogId is not empty
	if blogId == "" {
		es.log.Error("DeleteABlogById: blogId is empty")
		return nil, fmt.Errorf("blogId cannot be empty")
	}

	// Create a Delete request
	req := esapi.DeleteRequest{
		Index:      constants.ElasticsearchBlogIndex,
		DocumentID: blogId,
	}

	// Execute the delete request
	deleteResponse, err := req.Do(ctx, es.client)
	if err != nil {
		es.log.Errorf("DeleteABlogById: error executing delete request, error: %+v", err)
		return deleteResponse, err
	}
	defer deleteResponse.Body.Close()

	// Check if the response indicates an error
	if deleteResponse.IsError() {
		err = fmt.Errorf("DeleteABlogById: delete query failed, response: %+v", deleteResponse)
		es.log.Error(err)
		return deleteResponse, err
	}

	es.log.Infof("DeleteABlogById: successfully deleted blog with id: %s", blogId)
	return deleteResponse, nil
}

// GetLast100BlogsLatestFirst retrieves the last 100 blogs sorted by the latest first
func (es *elasticsearchStorage) GetLast100BlogsLatestFirst(ctx context.Context) (*pb.GetBlogsByTagsNameRes, error) {
	// Build the query to retrieve the last 100 blogs, sorted by the time field in descending order
	query := map[string]interface{}{
		"sort": []map[string]interface{}{
			{
				"blog.time": map[string]string{
					"order": "desc",
				},
			},
		},
		"size": 100,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{
						"term": map[string]interface{}{
							"is_draft": false,
						},
					},
				},
				"must_not": []map[string]interface{}{
					{
						"term": map[string]interface{}{
							"is_archived": true,
						},
					},
				},
				"should": []map[string]interface{}{
					{
						"term": map[string]interface{}{
							"is_archived": false,
						},
					},
					{
						"bool": map[string]interface{}{
							"must_not": map[string]interface{}{
								"exists": map[string]interface{}{
									"field": "is_archived",
								},
							},
						},
					},
				},
				"minimum_should_match": 1,
			},
		},
	}

	// Marshal the query to JSON
	bs, err := json.Marshal(query)
	if err != nil {
		es.log.Errorf("GetLast100BlogsLatestFirst: cannot marshal the query, error: %v", err)
		return nil, err
	}

	// Create a new search request with the query
	req := esapi.SearchRequest{
		Index: []string{constants.ElasticsearchBlogIndex},
		Body:  strings.NewReader(string(bs)),
	}

	// Execute the search request
	res, err := req.Do(ctx, es.client)
	if err != nil {
		es.log.Errorf("GetLast100BlogsLatestFirst: error executing search request, error: %+v", err)
		return nil, err
	}
	defer res.Body.Close()

	// Check if the response indicates an error
	if res.IsError() {
		err = fmt.Errorf("GetLast100BlogsLatestFirst: search query failed, response: %+v", res)
		es.log.Error(err)
		return nil, err
	}

	// Read the response body
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		es.log.Errorf("GetLast100BlogsLatestFirst: error reading response body, error: %v", err)
		return nil, err
	}

	// Parse the response body
	var esResponse map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &esResponse); err != nil {
		es.log.Errorf("GetLast100BlogsLatestFirst: error decoding response body, error: %v", err)
		return nil, err
	}

	// Extract the hits from the response
	hits, ok := esResponse["hits"].(map[string]interface{})["hits"].([]interface{})
	if !ok {
		err := fmt.Errorf("GetLast100BlogsLatestFirst: failed to parse hits from response")
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
			es.log.Errorf("GetLast100BlogsLatestFirst: error marshaling hit source, error: %v", err)
			continue
		}

		var blog pb.GetBlogsByTags
		if err := json.Unmarshal(hitBytes, &blog); err != nil {
			es.log.Errorf("GetLast100BlogsLatestFirst: error unmarshaling hit to GetBlogsByTags, error: %v", err)
			continue
		}
		blogs.TheBlogs = append(blogs.TheBlogs, &blog)
	}

	es.log.Infof("GetLast100BlogsLatestFirst: successfully fetched last 100 blogs sorted by latest first")
	return blogs, nil
}

func (es *elasticsearchStorage) GetDraftedBlogByIdAndOwner(ctx context.Context, blogId, ownerAccountId string) (*pb.BlogByIdRes, error) {
	// Ensure blogId and ownerAccountId are not empty
	if blogId == "" || ownerAccountId == "" {
		es.log.Error("GetDraftedBlogByIdAndOwner: blogId or ownerAccountId is empty")
		return nil, fmt.Errorf("blog id and owner account id cannot be empty")
	}

	// Build the query to search for a drafted blog by blog_id and owner_account_id
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{
						"term": map[string]interface{}{
							"blog_id.keyword": blogId,
						},
					},
					{
						"term": map[string]interface{}{
							"owner_account_id.keyword": ownerAccountId,
						},
					},
					{
						"term": map[string]interface{}{
							"is_draft": true,
						},
					},
				},
				"must_not": []map[string]interface{}{
					{
						"term": map[string]interface{}{
							"is_archived": true,
						},
					},
				},
				"should": []map[string]interface{}{
					{
						"term": map[string]interface{}{
							"is_archived": false,
						},
					},
					{
						"bool": map[string]interface{}{
							"must_not": map[string]interface{}{
								"exists": map[string]interface{}{
									"field": "is_archived",
								},
							},
						},
					},
				},
				"minimum_should_match": 1,
			},
		},
	}

	// Marshal the query to JSON
	bs, err := json.Marshal(query)
	if err != nil {
		es.log.Errorf("GetDraftedBlogByIdAndOwner: cannot marshal the query, error: %v", err)
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
		es.log.Errorf("GetDraftedBlogByIdAndOwner: error executing search request, error: %+v", err)
		return nil, err
	}
	defer res.Body.Close()

	// Check if the response indicates an error
	if res.IsError() {
		err = fmt.Errorf("GetDraftedBlogByIdAndOwner: search query failed, response: %+v", res)
		es.log.Error(err)
		return nil, err
	}

	// Read the response body
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		es.log.Errorf("GetDraftedBlogByIdAndOwner: error reading response body, error: %v", err)
		return nil, err
	}

	// Parse the response body
	var esResponse map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &esResponse); err != nil {
		es.log.Errorf("GetDraftedBlogByIdAndOwner: error decoding response body, error: %v", err)
		return nil, err
	}

	// Extract the hits from the response
	hits, ok := esResponse["hits"].(map[string]interface{})["hits"].([]interface{})
	if !ok || len(hits) == 0 {
		es.log.Infof("GetDraftedBlogByIdAndOwner: no drafted blog found with blogId: %s and ownerAccountId: %s", blogId, ownerAccountId)
		return nil, nil
	}

	// Convert the first hit to GetBlogByIdRes
	hitSource := hits[0].(map[string]interface{})["_source"]
	hitBytes, err := json.Marshal(hitSource)
	if err != nil {
		es.log.Errorf("GetDraftedBlogByIdAndOwner: error marshaling hit source, error: %v", err)
		return nil, err
	}

	var blog pb.BlogByIdRes
	if err := json.Unmarshal(hitBytes, &blog); err != nil {
		es.log.Errorf("GetDraftedBlogByIdAndOwner: error unmarshaling hit to GetBlogByIdRes, error: %v", err)
		return nil, err
	}

	es.log.Infof("GetDraftedBlogByIdAndOwner: successfully fetched drafted blog with blogId: %s and ownerAccountId: %s", blogId, ownerAccountId)
	return &blog, nil
}

func (es *elasticsearchStorage) GetPublishedBlogByIdAndOwner(ctx context.Context, blogId, ownerAccountId string) (*pb.BlogByIdRes, error) {
	// Ensure blogId and ownerAccountId are not empty
	if blogId == "" || ownerAccountId == "" {
		es.log.Error("GetPublishedBlogByIdAndOwner: blogId or ownerAccountId is empty")
		return nil, fmt.Errorf("blog id and owner account id cannot be empty")
	}

	// Build the query to search for a published blog by blog_id and owner_account_id
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{
						"term": map[string]interface{}{
							"blog_id.keyword": blogId,
						},
					},
					{
						"term": map[string]interface{}{
							"owner_account_id.keyword": ownerAccountId,
						},
					},
					{
						"term": map[string]interface{}{
							"is_draft": false,
						},
					},
				},
				"must_not": []map[string]interface{}{
					{
						"term": map[string]interface{}{
							"is_archived": true,
						},
					},
				},
				"should": []map[string]interface{}{
					{
						"term": map[string]interface{}{
							"is_archived": false,
						},
					},
					{
						"bool": map[string]interface{}{
							"must_not": map[string]interface{}{
								"exists": map[string]interface{}{
									"field": "is_archived",
								},
							},
						},
					},
				},
				"minimum_should_match": 1,
			},
		},
	}

	// Marshal the query to JSON
	bs, err := json.Marshal(query)
	if err != nil {
		es.log.Errorf("GetPublishedBlogByIdAndOwner: cannot marshal the query, error: %v", err)
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
		es.log.Errorf("GetPublishedBlogByIdAndOwner: error executing search request, error: %+v", err)
		return nil, err
	}
	defer res.Body.Close()

	// Check if the response indicates an error
	if res.IsError() {
		err = fmt.Errorf("GetPublishedBlogByIdAndOwner: search query failed, response: %+v", res)
		es.log.Error(err)
		return nil, err
	}

	// Read the response body
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		es.log.Errorf("GetPublishedBlogByIdAndOwner: error reading response body, error: %v", err)
		return nil, err
	}

	// Parse the response body
	var esResponse map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &esResponse); err != nil {
		es.log.Errorf("GetPublishedBlogByIdAndOwner: error decoding response body, error: %v", err)
		return nil, err
	}

	// Extract the hits from the response
	hits, ok := esResponse["hits"].(map[string]interface{})["hits"].([]interface{})
	if !ok || len(hits) == 0 {
		es.log.Infof("GetPublishedBlogByIdAndOwner: no published blog found with blogId: %s and ownerAccountId: %s", blogId, ownerAccountId)
		return nil, nil
	}

	// Convert the first hit to GetBlogByIdRes
	hitSource := hits[0].(map[string]interface{})["_source"]
	hitBytes, err := json.Marshal(hitSource)
	if err != nil {
		es.log.Errorf("GetPublishedBlogByIdAndOwner: error marshaling hit source, error: %v", err)
		return nil, err
	}

	var blog pb.BlogByIdRes
	if err := json.Unmarshal(hitBytes, &blog); err != nil {
		es.log.Errorf("GetPublishedBlogByIdAndOwner: error unmarshaling hit to GetBlogByIdRes, error: %v", err)
		return nil, err
	}

	es.log.Infof("GetPublishedBlogByIdAndOwner: successfully fetched published blog with blogId: %s and ownerAccountId: %s", blogId, ownerAccountId)
	return &blog, nil
}

func (es *elasticsearchStorage) GetBlogsByBlogIds(ctx context.Context, blogIds []string) (*pb.GetBlogsRes, error) {
	// Ensure blogIds is not empty
	if len(blogIds) == 0 {
		es.log.Error("GetBlogsByBlogIds: blogIds array is empty")
		return nil, fmt.Errorf("blogIds array cannot be empty")
	}

	// Build the query to search for blogs by blog_id and sort by blog time in descending order (latest first)
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"terms": map[string]interface{}{
				"blog_id.keyword": blogIds,
			},
		},
		"sort": []map[string]interface{}{
			{
				"blog.time": map[string]string{
					"order": "desc",
				},
			},
		},
	}

	// Marshal the query to JSON
	bs, err := json.Marshal(query)
	if err != nil {
		es.log.Errorf("GetBlogsByBlogIds: cannot marshal the query, error: %v", err)
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
		es.log.Errorf("GetBlogsByBlogIds: error executing search request, error: %+v", err)
		return nil, err
	}
	defer res.Body.Close()

	// Check if the response indicates an error
	if res.IsError() {
		err = fmt.Errorf("GetBlogsByBlogIds: search query failed, response: %+v", res)
		es.log.Error(err)
		return nil, err
	}

	// Read the response body
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		es.log.Errorf("GetBlogsByBlogIds: error reading response body, error: %v", err)
		return nil, err
	}

	// Parse the response body
	var esResponse map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &esResponse); err != nil {
		es.log.Errorf("GetBlogsByBlogIds: error decoding response body, error: %v", err)
		return nil, err
	}

	// Extract the hits from the response
	hits, ok := esResponse["hits"].(map[string]interface{})["hits"].([]interface{})
	if !ok || len(hits) == 0 {
		es.log.Infof("GetBlogsByBlogIds: no blogs found for provided blogIds")
		return nil, nil
	}

	// Initialize a response structure to hold multiple blogs
	var blogs = &pb.GetBlogsRes{
		Blogs: make([]*pb.GetBlogs, 0, len(hits)),
	}

	// Iterate over the hits and convert them to GetBlogs
	for _, hit := range hits {
		hitSource := hit.(map[string]interface{})["_source"]
		hitBytes, err := json.Marshal(hitSource)
		if err != nil {
			es.log.Errorf("GetBlogsByBlogIds: error marshaling hit source, error: %v", err)
			return nil, err
		}

		var blog pb.GetBlogs
		if err := json.Unmarshal(hitBytes, &blog); err != nil {
			es.log.Errorf("GetBlogsByBlogIds: error unmarshaling hit to GetBlogs, error: %v", err)
			return nil, err
		}

		blogs.Blogs = append(blogs.Blogs, &blog)
	}

	es.log.Infof("GetBlogsByBlogIds: successfully fetched %d blogs for provided blogIds", len(blogs.Blogs))
	return blogs, nil
}

func (es *elasticsearchStorage) GetDraftBlogByBlogId(ctx context.Context, blogId string) (*pb.BlogByIdRes, error) {
	// Ensure blogId is not empty
	if blogId == "" {
		es.log.Error("GetDraftedBlogById: blogId is empty")
		return nil, fmt.Errorf("blog id cannot be empty")
	}

	// Build the query to search for a drafted blog by blog_id
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{
						"term": map[string]interface{}{
							"blog_id.keyword": blogId,
						},
					},
					{
						"term": map[string]interface{}{
							"is_draft": true,
						},
					},
				},
				"must_not": []map[string]interface{}{
					{
						"term": map[string]interface{}{
							"is_archived": true,
						},
					},
				},
				"should": []map[string]interface{}{
					{
						"term": map[string]interface{}{
							"is_archived": false,
						},
					},
					{
						"bool": map[string]interface{}{
							"must_not": map[string]interface{}{
								"exists": map[string]interface{}{
									"field": "is_archived",
								},
							},
						},
					},
				},
				"minimum_should_match": 1,
			},
		},
	}

	// Marshal the query to JSON
	bs, err := json.Marshal(query)
	if err != nil {
		es.log.Errorf("GetDraftedBlogById: cannot marshal the query, error: %v", err)
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
		es.log.Errorf("GetDraftedBlogById: error executing search request, error: %+v", err)
		return nil, err
	}
	defer res.Body.Close()

	// Check if the response indicates an error
	if res.IsError() {
		err = fmt.Errorf("GetDraftedBlogById: search query failed, response: %+v", res)
		es.log.Error(err)
		return nil, err
	}

	// Read the response body
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		es.log.Errorf("GetDraftedBlogById: error reading response body, error: %v", err)
		return nil, err
	}

	// Parse the response body
	var esResponse map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &esResponse); err != nil {
		es.log.Errorf("GetDraftedBlogById: error decoding response body, error: %v", err)
		return nil, err
	}

	// Extract the hits from the response
	hits, ok := esResponse["hits"].(map[string]interface{})["hits"].([]interface{})
	if !ok || len(hits) == 0 {
		es.log.Infof("GetDraftedBlogById: no drafted blog found with blogId: %s", blogId)
		return nil, nil
	}

	// Convert the first hit to GetBlogByIdRes
	hitSource := hits[0].(map[string]interface{})["_source"]
	hitBytes, err := json.Marshal(hitSource)
	if err != nil {
		es.log.Errorf("GetDraftedBlogById: error marshaling hit source, error: %v", err)
		return nil, err
	}

	var blog pb.BlogByIdRes
	if err := json.Unmarshal(hitBytes, &blog); err != nil {
		es.log.Errorf("GetDraftedBlogById: error unmarshaling hit to GetBlogByIdRes, error: %v", err)
		return nil, err
	}

	es.log.Infof("GetDraftedBlogById: successfully fetched drafted blog with blogId: %s", blogId)
	return &blog, nil
}
