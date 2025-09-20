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

func getTagsOrDefault(tags interface{}) []string {
	if tags == nil {
		return []string{}
	}

	switch v := tags.(type) {
	case []string:
		return v
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, t := range v {
			if str, ok := t.(string); ok {
				result = append(result, str)
			}
		}
		return result
	default:
		return []string{}
	}
}

func getStringOrDefault(value interface{}, defaultValue string) string {
	if value == nil {
		return defaultValue
	}
	if str, ok := value.(string); ok {
		return str
	}
	return defaultValue
}

// Returns metadata, total counts of blogs, and errors
func (es *elasticsearchStorage) GetBlogsMetadataByTags(ctx context.Context, tags []string, isDraft bool, limit, offset int32) ([]map[string]interface{}, int, error) {
	// Ensure tags are not empty
	if len(tags) == 0 {
		es.log.Error("GetBlogsMetadataByTags: tags array is empty")
		return nil, 0, fmt.Errorf("tags array cannot be empty")
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
		"_source": []string{
			"blog_id",
			"owner_account_id",
			"blog.blocks", // To extract title, first paragraph, and first image
			"tags",
			"content_type",
			"published_time",
		},
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
		es.log.Errorf("GetBlogsMetadataByTags: cannot marshal the query, error: %v", err)
		return nil, 0, err
	}

	// Create a new search request with the query
	req := esapi.SearchRequest{
		Index: []string{constants.ElasticsearchBlogIndex},
		Body:  strings.NewReader(string(bs)),
	}

	// Execute the search request
	res, err := req.Do(ctx, es.client)
	if err != nil {
		es.log.Errorf("GetBlogsMetadataByTags: error executing search request, error: %+v", err)
		return nil, 0, err
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			es.log.Errorf("GetBlogsMetadataByTags: error closing response body, error: %v", err)
		}
	}()

	// Check if the response indicates an error
	if res.IsError() {
		err = fmt.Errorf("GetBlogsMetadataByTags: search query failed, response: %+v", res)
		es.log.Error(err)
		return nil, 0, err
	}

	// Read the response body
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		es.log.Errorf("GetBlogsMetadataByTags: error reading response body, error: %v", err)
		return nil, 0, err
	}

	// Parse the response body
	var esResponse map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &esResponse); err != nil {
		es.log.Errorf("GetBlogsMetadataByTags: error decoding response body, error: %v", err)
		return nil, 0, err
	}

	// Extract the total count of matching blogs
	totalCount := 0
	if hitsTotal, ok := esResponse["hits"].(map[string]interface{})["total"].(map[string]interface{}); ok {
		if value, exists := hitsTotal["value"].(float64); exists {
			totalCount = int(value)
		}
	}

	// Extract the hits from the response
	hits, ok := esResponse["hits"].(map[string]interface{})["hits"].([]interface{})
	if !ok {
		err := fmt.Errorf("GetBlogsMetadataByTags: failed to parse hits from response")
		es.log.Error(err)
		return nil, totalCount, err
	}

	// Prepare the result
	blogsMetadata := make([]map[string]interface{}, 0, len(hits))
	for _, hit := range hits {
		hitSource := hit.(map[string]interface{})["_source"].(map[string]interface{})
		blogMetadata := map[string]interface{}{
			"blog_id":          hitSource["blog_id"],
			"owner_account_id": hitSource["owner_account_id"],
			"tags":             hitSource["tags"],
			"content_type":     hitSource["content_type"],
			"published_time":   hitSource["published_time"],
		}

		// Extract title, first paragraph, and first image from blog.blocks
		blocks, ok := hitSource["blog"].(map[string]interface{})["blocks"].([]interface{})
		if !ok {
			es.log.Errorf("GetBlogsMetadataByTags: failed to parse blog blocks")
			continue
		}

		var title, firstParagraph, firstImage string
		for _, block := range blocks {
			blockMap := block.(map[string]interface{})
			blockType := blockMap["type"].(string)
			blockData := blockMap["data"].(map[string]interface{})

			switch blockType {
			case "header":
				if title == "" && blockData["level"].(float64) == 1 {
					title = blockData["text"].(string)
				}
			case "paragraph":
				if firstParagraph == "" {
					firstParagraph = blockData["text"].(string)
				}
			case "image":
				if firstImage == "" {
					firstImage = blockData["file"].(map[string]interface{})["url"].(string)
				}
			}
		}

		blogMetadata["title"] = title
		blogMetadata["first_paragraph"] = firstParagraph
		blogMetadata["first_image"] = firstImage

		blogsMetadata = append(blogsMetadata, blogMetadata)
	}

	es.log.Debugf("GetBlogsMetadataByTags: successfully fetched %d blogs metadata out of %d total", len(blogsMetadata), totalCount)
	return blogsMetadata, totalCount, nil
}

