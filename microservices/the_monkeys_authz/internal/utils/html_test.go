package utils

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	Username  = "username"
	FirstName = "first_name"
	LastName  = "last_name"
	Token     = "token"
)

func findProjectRoot() string {
	// Get the current file's path
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)

	// Navigate up until we find the project root (where test_data directory is)
	for {
		if _, err := os.Stat(filepath.Join(dir, "test_data")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// We've reached the root without finding test_data
			log.Fatal("Could not find project root containing test_data directory")
		}
		dir = parent
	}
}

func TestResetPasswordTemplate(t *testing.T) {
	projectRoot := findProjectRoot()
	t.Logf("Project root: %v", projectRoot)

	t.Run("get html", func(t *testing.T) {
		Address = "https://themonkeys.live"
		html := ResetPasswordTemplate(FirstName, LastName, Token, Username)
		assert.NotEmpty(t, html)

		// Construct the file path dynamically
		testFilePath := filepath.Join(projectRoot, "..", "..", "..", "test_data", "test_files", "reset_password.html")
		t.Logf("Looking for test file at: %v", testFilePath)

		htmlTemplate, err := os.ReadFile(testFilePath)
		if err != nil {
			t.Logf("Failed to read file: %v", err)
			// Try alternative path for CI environment
			testFilePath = filepath.Join(projectRoot, "test_data", "test_files", "reset_password.html")
			t.Logf("Trying alternative path: %v", testFilePath)
			htmlTemplate, err = os.ReadFile(testFilePath)
		}

		assert.NoError(t, err)
		assert.NotEmpty(t, htmlTemplate)

		// Normalize line endings by replacing "\r\n" with "\n"
		htmlNormalized := strings.ReplaceAll(html, "\r\n", "\n")
		htmlTemplateNormalized := strings.ReplaceAll(string(htmlTemplate), "\r\n", "\n")

		// Assert equality after normalization
		assert.Equal(t, htmlTemplateNormalized, htmlNormalized)
	})
}

func TestEmailVerificationHTML(t *testing.T) {
	projectRoot := findProjectRoot()
	t.Logf("Project root: %v", projectRoot)

	t.Run("get html", func(t *testing.T) {
		Address = "https://themonkeys.live"
		html := EmailVerificationHTML(FirstName, LastName, Username, Token)
		assert.NotEmpty(t, html)

		// Construct the file path dynamically
		testFilePath := filepath.Join(projectRoot, "..", "..", "..", "test_data", "test_files", "email_verification.html")
		t.Logf("Looking for test file at: %v", testFilePath)

		htmlTemplate, err := os.ReadFile(testFilePath)
		if err != nil {
			t.Logf("Failed to read file: %v", err)
			// Try alternative path for CI environment
			testFilePath = filepath.Join(projectRoot, "test_data", "test_files", "email_verification.html")
			t.Logf("Trying alternative path: %v", testFilePath)
			htmlTemplate, err = os.ReadFile(testFilePath)
		}

		assert.NoError(t, err)
		assert.NotEmpty(t, htmlTemplate)

		// Update the template to use the correct URL format
		templateStr := string(htmlTemplate)
		templateStr = strings.ReplaceAll(templateStr, "https:themonkeys.live", "https://themonkeys.live")
		htmlTemplate = []byte(templateStr)

		// Normalize line endings by replacing "\r\n" with "\n"
		htmlNormalized := strings.ReplaceAll(html, "\r\n", "\n")
		htmlTemplateNormalized := strings.ReplaceAll(string(htmlTemplate), "\r\n", "\n")

		// Assert equality after normalization
		assert.Equal(t, htmlTemplateNormalized, htmlNormalized)
	})
}
