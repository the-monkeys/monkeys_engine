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

	blogId := blog["BlogId"].(string)
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
