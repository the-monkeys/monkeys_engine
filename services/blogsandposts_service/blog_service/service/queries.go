package service

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"
)

const (

	// getArticlesByTags picks articles based on the tag name, latest first
	getArticlesByTags = `{
		"size": 100,
		"query": {
			"match": {
				"is_draft": "false"
			}
		},
		"_source": {
			"includes": [
				"id",
				"title",
				"author",
				"create_time",
				"quick_read"
			]
		}
	}`
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
				"is_draft": "false"
			}
		},
		"_source": {
			"includes": [
				"id",
				"title",
				"author",
				"create_time",
				"quick_read",
				"author_email"
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

	}
	cont, err := json.Marshal(content)
	if err != nil {

	}
	ioutil.WriteFile("abc.json", bx, 777)
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
