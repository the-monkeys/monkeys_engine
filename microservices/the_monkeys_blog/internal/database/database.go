package database

import (
	"context"
	"crypto/tls"
	"net/http"

	"github.com/opensearch-project/opensearch-go"
	"github.com/opensearch-project/opensearch-go/opensearchapi"
	"github.com/sirupsen/logrus"
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

	// Perform a simple operation to check the connection
	req := opensearchapi.PingRequest{}
	res, err := req.Do(context.Background(), client)
	if err != nil || res.IsError() {
		return nil, err
	}

	logrus.Infof("✅ Opensearch connection established successfully")
	return client, nil
}
