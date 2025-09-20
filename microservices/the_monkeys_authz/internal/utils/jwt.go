package utils

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_authz/internal/models"
)

type JwtWrapper struct {
	SecretKey       string
	Issuer          string
	ExpirationHours int64
}

type jwtClaims struct {
	jwt.StandardClaims
	AccountId               string `json:"account_id"`
	Email                   string `json:"email"`
	EmailVerificationStatus string `json:"email_verified"`
	Username                string `json:"username"`
	ClientId                string `json:"client_id"`
	Client                  string `json:"client"`
	IpAddress               string `json:"ip"`
}

// TODO: Add Username, profile_name and client_id
func (w *JwtWrapper) GenerateToken(user *models.TheMonkeysUser) (signedToken string, err error) {
	claims := &jwtClaims{
		AccountId:               user.AccountId,
		Email:                   user.Email,
		EmailVerificationStatus: user.EmailVerificationStatus,
		Username:                user.Username,
		ClientId:                user.ClientId,
		Client:                  user.Client,
		IpAddress:               user.IpAddress,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Local().Add(time.Hour * time.Duration(w.ExpirationHours)).Unix(),
			Issuer:    w.Issuer,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signedToken, err = token.SignedString([]byte(w.SecretKey))

	if err != nil {
		return "", err
	}

	return signedToken, nil
}

func (w *JwtWrapper) ValidateToken(signedToken string) (claims *jwtClaims, err error) {
	token, err := jwt.ParseWithClaims(
		signedToken,
		&jwtClaims{},
		func(token *jwt.Token) (interface{}, error) {
			return []byte(w.SecretKey), nil
		},
	)
	if err != nil {
		authzLog.Errorf("cannot parse with claims the json token, error: %v", err)
		return
	}

	claims, ok := token.Claims.(*jwtClaims)
	if !ok {
		authzLog.Errorf("cannot parse jwt claims, error: %v", err)
		return nil, errors.New("couldn't parse the claims")
	}

	if claims.ExpiresAt < time.Now().Local().Unix() {
		authzLog.Errorf("the token expired already, error: %v", err)
		return nil, errors.New("the token is expired")
	}

	return claims, nil
}
