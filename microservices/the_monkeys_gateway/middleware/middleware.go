package middleware

import (
	"bytes"
	"io"
	"net/http"

	"github.com/gin-contrib/cors" // Use this package
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func SetMiddlewareJSON(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next(w, r)
	}
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, PATCH, DELETE")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

func NewCorsMiddleware() gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOrigins:     []string{"*"}, // Allow all origins
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "IP", "Client", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "accept", "Cache-Control", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	})
}

func LogRequestBody() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Create a buffer to store the copied body
		var bodyBuffer bytes.Buffer
		// Copy the request body to the buffer
		if _, err := io.Copy(&bodyBuffer, c.Request.Body); err != nil {
			logrus.Errorf("error copying request body: %v", err)
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}
		// Close the original body (important for proper resource management)
		c.Request.Body.Close()

		// Restore the request body for downstream handlers
		c.Request.Body = io.NopCloser(&bodyBuffer)
		// logrus.Infof("Raw request body: %s", string(bodyBuffer.Bytes()))
		c.Next()
	}
}
