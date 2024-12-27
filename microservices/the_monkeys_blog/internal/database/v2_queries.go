package database

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_blog/internal/constants"
)

func (es *elasticsearchStorage) SaveBlog(ctx context.Context, blog map[string]interface{}) (*esapi.Response, error) {
	bs, err := json.Marshal(blog)
	if err != nil {
		es.log.Errorf("DraftABlog: cannot marshal the blog, error: %v", err)
		return nil, err
	}

	document := strings.NewReader(string(bs))

	blogId := blog["blog_id"].(string)
	ownerAccountId := blog["owner_account_id"].(string)
	req := esapi.IndexRequest{
		Index:      constants.ElasticsearchBlogIndex,
		DocumentID: blogId,
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

	es.log.Infof("DraftABlog: successfully created blog for user: %s, response: %+v", ownerAccountId, insertResponse)
	return insertResponse, nil
}

func (es *elasticsearchStorage) GetBlogsOfUsersByAccountIds(ctx context.Context, accountIds []string, limit, offset int32) ([]map[string]interface{}, error) {
	// Ensure accountIds is not empty
	if len(accountIds) == 0 {
		es.log.Error("GetBlogsOfUsersByAccountIds: accountIds array is empty")
		return nil, fmt.Errorf("accountIds array cannot be empty")
	}

	// Build the query to get published blogs, not archived, owned by specific account IDs
	query := map[string]interface{}{
		"sort": []map[string]interface{}{
			{
				"published_time": map[string]string{
					"order": "desc",
				},
			},
		},
		"from": offset,
		"size": limit,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{
						"terms": map[string]interface{}{
							"owner_account_id.keyword": accountIds,
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
							"is_archived": false,
						},
					},
				},
			},
		},
	}

	// Marshal the query to JSON
	bs, err := json.Marshal(query)
	if err != nil {
		es.log.Errorf("GetBlogsOfUsersByAccountIds: cannot marshal the query, error: %v", err)
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
		es.log.Errorf("GetBlogsOfUsersByAccountIds: error executing search request, error: %+v", err)
		return nil, err
	}
	defer res.Body.Close()

	// Check if the response indicates an error
	if res.IsError() {
		err = fmt.Errorf("GetBlogsOfUsersByAccountIds: search query failed, response: %+v", res)
		es.log.Error(err)
		return nil, err
	}

	// Read the response body
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		es.log.Errorf("GetBlogsOfUsersByAccountIds: error reading response body, error: %v", err)
		return nil, err
	}

	// Parse the response body
	var esResponse map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &esResponse); err != nil {
		es.log.Errorf("GetBlogsOfUsersByAccountIds: error decoding response body, error: %v", err)
		return nil, err
	}

	// Extract the hits from the response
	hits, ok := esResponse["hits"].(map[string]interface{})["hits"].([]interface{})
	if !ok {
		err := fmt.Errorf("GetBlogsOfUsersByAccountIds: failed to parse hits from response")
		es.log.Error(err)
		return nil, err
	}

	// Prepare the result
	blogs := make([]map[string]interface{}, 0, len(hits))
	for _, hit := range hits {
		hitSource := hit.(map[string]interface{})["_source"]
		blog, ok := hitSource.(map[string]interface{})
		if !ok {
			es.log.Errorf("GetBlogsOfUsersByAccountIds: failed to cast hit source to map")
			continue
		}
		blogs = append(blogs, blog)
	}

	es.log.Infof("GetBlogsOfUsersByAccountIds: successfully fetched %d blogs", len(blogs))
	return blogs, nil
}

