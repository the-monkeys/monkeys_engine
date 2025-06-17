package main

import (
	"database/sql"
	"log"
	"time"

	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
)

const (
	postgresDSN         = "user=user password=password dbname=dbname host=localhost port=5432 sslmode=disable"
	openSearchAppName   = "your-app-name"
	openSearchIndexName = "your-index-name"
)

var db *sql.DB

func main() {
	// Connect to PostgreSQL
	var err error
	db, err = sql.Open("postgres", postgresDSN)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			logrus.Errorf("failed to close PostgreSQL connection: %v", err)
		}
	}()

	// Connect to OpenSearch
	// client, err = opensearch.NewClient(openSearchAppName)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// Listen for PostgreSQL notifications
	_, err = db.Exec("LISTEN sync")
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for {
			// Wait for PostgreSQL notifications
			_, err := db.Exec("CREATE TRIGGER users_notify_trigger AFTER INSERT OR UPDATE OR DELETE ON the_monkeys_user FOR EACH ROW EXECUTE FUNCTION notify_user_changes();")
			if err != nil {
				log.Println(err)
			}

			// Synchronize data from PostgreSQL to OpenSearch
			if err := sync(); err != nil {
				log.Println(err)
			}
		}
	}()

	// Perform hourly consistency check
	ticker := time.NewTicker(time.Hour)
	for range ticker.C {
		// Synchronize data from PostgreSQL to OpenSearch
		if err := sync(); err != nil {
			log.Println(err)
		}
	}

}
func sync() error {
	logrus.Infoln("**********************************************************************88")
	return nil
}

// func sync() error {
// 	rows, err := db.Query("SELECT id, title, content FROM articles")
// 	if err != nil {
// 		return err
// 	}
// 	defer rows.Close()

// 	var articles []opensearch.Document
// 	for rows.Next() {
// 		var id int
// 		var title, content string
// 		if err := rows.Scan(&id, &title, &content); err != nil {
// 			return err
// 		}

// 		articles = append(articles, opensearch.Document{
// 			Fields: []opensearch.Field{
// 				{
// 					Name:  "id",
// 					Value: id,
// 				},
// 				{
// 					Name:  "title",
// 					Value: title,
// 				},
// 				{
// 					Name:  "content",
// 					Value: content,
// 				},
// 			},
// 		})
// 	}

// 	return client.Push(openSearchIndexName, articles)
// }
