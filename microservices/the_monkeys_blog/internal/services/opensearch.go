package services

// import (
// 	"context"
// 	"encoding/json"
// 	"strings"

// 	"github.com/opensearch-project/opensearch-go"
// 	"github.com/opensearch-project/opensearch-go/opensearchapi"
// 	"github.com/sirupsen/logrus"
// 	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_blog/internal/database"
// 	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_blog/internal/models"
// 	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_blog/internal/utils"
// )

// type osClient struct {
// 	client *opensearch.Client
// 	log    *logrus.Logger
// }

// func NewOpenSearchClient(url, username, password string, log *logrus.Logger) (*osClient, error) {
// 	client, err := database.NewOSClient(url, username, password)
// 	if err != nil {
// 		logrus.Errorf("Failed to connect to opensearch instance, error: %+v", err)
// 		return nil, err
// 	}

// 	return &osClient{
// 		client: client,
// 		log:    log,
// 	}, nil
// }

// // TODO: Implement MapArticleIndex function
// // MapArticleIndex setups the mapping considering database schema and search factors.
// func (oso *osClient) MapArticleIndex(mapping string) error {
// 	return nil
// }

// // CreateAnArticle creates a document for an article posted by a user
// func (oso *osClient) CreateAnArticle(article models.Blogs) (*opensearchapi.Response, error) {
// 	oso.log.Infof("received an article with id: %s", article.Id)

// 	bs, err := json.Marshal(article)
// 	if err != nil {
// 		oso.log.Errorf("cannot marshal the article, error: %v", err)
// 		return nil, err
// 	}

// 	document := strings.NewReader(string(bs))

// 	osReq := opensearchapi.IndexRequest{
// 		Index:      utils.OpensearchArticleIndex,
// 		DocumentID: article.Id,
// 		Body:       document,
// 	}

// 	insertResponse, err := osReq.Do(context.Background(), oso.client)
// 	if err != nil {
// 		oso.log.Errorf("error while creating/drafting article, error: %+v", err)
// 		return insertResponse, err
// 	}

// 	if insertResponse.IsError() {
// 		oso.log.Errorf("error creating an article, insert response: %+v", insertResponse)
// 		return insertResponse, err
// 	}

// 	oso.log.Infof("successfully created an article for user: %s, insert response: %+v", article.AuthorId, insertResponse)
// 	return insertResponse, nil
// }

// // GetLast100Articles gets us last 100 articles created
// func (oso *osClient) GetLast100Articles() (*opensearchapi.Response, error) {
// 	oso.log.Infof("getting last 100 articles")

// 	// Search for the document.
// 	content := strings.NewReader(getLast100Articles())

// 	search := opensearchapi.SearchRequest{
// 		Index: []string{utils.OpensearchArticleIndex},
// 		Body:  content,
// 	}

// 	searchResponse, err := search.Do(context.Background(), oso.client)
// 	if err != nil {
// 		oso.log.Errorf("failed to search document, error: %+v", err)
// 		return nil, err
// 	}

// 	if searchResponse.IsError() {
// 		oso.log.Errorf("error fetching 100 articles, search response: %+v", searchResponse)
// 		return searchResponse, err
// 	}

// 	return searchResponse, nil
// }

// // GetArticleById gets us an articles matching the id
// func (oso *osClient) GetArticleById(ctx context.Context, id string) (*opensearchapi.Response, error) {
// 	oso.log.Infof("getting the article: %v", id)

// 	content := strings.NewReader(getArticleById(id))

// 	search := opensearchapi.SearchRequest{
// 		Index:      []string{utils.OpensearchArticleIndex},
// 		Body:       content,
// 		ErrorTrace: true,
// 	}
// 	b := false
// 	search.AllowPartialSearchResults = &b
// 	searchResponse, err := search.Do(ctx, oso.client)
// 	if err != nil {
// 		oso.log.Errorf("failed to find document, error: %+v", err)
// 		return nil, err
// 	}

// 	return searchResponse, nil
// }

// // GetLast100Articles gets us last 100 articles created
// func (oso *osClient) GetLast100ArticlesByTag(tag string) (*opensearchapi.Response, error) {
// 	oso.log.Infof("getting last 100 articles by tag: %s", tag)

// 	// Search for the document.
// 	content := strings.NewReader(getLast100ArticlesByTag(tag))

// 	search := opensearchapi.SearchRequest{
// 		Index: []string{utils.OpensearchArticleIndex},
// 		Body:  content,
// 	}

// 	searchResponse, err := search.Do(context.Background(), oso.client)
// 	if err != nil {
// 		oso.log.Errorf("failed to search document, error: %+v", err)
// 		return nil, err
// 	}

// 	if searchResponse.IsError() {
// 		oso.log.Errorf("error fetching 100 articles, search response: %+v", searchResponse)
// 		return searchResponse, err
// 	}

// 	return searchResponse, nil
// }

// // CreateAnArticle creates a document for an article posted by a user
// func (oso *osClient) CreateABlog(article models.BlogsService) (*opensearchapi.Response, error) {
// 	oso.log.Infof("received an article with id: %s", article.Id)

// 	bs, err := json.Marshal(article)
// 	if err != nil {
// 		oso.log.Errorf("cannot marshal the article, error: %v", err)
// 		return nil, err
// 	}

// 	document := strings.NewReader(string(bs))

// 	osReq := opensearchapi.IndexRequest{
// 		Index:      utils.OpensearchArticleIndex,
// 		DocumentID: article.Id,
// 		Body:       document,
// 	}

// 	insertResponse, err := osReq.Do(context.Background(), oso.client)
// 	if err != nil {
// 		oso.log.Errorf("error while creating/drafting article, error: %+v", err)
// 		return insertResponse, err
// 	}

// 	if insertResponse.IsError() {
// 		oso.log.Errorf("error creating an article, insert response: %+v", insertResponse)
// 		return insertResponse, err
// 	}

// 	oso.log.Infof("successfully created an article for user: %s, insert response: %+v", article.AuthorEmail, insertResponse)
// 	return insertResponse, nil
// }
