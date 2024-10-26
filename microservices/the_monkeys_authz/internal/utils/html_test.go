package utils

import (
	"fmt"
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

func TestResetPasswordTemplate(t *testing.T) {
	_, filename, _, _ := runtime.Caller(1)
	fmt.Printf("filename: %v\n", filename)

	// Print the current working directory
	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Current working directory: %v\n", dir)

	// Navigate up to the desired directory
	baseDir := dir
	for !strings.HasSuffix(baseDir, "the_monkeys_engine") {
		baseDir = filepath.Dir(baseDir)
		if baseDir == filepath.Dir(baseDir) {
			log.Fatal("Base directory not found")
		}
	}
	fmt.Printf("Base directory: %v\n", baseDir)

	t.Run("get html", func(t *testing.T) {
		Address = "https://themonkeys.live"
		html := ResetPasswordTemplate(FirstName, LastName, Token, Username)
		assert.NotEmpty(t, html)

		// Construct the file path dynamically
		testFilePath := filepath.Join(baseDir, "test_data", "test_files", "reset_password.html")
		htmlTemplate, err := os.ReadFile(testFilePath)

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
	_, filename, _, _ := runtime.Caller(1)
	fmt.Printf("filename: %v\n", filename)

	// Print the current working directory
	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Current working directory: %v\n", dir)

	// Navigate up to the desired directory
	baseDir := dir
	for !strings.HasSuffix(baseDir, "the_monkeys_engine") {
		baseDir = filepath.Dir(baseDir)
		if baseDir == filepath.Dir(baseDir) {
			log.Fatal("Base directory not found")
		}
	}
	fmt.Printf("Base directory: %v\n", baseDir)

	t.Run("get html", func(t *testing.T) {
		Address = "https:themonkeys.live"
		html := EmailVerificationHTML(FirstName, LastName, Username, Token)
		assert.NotEmpty(t, html)

		// Construct the file path dynamically
		testFilePath := filepath.Join(baseDir, "test_data", "test_files", "email_verification.html")
		htmlTemplate, err := os.ReadFile(testFilePath)
		assert.NoError(t, err)
		assert.NotEmpty(t, htmlTemplate)

		// Normalize line endings by replacing "\r\n" with "\n"
		htmlNormalized := strings.ReplaceAll(html, "\r\n", "\n")
		htmlTemplateNormalized := strings.ReplaceAll(string(htmlTemplate), "\r\n", "\n")

		// Assert equality after normalization
		assert.Equal(t, htmlTemplateNormalized, htmlNormalized)
	})
}
