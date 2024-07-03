package utils

import (
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_authz/pb"
	"github.com/the-monkeys/the_monkeys/constants"
)

func PublicIP() string {
	resp, err := http.Get("https://ifconfig.co/ip")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

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
		return fmt.Errorf("incomplete information: email, first name, last name and password are required")
	}
	return nil
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
