package utils

import (
	"errors"
	"time"

	"github.com/89minutes/the_new_project/services/auth_service/pkg/models"
	"github.com/golang-jwt/jwt"
	"github.com/sirupsen/logrus"
)

type JwtWrapper struct {
	SecretKey       string
	Issuer          string
	ExpirationHours int64
}

type jwtClaims struct {
	jwt.StandardClaims
	Id    int64
	Email string
}

func (w *JwtWrapper) GenerateToken(user models.TheMonkeysUser) (signedToken string, err error) {
	claims := &jwtClaims{
		Id:    user.Id,
		Email: user.Email,
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
