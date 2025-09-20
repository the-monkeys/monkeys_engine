package utils

import (
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_authz/pb"
	"github.com/the-monkeys/the_monkeys/constants"
)

func PublicIP() string {
	resp, err := http.Get("https://ifconfig.co/ip")
	if err != nil {
		panic(err)
	}
	// defer resp.Body.Close()
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			fmt.Println("Error closing response body:", closeErr)
		}
	}()

	ip, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	fmt.Println("Public IP address:", string(ip))
	return string(ip)
}

func GetUUID() string {
	uuid := uuid.New()
	id := uuid.ID()

	return strconv.Itoa(int(id))
}

func RandomString(n int) string {
	s, r := make([]rune, n), []rune(alphaNumRunes)

	for i := range s {
		p, _ := rand.Prime(rand.Reader, len(r))
		x, y := p.Uint64(), uint64(len(r))
		s[i] = r[x%y]
	}

	return string(s)
}

func ValidateRegisterUserRequest(req *pb.RegisterUserRequest) error {
	if req.Email == "" || req.FirstName == "" || req.Password == "" {
		return fmt.Errorf("incomplete information: email, first name and password are required")
	}

	// Check for disposable email
	if IsDisposableEmail(req.Email) {
		return fmt.Errorf("disposable email addresses are not allowed")
	}

	return nil
}

// IsDisposableEmail checks if the email domain is a known disposable email provider
func IsDisposableEmail(email string) bool {
	// Extract domain from email
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false // Invalid email format
	}

	domain := strings.ToLower(strings.TrimSpace(parts[1]))

	// Check against our disposable domains list
	return constants.DisposableEmailDomains[domain]
}

func IpClientConvert(ip, client string) (string, string) {
	if ip == "" {
		ip = "127.0.0.1"
	}

	for _, cli := range constants.Clients {
		if client == cli {
			return ip, client
		}
	}

	client = "Others"

	return ip, client
}

// Function to generate a GUID-like username
func GenerateGUID() string {
	// Get current time in UnixNano format
	timestamp := time.Now().UnixNano()

	// Generate a random byte slice
	randomBytes := make([]byte, 8)
	_, err := rand.Read(randomBytes) // Uses crypto/rand for secure random bytes
	if err != nil {
		panic(err)
	}

	return fmt.Sprintf("%x%x", randomBytes, timestamp)
}

// IsRestrictedUsername checks if a username is in the restricted list
func IsRestrictedUsername(username string) bool {
	lowerUsername := strings.ToLower(username)
	for _, restricted := range constants.RestrictedUsernames {
		if lowerUsername == strings.ToLower(restricted) {
			return true
		}
	}
	return false
}

// Function to shuffle a string
//func shuffleString(s string) string {
//	// Convert string to a slice of runes (to handle Unicode characters properly)
//	runes := []rune(s)
//
//	// Seed the random number generator
//	r := mathRand.New(mathRand.NewSource(time.Now().UnixNano())) // Uses math/rand for shuffling
//
//	// Shuffle the slice of runes
//	r.Shuffle(len(runes), func(i, j int) {
//		runes[i], runes[j] = runes[j], runes[i]
//	})
//
//	// Convert the runes slice back to a string and return
//	return string(runes)
//}
