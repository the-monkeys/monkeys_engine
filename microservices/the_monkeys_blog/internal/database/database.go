package database

import (
	"crypto/tls"
	"net/http"

	"github.com/opensearch-project/opensearch-go"
)

func NewOSClient(url, username, password string) (*opensearch.Client, error) {
	client, err := opensearch.NewClient(opensearch.Config{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Addresses: []string{url},
		Username:  username, // For testing only. Don't store credentials in code.
		Password:  password,
	})

	if err != nil {
		return nil, err
	}

	// ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	// defer cancel()

	// // Perform a simple operation to check the connection
	// res, err := client.Ping(ctx, nil)
	// if err != nil || res.IsError() {
	// 	return nil, err
	// }

	return client, nil
}
