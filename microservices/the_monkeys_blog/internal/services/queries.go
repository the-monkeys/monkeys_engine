package services

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

// getLast100Articles basically picks recent 100 published articles skipping the drafts
func getLast100Articles() string {
	return `{
		"size": 100,
		"sort": {
			"create_time": {
				"order": "desc"
			}
		},
		"query": {
			"match": {
				"published": "true"
			}
		},
		"_source": {
			"includes": [
				"id",
				"title",
				"content_raw",
				"author_name",
				"author_id",
				"create_time"
			]
		}
	}`
}

func getArticleById(id string) string {
	return fmt.Sprintf(`{
		"query": {
			"match": {
				"id": "%s"
			}
		}
	}`, id)
}

func updateArticleById(id, title, content string, tags []string) string {
	bx, err := json.Marshal(tags)
	if err != nil {
		logrus.Errorf("cannot marshal tags, error %v", err)
	}
	cont, err := json.Marshal(content)
	if err != nil {
		logrus.Errorf("cannot marshal content, error %v", err)
	}

	return fmt.Sprintf(`{
			"query": {
				"match": {
					"id": "%s"
				}
			},
			"script": {
				"source": "ctx._source.title = params.title; ctx._source.content = params.content; ctx._source.tags = params.tags; ctx._source.update_time = params.update_time",
				"lang": "painless",
				"params": {
					"title": "%s",
					"content": %s,
					"tags": %v,
					"update_time": "%v"
				}
			}
		}`, id, title, string(cont), string(bx), time.Now().Format("2006-01-02T15:04:05Z07:00"))
}

// getLast100Articles basically picks recent 100 published articles skipping the drafts
func getLast100ArticlesByTag(tag string) string {
	return fmt.Sprintf(`{
		"size": 100,
		"sort": {
			"create_time": {
				"order": "desc"
			}
		},
		"query": {
			"term": {
				"tags": "%s"
			}
		}
	}`, tag)
}
