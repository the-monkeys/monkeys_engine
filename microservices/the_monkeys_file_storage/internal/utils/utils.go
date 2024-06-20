package utils

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"

	"github.com/sirupsen/logrus"
)

func RemoveSpecialChar(val string) string {
	// Define regular expression to match all special characters
	reg, err := regexp.Compile("[^a-zA-Z0-9\\.]+")
	if err != nil {
		logrus.Error("cannot compile regexp")
	}

	// Remove special characters from the input string
	output := reg.ReplaceAllString(val, "")

	return output
}

func ConstructPath(basePath, blogId, fileName string) (string, string) {
	return filepath.Join(basePath, blogId), filepath.Join(basePath, blogId, fileName)
}

func ReadImageFromURL(url string) ([]byte, error) {
	// Create a new HTTP client
	client := http.Client{}

	// Send a GET request to the URL
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Check for successful response status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status code %d", resp.StatusCode)
	}

	// Read the response body into a byte array
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Error reading response body: %v", err)
	}

	return data, nil
}
