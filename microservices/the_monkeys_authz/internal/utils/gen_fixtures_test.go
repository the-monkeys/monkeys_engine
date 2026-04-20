package utils

import (
	"os"
	"path/filepath"
	"testing"
)

// Run with: go test -run TestGenFixtures -tags=genfixtures ./microservices/the_monkeys_authz/internal/utils/
func TestGenFixtures(t *testing.T) {
	if os.Getenv("GEN_FIXTURES") != "1" {
		t.Skip("set GEN_FIXTURES=1 to regenerate test fixtures")
	}
	Address = "https://themonkeys.live"
	root := findProjectRoot()

	rp := ResetPasswordTemplate(FirstName, LastName, Token, Username)
	ev := EmailVerificationHTML(FirstName, LastName, Username, Token)

	if err := os.WriteFile(filepath.Join(root, "test_data", "test_files", "reset_password.html"), []byte(rp), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "test_data", "test_files", "email_verification.html"), []byte(ev), 0644); err != nil {
		t.Fatal(err)
	}
}
