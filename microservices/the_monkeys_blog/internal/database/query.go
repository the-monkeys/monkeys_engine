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

func (es *elasticsearchStorage) DeleteBlogsByOwnerAccountID(ctx context.Context, ownerAccountId string) (*esapi.Response, error) {
	// Ensure ownerAccountId is not empty
	if ownerAccountId == "" {
		es.log.Error("DeleteBlogsByOwnerAccountID: ownerAccountId is empty")
		return nil, fmt.Errorf("owner account id cannot be empty")
	}

	// Build the query to search for blogs by owner_account_id
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				"owner_account_id.keyword": ownerAccountId,
			},
		},
	}

	// Marshal the query to JSON
	bs, err := json.Marshal(query)
	if err != nil {
		es.log.Errorf("DeleteBlogsByOwnerAccountID: cannot marshal the query, error: %v", err)
		return nil, err
	}

	// Print the query for debugging
	es.log.Infof("Executing query: %s", string(bs))

	// Create a new search request to find all blogs for the user
	searchReq := esapi.SearchRequest{
		Index: []string{constants.ElasticsearchBlogIndex},
		Body:  strings.NewReader(string(bs)),
	}

	// Execute the search request
	searchRes, err := searchReq.Do(ctx, es.client)
	if err != nil {
		es.log.Errorf("DeleteBlogsByOwnerAccountID: error executing search request, error: %+v", err)
		return nil, err
	}
	defer searchRes.Body.Close()

	// Check if the response indicates an error
	if searchRes.IsError() {
		err = fmt.Errorf("DeleteBlogsByOwnerAccountID: search query failed, response: %+v", searchRes)
		es.log.Error(err)
		return nil, err
	}

	// Read the search response body
	searchBodyBytes, err := io.ReadAll(searchRes.Body)
	if err != nil {
		es.log.Errorf("DeleteBlogsByOwnerAccountID: error reading search response body, error: %v", err)
		return nil, err
	}

	// Parse the search response body
	var esSearchResponse map[string]interface{}
	if err := json.Unmarshal(searchBodyBytes, &esSearchResponse); err != nil {
		es.log.Errorf("DeleteBlogsByOwnerAccountID: error decoding search response body, error: %v", err)
		return nil, err
	}

	// Extract the hits (blogs) from the search response
	hits, ok := esSearchResponse["hits"].(map[string]interface{})["hits"].([]interface{})
	if !ok || len(hits) == 0 {
		es.log.Infof("DeleteBlogsByOwnerAccountID: no blogs found for ownerAccountId: %s", ownerAccountId)
		return nil, nil
	}

	// Prepare a bulk delete request for all the found blogs
	var bulkBody strings.Builder
	for _, hit := range hits {
		blogId := hit.(map[string]interface{})["_id"].(string)

		// Add a delete operation for each blog in the bulk request
		meta := map[string]interface{}{
			"delete": map[string]interface{}{
				"_index": constants.ElasticsearchBlogIndex,
				"_id":    blogId,
			},
		}
		metaBytes, err := json.Marshal(meta)
		if err != nil {
			es.log.Errorf("DeleteBlogsByOwnerAccountID: error marshaling delete meta for blogId: %s, error: %v", blogId, err)
			return nil, err
		}

		bulkBody.Write(metaBytes)
		bulkBody.WriteString("\n")
	}

	// Execute the bulk delete request
	bulkReq := esapi.BulkRequest{
		Index: constants.ElasticsearchBlogIndex,
		Body:  strings.NewReader(bulkBody.String()),
	}

	// Perform the bulk delete operation
	bulkRes, err := bulkReq.Do(ctx, es.client)
	if err != nil {
		es.log.Errorf("DeleteBlogsByOwnerAccountID: error executing bulk delete request, error: %+v", err)
		return nil, err
	}
	defer bulkRes.Body.Close()

	// Check if the bulk delete response indicates an error
	if bulkRes.IsError() {
		err = fmt.Errorf("DeleteBlogsByOwnerAccountID: bulk delete failed, response: %+v", bulkRes)
		es.log.Error(err)
		return nil, err
	}

	es.log.Infof("DeleteBlogsByOwnerAccountID: successfully deleted blogs for ownerAccountId: %s", ownerAccountId)
	return bulkRes, nil
}
