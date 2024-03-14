package utils

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_authz/internal/models"
)

type JwtWrapper struct {
	SecretKey       string
	Issuer          string
	ExpirationHours int64
}

type jwtClaims struct {
	jwt.StandardClaims
	AccountId string
	Email     string
	Username  string
	ClientId  string
	Client    string
	IpAddress string
}

// TODO: Add Username, profile_name and client_id
func (w *JwtWrapper) GenerateToken(user *models.TheMonkeysUser) (signedToken string, err error) {
	claims := &jwtClaims{
		AccountId: user.AccountId,
		Email:     user.Email,
		Username:  user.Username,
		ClientId:  user.ClientId,
		Client:    user.Client,
		IpAddress: user.IpAddress,
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
		logrus.Errorf("cannot parse with claims the json token, error: %v", err)
		return
	}

	claims, ok := token.Claims.(*jwtClaims)
	if !ok {
		logrus.Errorf("cannot parse jwt claims, error: %v", err)
		return nil, errors.New("Couldn't parse claims")
	}

	if claims.ExpiresAt < time.Now().Local().Unix() {
		logrus.Errorf("token expired already, error: %v", err)
		return nil, errors.New("JWT is expired")
	}

	return claims, nil

}
