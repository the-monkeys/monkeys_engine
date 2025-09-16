package utils

import (
	"math/rand"

	"github.com/the-monkeys/the_monkeys/logger"
	"golang.org/x/crypto/bcrypt"
)

var (
	alphaNumRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890")
	authzLog      = logger.ZapForService("tm-authz")
)

func HashPassword(password string) string {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		authzLog.Errorf("bcrypt cannot generate hash, error %+v", err)
		return ""
	}
	return string(bytes)
}

func CheckPasswordHash(password string, hash string) bool {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		authzLog.Errorf("bcrypt cannot compare hash, error %+v", err)
		return false
	}
	return true
}

func GenHash() []rune {
	randomHash := make([]rune, 64)
	for i := 0; i < 64; i++ {
		randomHash[i] = alphaNumRunes[rand.Intn(len(alphaNumRunes)-1)]
	}
	return randomHash
}