func (es *elasticsearchStorage) GetBlogsByTagsAccId(ctx context.Context, accountId string, tags []string, isDraft bool, limit, offset int32) ([]map[string]interface{}, error) {
	// Ensure accountId and tags are not empty
	if accountId == "" {
		es.log.Error("GetBlogsByTags: accountId is empty")
		return nil, fmt.Errorf("accountId cannot be empty")
	}
	if len(tags) == 0 {
		es.log.Error("GetBlogsByTags: tags array is empty")
		return nil, fmt.Errorf("tags array cannot be empty")
	}

	// Build the query to get blogs by tags with sorting by latest first
	query := map[string]interface{}{
		"sort": []map[string]interface{}{
			{
				"blog.time": map[string]string{
					"order": "desc",
				},
			},
		},
		"from": offset,
		"size": limit,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{
						"term": map[string]interface{}{
							"owner_account_id.keyword": accountId,
						},
					},
					{
						"terms": map[string]interface{}{
							"tags.keyword": tags,
						},
					},
					{
						"term": map[string]interface{}{
							"is_draft": isDraft,
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
			},
		},
	}

	// Marshal the query to JSON
	bs, err := json.Marshal(query)
	if err != nil {
		es.log.Errorf("GetBlogsByTags: cannot marshal the query, error: %v", err)
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
		es.log.Errorf("GetBlogsByTags: error executing search request, error: %+v", err)
		return nil, err
	}
	defer res.Body.Close()

	// Check if the response indicates an error
	if res.IsError() {
		err = fmt.Errorf("GetBlogsByTags: search query failed, response: %+v", res)
		es.log.Error(err)
		return nil, err
	}

	// Read the response body
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		es.log.Errorf("GetBlogsByTags: error reading response body, error: %v", err)
		return nil, err
	}

	// Parse the response body
	var esResponse map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &esResponse); err != nil {
		es.log.Errorf("GetBlogsByTags: error decoding response body, error: %v", err)
		return nil, err
	}

	// Extract the hits from the response
	hits, ok := esResponse["hits"].(map[string]interface{})["hits"].([]interface{})
	if !ok {
		err := fmt.Errorf("GetBlogsByTags: failed to parse hits from response")
		es.log.Error(err)
		return nil, err
	}

	// Prepare the result
	blogs := make([]map[string]interface{}, 0, len(hits))
	for _, hit := range hits {
		hitSource := hit.(map[string]interface{})["_source"]
		blog, ok := hitSource.(map[string]interface{})
		if !ok {
			es.log.Errorf("GetBlogsByTags: failed to cast hit source to map")
			continue
		}
		blogs = append(blogs, blog)
	}

	es.log.Infof("GetBlogsByTags: successfully fetched %d blogs", len(blogs))
	return blogs, nil
}

func (es *elasticsearchStorage) GetBlogsByAccountId(ctx context.Context, accountId string, isDraft bool, limit, offset int32) ([]map[string]interface{}, error) {
	// Ensure accountId is not empty
	if accountId == "" {
		es.log.Error("GetBlogsByAccountId: accountId is empty")
		return nil, fmt.Errorf("accountId cannot be empty")
	}

	// Build the query to get blogs by accountId, filtered by isDraft, with sorting by latest first
	query := map[string]interface{}{
		"sort": []map[string]interface{}{
			{
				"blog.time": map[string]string{
					"order": "desc",
				},
			},
		},
		"from": offset,
		"size": limit,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{
						"term": map[string]interface{}{
							"owner_account_id.keyword": accountId,
						},
					},
					{
						"term": map[string]interface{}{
							"is_draft": isDraft,
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
			},
		},
	}

	// Marshal the query to JSON
	bs, err := json.Marshal(query)
	if err != nil {
		es.log.Errorf("GetBlogsByAccountId: cannot marshal the query, error: %v", err)
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
		es.log.Errorf("GetBlogsByAccountId: error executing search request, error: %+v", err)
		return nil, err
	}
	defer res.Body.Close()

	// Check if the response indicates an error
	if res.IsError() {
		err = fmt.Errorf("GetBlogsByAccountId: search query failed, response: %+v", res)
		es.log.Error(err)
		return nil, err
	}

	// Read the response body
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		es.log.Errorf("GetBlogsByAccountId: error reading response body, error: %v", err)
		return nil, err
	}

	// Parse the response body
	var esResponse map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &esResponse); err != nil {
		es.log.Errorf("GetBlogsByAccountId: error decoding response body, error: %v", err)
		return nil, err
	}

	// Extract the hits from the response
	hits, ok := esResponse["hits"].(map[string]interface{})["hits"].([]interface{})
	if !ok {
		err := fmt.Errorf("GetBlogsByAccountId: failed to parse hits from response")
		es.log.Error(err)
		return nil, err
	}

	// Prepare the result
	blogs := make([]map[string]interface{}, 0, len(hits))
	for _, hit := range hits {
		hitSource := hit.(map[string]interface{})["_source"]
		blog, ok := hitSource.(map[string]interface{})
		if !ok {
			es.log.Errorf("GetBlogsByAccountId: failed to cast hit source to map")
			continue
		}
		blogs = append(blogs, blog)
	}

	es.log.Infof("GetBlogsByAccountId: successfully fetched %d blogs", len(blogs))
	return blogs, nil
}