func (es *elasticsearchStorage) GetAllPublishedBlogsMetadata(ctx context.Context, limit, offset int) ([]map[string]interface{}, int, error) {
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
		"_source": []string{
			"blog_id",
			"owner_account_id",
			"blog.blocks", // To extract title, first paragraph, and first image
			"tags",
			"content_type",
			"published_time",
		},
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
		es.log.Errorf("GetAllPublishedBlogsMetadata: cannot marshal the query, error: %v", err)
		return nil, 0, err
	}

	req := esapi.SearchRequest{
		Index: []string{constants.ElasticsearchBlogIndex},
		Body:  strings.NewReader(string(bs)),
	}

	res, err := req.Do(ctx, es.client)
	if err != nil {
		es.log.Errorf("GetAllPublishedBlogsMetadata: error executing search request, error: %+v", err)
		return nil, 0, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			es.log.Errorf("GetAllPublishedBlogsMetadata: error closing response body, error: %v", err)
		}
	}()

	if res.IsError() {
		err = fmt.Errorf("GetAllPublishedBlogsMetadata: search query failed, response: %+v", res)
		es.log.Error(err)
		return nil, 0, err
	}

	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		es.log.Errorf("GetAllPublishedBlogsMetadata: error reading response body, error: %v", err)
		return nil, 0, err
	}

	var esResponse map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &esResponse); err != nil {
		es.log.Errorf("GetAllPublishedBlogsMetadata: error decoding response body, error: %v", err)
		return nil, 0, err
	}

	totalCount := 0
	if hitsTotal, ok := esResponse["hits"].(map[string]interface{})["total"].(map[string]interface{}); ok {
		if value, exists := hitsTotal["value"].(float64); exists {
			totalCount = int(value)
		}
	}

	hits, ok := esResponse["hits"].(map[string]interface{})["hits"].([]interface{})
	if !ok {
		err := fmt.Errorf("GetAllPublishedBlogsMetadata: failed to parse hits from response")
		es.log.Error(err)
		return nil, totalCount, err
	}

	blogsMetadata := make([]map[string]interface{}, 0, len(hits))
	for _, hit := range hits {
		hitSource := hit.(map[string]interface{})["_source"].(map[string]interface{})
		blogMetadata := map[string]interface{}{
			"blog_id":          hitSource["blog_id"],
			"owner_account_id": hitSource["owner_account_id"],
			"tags":             hitSource["tags"],
			"content_type":     hitSource["content_type"],
			"published_time":   hitSource["published_time"],
		}

		blocks, ok := hitSource["blog"].(map[string]interface{})["blocks"].([]interface{})
		if !ok {
			es.log.Errorf("GetAllPublishedBlogsMetadata: failed to parse blog blocks")
			continue
		}

		var title, firstParagraph, firstImage string
		for _, block := range blocks {
			blockMap := block.(map[string]interface{})
			blockType := blockMap["type"].(string)
			blockData := blockMap["data"].(map[string]interface{})

			switch blockType {
			case "header":
				if title == "" && blockData["level"].(float64) == 1 {
					title = blockData["text"].(string)
				}
			case "paragraph":
				if firstParagraph == "" {
					firstParagraph = blockData["text"].(string)
				}
			case "image":
				if firstImage == "" {
					firstImage = blockData["file"].(map[string]interface{})["url"].(string)
				}
			}
		}

		blogMetadata["title"] = title
		blogMetadata["first_paragraph"] = firstParagraph
		blogMetadata["first_image"] = firstImage

		blogsMetadata = append(blogsMetadata, blogMetadata)
	}

	es.log.Debugf("GetAllPublishedBlogsMetadata: successfully fetched %d blogs metadata out of %d total", len(blogsMetadata), totalCount)
	return blogsMetadata, totalCount, nil
}

