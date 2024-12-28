package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

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

func GetCLientIP(ctx *gin.Context) {
	ip := ctx.ClientIP()
	if _, err := os.Stat("ip.json"); os.IsNotExist(err) {
		file, err := os.Create("ip.json")
		if err != nil {
			logrus.Errorf("cannot create log file, error: %v", err)
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot create log file"})
			return
		}
		defer file.Close()

		logData := map[string]string{"ip": ip}
		logBytes, err := json.Marshal(logData)
		if err != nil {
			logrus.Errorf("cannot marshal log data, error: %v", err)
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot marshal log data"})
			return
		}

		if _, err := file.Write(logBytes); err != nil {
			logrus.Errorf("cannot write to log file, error: %v", err)
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "cannot write to log file"})
			return
		}
	}
}