func (es *elasticsearchStorage) GetBlogByBlogId(ctx context.Context, blogId string, isDraft bool) (map[string]interface{}, error) {
	// Ensure blogId is not empty
	if blogId == "" {
		es.log.Error("GetBlogByBlogId: blogId is empty")
		return nil, fmt.Errorf("blogId cannot be empty")
	}

	// Build the query to fetch the blog by blogId
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
							"is_draft": isDraft,
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
			},
		},
	}

	// Marshal the query to JSON
	bs, err := json.Marshal(query)
	if err != nil {
		es.log.Errorf("GetBlogByBlogId: cannot marshal the query, error: %v", err)
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
		es.log.Errorf("GetBlogByBlogId: error executing search request, error: %+v", err)
		return nil, err
	}
	defer res.Body.Close()

	// Check if the response indicates an error
	if res.IsError() {
		err = fmt.Errorf("GetBlogByBlogId: search query failed, response: %+v", res)
		es.log.Error(err)
		return nil, err
	}

	// Read the response body
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		es.log.Errorf("GetBlogByBlogId: error reading response body, error: %v", err)
		return nil, err
	}

	// Parse the response body
	var esResponse map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &esResponse); err != nil {
		es.log.Errorf("GetBlogByBlogId: error decoding response body, error: %v", err)
		return nil, err
	}

	// Extract the hits from the response
	hits, ok := esResponse["hits"].(map[string]interface{})["hits"].([]interface{})
	if !ok || len(hits) == 0 {
		es.log.Infof("GetBlogByBlogId: no blog found with blogId: %s", blogId)
		return nil, nil
	}

	// Return the first blog
	hitSource := hits[0].(map[string]interface{})["_source"]
	blog, ok := hitSource.(map[string]interface{})
	if !ok {
		es.log.Errorf("GetBlogByBlogId: failed to cast hit source to map")
		return nil, fmt.Errorf("failed to cast hit source to map")
	}

	es.log.Infof("GetBlogByBlogId: successfully fetched blog with blogId: %s", blogId)
	return blog, nil
}

