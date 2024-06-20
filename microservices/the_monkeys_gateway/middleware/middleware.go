package middleware

import (
	"bytes"
	"io/ioutil"
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
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, PUT, DELETE, PATCH")

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
		AllowMethods:     []string{"PUT", "PATCH", "GET", "POST", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "IP", "Client", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "accept", "Cache-Control", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	})
}

func LogRequestBody() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Log the raw request body
		body, _ := c.GetRawData()
		logrus.Infof("Raw request body: %s", string(body))

		// Restore the request body for downstream handlers
		c.Request.Body = ioutil.NopCloser(bytes.NewReader(body))
		c.Next()
	}
}
