package utils

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// CheckUserAccessLevel checks if a specific access level is present in the user_access_level []string.
func CheckUserAccessLevel(accessLevels []string, accessToCheck string) bool {
	for _, access := range accessLevels {
		if access == accessToCheck {
			return true
		}
	}
	return false
}

func CheckUserAccessInContext(ctx *gin.Context, accessToCheck string) bool {
	accessValue, exists := ctx.Get("user_access_level")
	if !exists {
		fmt.Println("user_access_level not found in context")
		return false
	}
	accessLevels, ok := accessValue.([]string)
	if !ok {
		fmt.Println("user_access_level is not of type []string")
		return false
	}
	return CheckUserAccessLevel(accessLevels, accessToCheck)
}

func CheckUserRoleInContext(ctx *gin.Context, role string) bool {
	return strings.EqualFold(ctx.GetString("user_role"), role)
}

func SetMonkeysAuthCookie(ctx *gin.Context, token string) {
	authCookie := &http.Cookie{
		Name:     "mat",
		Value:    token,
		HttpOnly: true,
		Path:     "/",
		MaxAge:   int(time.Duration(24*30)*time.Hour) / int(time.Second),
		SameSite: http.SameSiteNoneMode,
		Secure:   true,
	}

	http.SetCookie(ctx.Writer, authCookie)
}