func (es *elasticsearchStorage) GetABlogByBlogIdAccId(ctx context.Context, blogId, accountId string, isDraft bool) (map[string]interface{}, error) {
	// Ensure blogId and accountId are not empty
	if blogId == "" {
		es.log.Error("GetABlogByBlogIdAccId: blogId is empty")
		return nil, fmt.Errorf("blogId cannot be empty")
	}
	if accountId == "" {
		es.log.Error("GetABlogByBlogIdAccId: accountId is empty")
		return nil, fmt.Errorf("accountId cannot be empty")
	}

	// Build the query to fetch the blog by blogId and accountId
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
							"owner_account_id.keyword": accountId,
						},
					},
					{
						"term": map[string]interface{}{
							"is_draft": isDraft,
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
			},
		},
	}

	// Marshal the query to JSON
	bs, err := json.Marshal(query)
	if err != nil {
		es.log.Errorf("GetABlogByBlogIdAccId: cannot marshal the query, error: %v", err)
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
		es.log.Errorf("GetABlogByBlogIdAccId: error executing search request, error: %+v", err)
		return nil, err
	}
	defer res.Body.Close()

	// Check if the response indicates an error
	if res.IsError() {
		err = fmt.Errorf("GetABlogByBlogIdAccId: search query failed, response: %+v", res)
		es.log.Error(err)
		return nil, err
	}

	// Read the response body
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		es.log.Errorf("GetABlogByBlogIdAccId: error reading response body, error: %v", err)
		return nil, err
	}

	// Parse the response body
	var esResponse map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &esResponse); err != nil {
		es.log.Errorf("GetABlogByBlogIdAccId: error decoding response body, error: %v", err)
		return nil, err
	}

	// Extract the hits from the response
	hits, ok := esResponse["hits"].(map[string]interface{})["hits"].([]interface{})
	if !ok || len(hits) == 0 {
		es.log.Infof("GetABlogByBlogIdAccId: no blog found with blogId: %s and accountId: %s", blogId, accountId)
		return nil, nil
	}

	// Return the first blog
	hitSource := hits[0].(map[string]interface{})["_source"]
	blog, ok := hitSource.(map[string]interface{})
	if !ok {
		es.log.Errorf("GetABlogByBlogIdAccId: failed to cast hit source to map")
		return nil, fmt.Errorf("failed to cast hit source to map")
	}

	es.log.Infof("GetABlogByBlogIdAccId: successfully fetched blog with blogId: %s and accountId: %s", blogId, accountId)
	return blog, nil
}

func (es *elasticsearchStorage) GetBlogsByTags(ctx context.Context, tags []string, isDraft bool, limit, offset int32) ([]map[string]interface{}, error) {
	// Ensure tags are not empty
	if len(tags) == 0 {
		es.log.Error("GetBlogsByTagsWithoutAccId: tags array is empty")
		return nil, fmt.Errorf("tags array cannot be empty")
	}

	// Build the query to get blogs by tags with sorting by latest first
	query := map[string]interface{}{
		"sort": []map[string]interface{}{
			{
				"blog.time": map[string]string{
					"order": "desc",
				},
			},
		},
		"from": offset,
		"size": limit,
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
							"is_draft": isDraft,
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
			},
		},
	}

	// Marshal the query to JSON
	bs, err := json.Marshal(query)
	if err != nil {
		es.log.Errorf("GetBlogsByTagsWithoutAccId: cannot marshal the query, error: %v", err)
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
		es.log.Errorf("GetBlogsByTagsWithoutAccId: error executing search request, error: %+v", err)
		return nil, err
	}
	defer res.Body.Close()

	// Check if the response indicates an error
	if res.IsError() {
		err = fmt.Errorf("GetBlogsByTagsWithoutAccId: search query failed, response: %+v", res)
		es.log.Error(err)
		return nil, err
	}

	// Read the response body
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		es.log.Errorf("GetBlogsByTagsWithoutAccId: error reading response body, error: %v", err)
		return nil, err
	}

	// Parse the response body
	var esResponse map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &esResponse); err != nil {
		es.log.Errorf("GetBlogsByTagsWithoutAccId: error decoding response body, error: %v", err)
		return nil, err
	}

	// Extract the hits from the response
	hits, ok := esResponse["hits"].(map[string]interface{})["hits"].([]interface{})
	if !ok {
		err := fmt.Errorf("GetBlogsByTagsWithoutAccId: failed to parse hits from response")
		es.log.Error(err)
		return nil, err
	}

	// Prepare the result
	blogs := make([]map[string]interface{}, 0, len(hits))
	for _, hit := range hits {
		hitSource := hit.(map[string]interface{})["_source"]
		blog, ok := hitSource.(map[string]interface{})
		if !ok {
			es.log.Errorf("GetBlogsByTagsWithoutAccId: failed to cast hit source to map")
			continue
		}
		blogs = append(blogs, blog)
	}

	es.log.Infof("GetBlogsByTagsWithoutAccId: successfully fetched %d blogs", len(blogs))
	return blogs, nil
}

