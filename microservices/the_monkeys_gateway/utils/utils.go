package utils

import (
	"fmt"

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
	// ctx.Get returns two values: the value and a boolean indicating if it exists
	accessValue, exists := ctx.Get("user_access_level")

	// Check if the value exists in the context
	if !exists {
		fmt.Println("user_access_level not found in context")
		return false
	}

	// Type assert the value to []string (make sure the context actually holds this type)
	accessLevels, ok := accessValue.([]string)
	if !ok {
		fmt.Println("user_access_level is not of type []string")
		return false
	}

	// Use the helper function to check if the specific access exists
	return CheckUserAccessLevel(accessLevels, accessToCheck)
}