func (es *elasticsearchStorage) GetBlogsMetadataByQuery(ctx context.Context, queryTexts []string, isDraft bool, limit, offset int32) ([]map[string]interface{}, int, error) {
	if len(queryTexts) == 0 {
		es.log.Error("GetBlogsMetadataByQuery: queryTexts array is empty")
		return nil, 0, fmt.Errorf("queryTexts array cannot be empty")
	}

	query := map[string]interface{}{
		"from": offset,
		"size": limit,
		"sort": []map[string]interface{}{
			{
				"published_time": map[string]interface{}{
					"order":         "desc",
					"unmapped_type": "date",
				},
			},
			{
				"blog.time": map[string]string{
					"order": "desc",
				},
			},
		},
		"_source": []string{
			"blog_id",
			"owner_account_id",
			"blog.blocks",
			"tags",
			"content_type",
			"published_time",
		},
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{
						"bool": map[string]interface{}{
							"should": func() []map[string]interface{} {
								should := make([]map[string]interface{}, 0)
								for _, queryText := range queryTexts {
									// Add match on blog content
									should = append(should, map[string]interface{}{
										"match": map[string]interface{}{
											"blog.blocks.data.text": map[string]interface{}{
												"query": queryText,
												"boost": 2.0,
											},
										},
									})
									// Add match on tags
									should = append(should, map[string]interface{}{
										"match": map[string]interface{}{
											"tags": map[string]interface{}{
												"query": queryText,
												"boost": 2.5,
											},
										},
									})
									// Add phrase match on blog content
									should = append(should, map[string]interface{}{
										"match_phrase": map[string]interface{}{
											"blog.blocks.data.text": map[string]interface{}{
												"query": queryText,
												"boost": 3.0,
												"slop":  3,
											},
										},
									})
								}
								return should
							}(),
							"minimum_should_match": 1,
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

	bs, err := json.Marshal(query)
	if err != nil {
		es.log.Errorf("GetBlogsMetadataByQuery: failed to marshal query, error: %v", err)
		return nil, 0, err
	}

	req := esapi.SearchRequest{
		Index: []string{constants.ElasticsearchBlogIndex},
		Body:  strings.NewReader(string(bs)),
	}

	res, err := req.Do(ctx, es.client)
	if err != nil {
		es.log.Errorf("GetBlogsMetadataByQuery: search request failed, error: %+v", err)
		return nil, 0, err
	}
	defer res.Body.Close()

	if res.IsError() {
		err = fmt.Errorf("GetBlogsMetadataByQuery: Elasticsearch returned an error: %+v", res)
		es.log.Error(err)
		return nil, 0, err
	}

	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		es.log.Errorf("GetBlogsMetadataByQuery: error reading response body, error: %v", err)
		return nil, 0, err
	}

	var esResponse map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &esResponse); err != nil {
		es.log.Errorf("GetBlogsMetadataByQuery: error decoding response body, error: %v", err)
		return nil, 0, err
	}

	totalCount := 0
	if hitsTotal, ok := esResponse["hits"].(map[string]interface{})["total"].(map[string]interface{}); ok {
		if value, exists := hitsTotal["value"].(float64); exists {
			totalCount = int(value)
		}
	}

	hits, ok := esResponse["hits"].(map[string]interface{})["hits"].([]interface{})
	if !ok {
		err := fmt.Errorf("GetBlogsMetadataByQuery: failed to parse hits from response")
		es.log.Error(err)
		return nil, totalCount, err
	}

	blogsMetadata := make([]map[string]interface{}, 0, len(hits))
	for _, hit := range hits {
		hitSource := hit.(map[string]interface{})["_source"].(map[string]interface{})
		blogMetadata := map[string]interface{}{
			"blog_id":          hitSource["blog_id"],
			"owner_account_id": hitSource["owner_account_id"],
			"tags":             getTagsOrDefault(hitSource["tags"]),
			"content_type":     getStringOrDefault(hitSource["content_type"], "editor"),
			"published_time":   hitSource["published_time"],
			"bookmark_count":   0,
			"like_count":       0,
		}

		blogMetadata["title"] = "" // Initialize with empty string
		blogMetadata["first_paragraph"] = ""
		blogMetadata["first_image"] = ""

		// Try to extract content from blocks if available
		if blog, ok := hitSource["blog"].(map[string]interface{}); ok {
			if blocks, ok := blog["blocks"].([]interface{}); ok {
				for _, b := range blocks {
					if block, ok := b.(map[string]interface{}); ok {
						if t, ok := block["type"].(string); ok {
							if data, ok := block["data"].(map[string]interface{}); ok {
								switch t {
								case "header":
									if blogMetadata["title"] == "" {
										if level, ok := data["level"].(float64); ok && level == 1 {
											if text, ok := data["text"].(string); ok {
												blogMetadata["title"] = text
											}
										}
									}
								case "paragraph":
									if blogMetadata["first_paragraph"] == "" {
										if text, ok := data["text"].(string); ok {
											blogMetadata["first_paragraph"] = text
										}
									}
								case "image":
									if blogMetadata["first_image"] == "" {
										if file, ok := data["file"].(map[string]interface{}); ok {
											if url, ok := file["url"].(string); ok {
												blogMetadata["first_image"] = url
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}

		blogsMetadata = append(blogsMetadata, blogMetadata)
	}

	es.log.Debugf("GetBlogsMetadataByQuery: fetched %d blogs out of %d total", len(blogsMetadata), totalCount)
	return blogsMetadata, totalCount, nil
}

func (es *elasticsearchStorage) GetBlogsMetaByAccountId(ctx context.Context, accountId string, isDraft bool, limit, offset int32) ([]map[string]interface{}, int, error) {
	// Validate input
	if accountId == "" {
		es.log.Error("GetBlogsMetaByAccountId: accountId cannot be empty")
		return nil, 0, fmt.Errorf("accountId cannot be empty")
	}

	// Build the query to get blogs by account ID with sorting by latest first
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
		"_source": []string{
			"blog_id",
			"owner_account_id",
			"blog.blocks", // To extract title, first paragraph, and first image
			"tags",
			"content_type",
			"published_time",
		},
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
		es.log.Errorf("GetBlogsMetaByAccountId: cannot marshal the query, error: %v", err)
		return nil, 0, err
	}

	// Create a new search request with the query
	req := esapi.SearchRequest{
		Index: []string{constants.ElasticsearchBlogIndex},
		Body:  strings.NewReader(string(bs)),
	}

	// Execute the search request
	res, err := req.Do(ctx, es.client)
	if err != nil {
		es.log.Errorf("GetBlogsMetaByAccountId: error executing search request, error: %+v", err)
		return nil, 0, err
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			es.log.Errorf("GetBlogsMetaByAccountId: error closing response body, error: %v", err)
		}
	}()

	// Check if the response indicates an error
	if res.IsError() {
		err = fmt.Errorf("GetBlogsMetaByAccountId: search query failed, response: %+v", res)
		es.log.Error(err)
		return nil, 0, err
	}

	// Read the response body
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		es.log.Errorf("GetBlogsMetaByAccountId: error reading response body, error: %v", err)
		return nil, 0, err
	}

	// Parse the response body
	var esResponse map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &esResponse); err != nil {
		es.log.Errorf("GetBlogsMetaByAccountId: error decoding response body, error: %v", err)
		return nil, 0, err
	}

	// Extract the total count of matching blogs
	totalCount := 0
	if hitsTotal, ok := esResponse["hits"].(map[string]interface{})["total"].(map[string]interface{}); ok {
		if value, exists := hitsTotal["value"].(float64); exists {
			totalCount = int(value)
		}
	}

	// Extract the hits from the response
	hits, ok := esResponse["hits"].(map[string]interface{})["hits"].([]interface{})
	if !ok {
		err := fmt.Errorf("GetBlogsMetaByAccountId: failed to parse hits from response")
		es.log.Error(err)
		return nil, totalCount, err
	}

	// Prepare the result
	blogsMetadata := make([]map[string]interface{}, 0, len(hits))
	for _, hit := range hits {
		hitSource := hit.(map[string]interface{})["_source"].(map[string]interface{})
		blogMetadata := map[string]interface{}{
			"blog_id":          hitSource["blog_id"],
			"owner_account_id": hitSource["owner_account_id"],
			"tags":             getTagsOrDefault(hitSource["tags"]),
			"content_type":     getStringOrDefault(hitSource["content_type"], "editorjs"),
			"published_time":   hitSource["published_time"],
		}

		// Initialize metadata fields
		blogMetadata["title"] = ""
		blogMetadata["first_paragraph"] = ""
		blogMetadata["first_image"] = ""

		// Extract title, first paragraph, and first image from blog.blocks
		if blog, ok := hitSource["blog"].(map[string]interface{}); ok {
			if blocks, ok := blog["blocks"].([]interface{}); ok {
				for _, b := range blocks {
					if block, ok := b.(map[string]interface{}); ok {
						if blockType, ok := block["type"].(string); ok {
							if blockData, ok := block["data"].(map[string]interface{}); ok {
								switch blockType {
								case "header":
									if blogMetadata["title"] == "" {
										if level, ok := blockData["level"].(float64); ok && level == 1 {
											if text, ok := blockData["text"].(string); ok {
												blogMetadata["title"] = text
											}
										}
									}
								case "paragraph":
									if blogMetadata["first_paragraph"] == "" {
										if text, ok := blockData["text"].(string); ok {
											blogMetadata["first_paragraph"] = text
										}
									}
								case "image":
									if blogMetadata["first_image"] == "" {
										if file, ok := blockData["file"].(map[string]interface{}); ok {
											if url, ok := file["url"].(string); ok {
												blogMetadata["first_image"] = url
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}

		blogsMetadata = append(blogsMetadata, blogMetadata)
	}

	es.log.Debugf("GetBlogsMetaByAccountId: successfully fetched %d blogs metadata out of %d total for account %s", len(blogsMetadata), totalCount, accountId)
	return blogsMetadata, totalCount, nil
}

func (es *elasticsearchStorage) GetBlogsMetaByBlogIdsV2(ctx context.Context, blogIds []string, isDraft bool, limit, offset int32) ([]map[string]interface{}, int, error) {
	// Validate input
	if len(blogIds) == 0 {
		es.log.Error("GetBlogsMetaByBlogIdsV2: blogIds array cannot be empty")
		return nil, 0, fmt.Errorf("blogIds array cannot be empty")
	}

	// Build the query to get blogs by blog IDs with sorting by latest first
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
		"_source": []string{
			"blog_id",
			"owner_account_id",
			"blog.blocks", // To extract title, first paragraph, and first image
			"tags",
			"content_type",
			"published_time",
		},
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{
						"terms": map[string]interface{}{
							"blog_id.keyword": blogIds,
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
		es.log.Errorf("GetBlogsMetaByBlogIdsV2: cannot marshal the query, error: %v", err)
		return nil, 0, err
	}

	// Create a new search request with the query
	req := esapi.SearchRequest{
		Index: []string{constants.ElasticsearchBlogIndex},
		Body:  strings.NewReader(string(bs)),
	}

	// Execute the search request
	res, err := req.Do(ctx, es.client)
	if err != nil {
		es.log.Errorf("GetBlogsMetaByBlogIdsV2: error executing search request, error: %+v", err)
		return nil, 0, err
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			es.log.Errorf("GetBlogsMetaByBlogIdsV2: error closing response body, error: %v", err)
		}
	}()

	// Check if the response indicates an error
	if res.IsError() {
		err = fmt.Errorf("GetBlogsMetaByBlogIdsV2: search query failed, response: %+v", res)
		es.log.Error(err)
		return nil, 0, err
	}

	// Read the response body
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		es.log.Errorf("GetBlogsMetaByBlogIdsV2: error reading response body, error: %v", err)
		return nil, 0, err
	}

	// Parse the response body
	var esResponse map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &esResponse); err != nil {
		es.log.Errorf("GetBlogsMetaByBlogIdsV2: error decoding response body, error: %v", err)
		return nil, 0, err
	}

	// Extract the total count of matching blogs
	totalCount := 0
	if hitsTotal, ok := esResponse["hits"].(map[string]interface{})["total"].(map[string]interface{}); ok {
		if value, exists := hitsTotal["value"].(float64); exists {
			totalCount = int(value)
		}
	}

	// Extract the hits from the response
	hits, ok := esResponse["hits"].(map[string]interface{})["hits"].([]interface{})
	if !ok {
		err := fmt.Errorf("GetBlogsMetaByBlogIdsV2: failed to parse hits from response")
		es.log.Error(err)
		return nil, totalCount, err
	}

	// Prepare the result
	blogsMetadata := make([]map[string]interface{}, 0, len(hits))
	for _, hit := range hits {
		hitSource := hit.(map[string]interface{})["_source"].(map[string]interface{})
		blogMetadata := map[string]interface{}{
			"blog_id":          hitSource["blog_id"],
			"owner_account_id": hitSource["owner_account_id"],
			"tags":             getTagsOrDefault(hitSource["tags"]),
			"content_type":     getStringOrDefault(hitSource["content_type"], "editorjs"),
			"published_time":   hitSource["published_time"],
		}

		// Initialize metadata fields
		blogMetadata["title"] = ""
		blogMetadata["first_paragraph"] = ""
		blogMetadata["first_image"] = ""

		// Extract title, first paragraph, and first image from blog.blocks
		if blog, ok := hitSource["blog"].(map[string]interface{}); ok {
			if blocks, ok := blog["blocks"].([]interface{}); ok {
				for _, b := range blocks {
					if block, ok := b.(map[string]interface{}); ok {
						if blockType, ok := block["type"].(string); ok {
							if blockData, ok := block["data"].(map[string]interface{}); ok {
								switch blockType {
								case "header":
									if blogMetadata["title"] == "" {
										if level, ok := blockData["level"].(float64); ok && level == 1 {
											if text, ok := blockData["text"].(string); ok {
												blogMetadata["title"] = text
											}
										}
									}
								case "paragraph":
									if blogMetadata["first_paragraph"] == "" {
										if text, ok := blockData["text"].(string); ok {
											blogMetadata["first_paragraph"] = text
										}
									}
								case "image":
									if blogMetadata["first_image"] == "" {
										if file, ok := blockData["file"].(map[string]interface{}); ok {
											if url, ok := file["url"].(string); ok {
												blogMetadata["first_image"] = url
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}

		blogsMetadata = append(blogsMetadata, blogMetadata)
	}

	es.log.Debugf("GetBlogsMetaByBlogIdsV2: successfully fetched %d blogs metadata out of %d total for %d blog IDs", len(blogsMetadata), totalCount, len(blogIds))
	return blogsMetadata, totalCount, nil
}