func (es *elasticsearchStorage) GetBlogsByBlogIdsV2(ctx context.Context, blogIds []string, limit, offset int32) ([]map[string]interface{}, error) {
	// Ensure blogIds is not empty
	if len(blogIds) == 0 {
		es.log.Error("GetBlogsByBlogIds: blogIds array is empty")
		return nil, fmt.Errorf("blogIds array cannot be empty")
	}

	// Build the query to get blogs by blogIds with sorting by latest first
	query := map[string]interface{}{
		"sort": []map[string]interface{}{
			{
				"blog.time": map[string]string{
					"order": "desc",
				},
			},
		},
		"from": offset,
		"size": limit,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{
						"terms": map[string]interface{}{
							"blog_id.keyword": blogIds,
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
			},
		},
	}

	// Marshal the query to JSON
	bs, err := json.Marshal(query)
	if err != nil {
		es.log.Errorf("GetBlogsByBlogIds: cannot marshal the query, error: %v", err)
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
	if !ok {
		err := fmt.Errorf("GetBlogsByBlogIds: failed to parse hits from response")
		es.log.Error(err)
		return nil, err
	}

	// Prepare the result
	blogs := make([]map[string]interface{}, 0, len(hits))
	for _, hit := range hits {
		hitSource := hit.(map[string]interface{})["_source"]
		blog, ok := hitSource.(map[string]interface{})
		if !ok {
			es.log.Errorf("GetBlogsByBlogIds: failed to cast hit source to map")
			continue
		}
		blogs = append(blogs, blog)
	}

	es.log.Infof("GetBlogsByBlogIds: successfully fetched %d blogs", len(blogs))
	return blogs, nil
}

func (es *elasticsearchStorage) GetAllPublishedBlogsLatestFirst(ctx context.Context, limit, offset int) ([]map[string]interface{}, error) {
	// Build the query to get all published blogs with sorting by published_time or blog.time as fallback
	query := map[string]interface{}{
		"sort": []map[string]interface{}{
			{
				"published_time": map[string]interface{}{
					"order":         "desc",
					"unmapped_type": "date", // Handle cases where published_time is missing
				},
			},
			{
				"blog.time": map[string]string{
					"order": "desc",
				},
			},
		},
		"from": offset,
		"size": limit,
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
			},
		},
	}

	bs, err := json.Marshal(query)
	if err != nil {
		es.log.Errorf("GetAllPublishedBlogsLatestFirst: cannot marshal the query, error: %v", err)
		return nil, err
	}

	req := esapi.SearchRequest{
		Index: []string{constants.ElasticsearchBlogIndex},
		Body:  strings.NewReader(string(bs)),
	}

	res, err := req.Do(ctx, es.client)
	if err != nil {
		es.log.Errorf("GetAllPublishedBlogsLatestFirst: error executing search request, error: %+v", err)
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		err = fmt.Errorf("GetAllPublishedBlogsLatestFirst: search query failed, response: %+v", res)
		es.log.Error(err)
		return nil, err
	}

	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		es.log.Errorf("GetAllPublishedBlogsLatestFirst: error reading response body, error: %v", err)
		return nil, err
	}

	var esResponse map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &esResponse); err != nil {
		es.log.Errorf("GetAllPublishedBlogsLatestFirst: error decoding response body, error: %v", err)
		return nil, err
	}

	hits, ok := esResponse["hits"].(map[string]interface{})["hits"].([]interface{})
	if !ok {
		err := fmt.Errorf("GetAllPublishedBlogsLatestFirst: failed to parse hits from response")
		es.log.Error(err)
		return nil, err
	}

	blogs := make([]map[string]interface{}, 0, len(hits))
	for _, hit := range hits {
		hitSource := hit.(map[string]interface{})["_source"]
		blog, ok := hitSource.(map[string]interface{})
		if !ok {
			es.log.Errorf("GetAllPublishedBlogsLatestFirst: failed to cast hit source to map")
			continue
		}
		blogs = append(blogs, blog)
	}

	es.log.Infof("GetAllPublishedBlogsLatestFirst: successfully fetched %d blogs", len(blogs))
	return blogs, nil
}
