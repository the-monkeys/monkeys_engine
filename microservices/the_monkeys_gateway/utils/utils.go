package utils

import (
	//"encoding/json"
	"fmt"
	"net/http"
	//"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
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
	// ctx.Get returns two values: the value and a boolean indicating if it exists
	accessValue, exists := ctx.Get("user_access_level")

	// Check if the value exists in the context
	if !exists {
		fmt.Println("user_access_level not found in context")
		return false
	}

	// Type assert the value to []string (make sure the context actually holds this type)
	accessLevels, ok := accessValue.([]string)
	logrus.Info("accessLevels: ", accessLevels)
	if !ok {
		fmt.Println("user_access_level is not of type []string")
		return false
	}

	// Use the helper function to check if the specific access exists
	return CheckUserAccessLevel(accessLevels, accessToCheck)
}

func CheckUserRoleInContext(ctx *gin.Context, role string) bool {
	userRole := ctx.GetString("user_role")
	return strings.EqualFold(userRole, role)
}

// func GetClientIP(ctx *gin.Context) {
// 	ip := ctx.ClientIP()

// 	file, err := os.OpenFile("ip.json", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
// 	if err != nil {
// 		logrus.Errorf("cannot open log file, error: %v", err)
// 		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot open log file"})
// 		return
// 	}
// 	defer func() {
// 		if err := file.Close(); err != nil {
// 			logrus.Errorf("cannot close log file, error: %v", err)
// 		}
// 	}()

// 	logData := map[string]string{"ip": ip}
// 	completeMapData := []map[string]string{}
// 	completeMapData = append(completeMapData, logData)
// 	logBytes, err := json.Marshal(completeMapData)
// 	if err != nil {
// 		logrus.Errorf("cannot marshal log data, error: %v", err)
// 		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot marshal log data"})
// 		return
// 	}

// 	if _, err := file.Write(append(logBytes, '\n')); err != nil {
// 		logrus.Errorf("cannot write to log file, error: %v", err)
// 		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot write to log file"})
// 		return
// 	}
// }

func SetMonkeysAuthCookie(ctx *gin.Context, token string) {
	//var authCookie *http.Cookie
	authCookie := &http.Cookie{
		Name:     "mat",
		Value:    token,
		HttpOnly: true,
		Path:     "/",
		MaxAge:   int(time.Duration(24*30)*time.Hour) / int(time.Second), // 30d days
		SameSite: http.SameSiteNoneMode,
		Secure:   true,
	}

	http.SetCookie(ctx.Writer, authCookie)
}
