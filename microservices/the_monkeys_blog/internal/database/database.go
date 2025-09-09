package database

import (
	"context"
	"crypto/tls"
	"net/http"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"go.uber.org/zap"
)

func NewESClient(url, username, password string, log *zap.SugaredLogger) (*elasticsearch.Client, error) {
	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{url},
		Username:  username,
		Password:  password,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // Disable SSL certificate verification (for testing)
			},
			MaxIdleConnsPerHost:   10, // Set the maximum number of idle connections per host
			MaxIdleConns:          10, // Set the maximum number of idle connections
			IdleConnTimeout:       90, // Set the maximum amount of time an idle connection will remain idle before closing itself
			TLSHandshakeTimeout:   10, // Set the maximum amount of time waiting to wait for a TLS handshake
			ExpectContinueTimeout: 1,  // Set the maximum amount of time to wait for an HTTP/1.1 100-continue response
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
	defer func() {
		if err := res.Body.Close(); err != nil {
			log.Error("Error closing response body:", err)
		}
	}()

	log.Infof("✅ Elasticsearch connection established successfully")
	return client, nil
}
