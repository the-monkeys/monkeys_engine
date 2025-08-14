package middleware

import (
	"bytes"
	"io"
	"net/http"
	"regexp"

	"github.com/gin-contrib/cors" // Use this package
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/ulule/limiter/v3"
	limiterGin "github.com/ulule/limiter/v3/drivers/middleware/gin"
	"github.com/ulule/limiter/v3/drivers/store/memory"
)

func SetMiddlewareJSON(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next(w, r)
	}
}

func CORSMiddleware(allowedOriginExp string) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestOrigin := c.Request.Header.Get("Origin")
		if match, _ := regexp.Match(allowedOriginExp, []byte(requestOrigin)); match {
			c.Writer.Header().Set("Access-Control-Allow-Origin", requestOrigin)
			logrus.WithFields(logrus.Fields{
				"origin": requestOrigin,
				"method": c.Request.Method,
			}).Debug("CORS request allowed")
		} else {
			logrus.WithFields(logrus.Fields{
				"origin": requestOrigin,
				"method": c.Request.Method,
			}).Warn("CORS request blocked - origin not allowed")
		}

		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, IP, Client, OS")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, PATCH, DELETE")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

func TmpCORSMiddleware() gin.HandlerFunc {
	config := cors.Config{
		AllowOrigins:     []string{"*"},                                                // Allow all origins
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"}, // Allow all methods
		AllowHeaders:     []string{"*"},                                                // Allow all headers
		AllowCredentials: true,
	}

	corsMiddleware := cors.New(config)

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin != "" {
			logrus.WithFields(logrus.Fields{
				"origin": origin,
				"method": c.Request.Method,
			}).Debug("Temporary CORS middleware - allowing all origins")
		}
		corsMiddleware(c)
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
			if err := c.AbortWithError(http.StatusBadRequest, err); err != nil {
				logrus.Errorf("error aborting request: %v", err)
			}
			return
		}
		// Close the original body (important for proper resource management)
		if err := c.Request.Body.Close(); err != nil {
			logrus.Errorf("error closing request body: %v", err)
		}

		// Restore the request body for downstream handlers
		c.Request.Body = io.NopCloser(&bodyBuffer)
		// logrus.Infof("Raw request body: %s", string(bodyBuffer.Bytes()))
		c.Next()
	}
}

// RateLimiterMiddleware creates a rate limiter middleware for Gin routes
func RateLimiterMiddleware(limit string) gin.HandlerFunc {
	// Define the rate limit (e.g., "5-S" means 5 requests per second)
	rate, err := limiter.NewRateFromFormatted(limit)
	if err != nil {
		panic(err)
	}

	store := memory.NewStore()

	instance := limiter.New(store, rate)

	return limiterGin.NewMiddleware(instance)
}
