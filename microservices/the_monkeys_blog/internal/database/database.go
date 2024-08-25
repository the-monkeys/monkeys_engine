package database

import (
	"context"
	"crypto/tls"
	"net/http"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/sirupsen/logrus"
)

func NewESClient(url, username, password string) (*elasticsearch.Client, error) {
	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{url},
		Username:  username,
		Password:  password,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // Disable SSL certificate verification (for testing)
			},
		},
	})
	if err != nil {
		return nil, err
	}

	// Perform a simple operation to check the connection
	req := esapi.PingRequest{}
	res, err := req.Do(context.Background(), client)
	if err != nil || res.IsError() {
		return nil, err
	}
	defer res.Body.Close()

	logrus.Infof("✅ Elasticsearch connection established successfully")
	return client, nil
}
