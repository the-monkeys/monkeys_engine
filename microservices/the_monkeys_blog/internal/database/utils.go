package database

import "fmt"

func getBlogById(id string) string {
	return fmt.Sprintf(`{
		"query": {
			"match": {
				"id": "%s"
			}
		}
	}`, id)
}
