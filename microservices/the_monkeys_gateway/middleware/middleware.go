package middleware

import (
	"bytes"
	"io"
	"net"
	"net/http"

	// Use this package
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

func SetMiddlewareJSON(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next(w, r)
	}
}

func CORSMiddleware(allowedOrigins []string, allowedIPs []string) gin.HandlerFunc {
	// Convert slices to maps for faster lookup
	allowedOriginsMap := make(map[string]bool)
	for _, origin := range allowedOrigins {
		allowedOriginsMap[origin] = true
	}

	allowedIPsMap := make(map[string]bool)
	for _, ip := range allowedIPs {
		allowedIPsMap[ip] = true
	}

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		clientIP := c.ClientIP()

		// Check if the origin is allowed
		if _, ok := allowedOriginsMap[origin]; ok {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		}

		// Check if the client IP is allowed
		if _, ok := allowedIPsMap[clientIP]; ok {
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*") // Wildcard for IPs
		}

		// Set other CORS headers
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, IP, Client, OS")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, PATCH, DELETE")

		// Handle preflight requests
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		// If neither the origin nor the IP is allowed, block the request
		if _, originAllowed := allowedOriginsMap[origin]; !originAllowed {
			if _, ipAllowed := allowedIPsMap[clientIP]; !ipAllowed {
				c.AbortWithStatusJSON(403, gin.H{"error": "Access forbidden"})
				return
			}
		}

		c.Next()
	}
}

func NewWebSocketUpgrader(allowedOrigins []string, allowedIPs []string) websocket.Upgrader {
	// Convert slices to maps for faster lookup
	allowedOriginsMap := make(map[string]bool)
	for _, origin := range allowedOrigins {
		allowedOriginsMap[origin] = true
	}

	allowedIPsMap := make(map[string]bool)
	for _, ip := range allowedIPs {
		allowedIPsMap[ip] = true
	}

	return websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			// Check the Origin header
			origin := r.Header.Get("Origin")
			if origin != "" {
				if _, ok := allowedOriginsMap[origin]; ok {
					return true
				}
			}

			// Check the client's IP address
			clientIP, _, err := net.SplitHostPort(r.RemoteAddr)
			if err == nil {
				if _, ok := allowedIPsMap[clientIP]; ok {
					return true
				}
			}

			// Reject requests not matching the allowed origins or IPs
			return false
		},
	}
}

// func TmpCORSMiddleware() gin.HandlerFunc {
// 	config := cors.Config{
// 		AllowOrigins:     []string{"*"},                                                // Allow all origins
// 		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"}, // Allow all methods
// 		AllowHeaders:     []string{"*"},                                                // Allow all headers
// 		AllowCredentials: true,
// 	}
// 	return cors.New(config)
// }

// func NewCorsMiddleware() gin.HandlerFunc {
// 	return cors.New(cors.Config{
// 		AllowOrigins:     []string{"*"}, // Allow all origins
// 		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
// 		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "IP", "Client", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "accept", "Cache-Control", "X-Requested-With"},
// 		ExposeHeaders:    []string{"Content-Length"},
// 		AllowCredentials: true,
// 	})
// }

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
