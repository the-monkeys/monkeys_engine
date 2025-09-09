package main

import (
	"database/sql"
	"time"

	_ "github.com/lib/pq"
	"github.com/the-monkeys/the_monkeys/logger"
)

const (
	postgresDSN         = "user=user password=password dbname=dbname host=localhost port=5432 sslmode=disable"
	openSearchAppName   = "your-app-name"
	openSearchIndexName = "your-index-name"
)

var (
	// global logger for this microservice
	log = logger.ZapForService("tm_pg")
	// db handle
	db *sql.DB
)

func main() {
	// Connect to PostgreSQL
	var err error
	db, err = sql.Open("postgres", postgresDSN)
	if err != nil {
		log.Fatalf("failed to open postgres: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Errorf("failed to close PostgreSQL connection: %v", err)
		}
	}()

	// Connect to OpenSearch (commented out placeholder)
	// client, err = opensearch.NewClient(openSearchAppName)
	// if err != nil {
	// 	log.Fatalf("failed to init opensearch client: %v", err)
	// }

	// Listen for PostgreSQL notifications
	_, err = db.Exec("LISTEN sync")
	if err != nil {
		log.Fatalf("failed to LISTEN sync channel: %v", err)
	}

	go func() {
		for {
			// Example trigger creation each loop (original behavior preserved)
			_, err := db.Exec("CREATE TRIGGER users_notify_trigger AFTER INSERT OR UPDATE OR DELETE ON the_monkeys_user FOR EACH ROW EXECUTE FUNCTION notify_user_changes();")
			if err != nil {
				log.Errorf("failed to create trigger: %v", err)
			}

			// Synchronize data from PostgreSQL to OpenSearch
			if err := sync(); err != nil {
				log.Errorf("sync error: %v", err)
			}
		}
	}()

	// Perform hourly consistency check
	ticker := time.NewTicker(time.Hour)
	for range ticker.C {
		if err := sync(); err != nil {
			log.Errorf("hourly sync error: %v", err)
		}
	}
}

func sync() error {
	log.Debug("sync placeholder execution")
	return nil
}

// Original full sync implementation retained below (commented)
// func sync() error {
// 	rows, err := db.Query("SELECT id, title, content FROM articles")
// 	if err != nil {
// 		return err
// 	}
// 	defer rows.Close()
//
// 	var articles []opensearch.Document
// 	for rows.Next() {
// 		var id int
// 		var title, content string
// 		if err := rows.Scan(&id, &title, &content); err != nil {
// 			return err
// 		}
//
// 		articles = append(articles, opensearch.Document{
// 			Fields: []opensearch.Field{
// 				{Name: "id", Value: id},
// 				{Name: "title", Value: title},
// 				{Name: "content", Value: content},
// 			},
// 		})
// 	}
//
// 	return client.Push(openSearchIndexName, articles)
// }
