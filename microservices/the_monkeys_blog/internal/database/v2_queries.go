package database

import (
	"context"
	"encoding/json"
	"fmt"
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
