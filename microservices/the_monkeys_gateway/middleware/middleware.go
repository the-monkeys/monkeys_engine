package middleware

import (
	"bytes"
	"io"
	"net/http"
	"regexp"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/ulule/limiter/v3"
	limiterGin "github.com/ulule/limiter/v3/drivers/middleware/gin"
	"github.com/ulule/limiter/v3/drivers/store/memory"
	"go.uber.org/zap"
)

func SetMiddlewareJSON(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next(w, r)
	}
}

func CORSMiddleware(allowedOriginExp string) gin.HandlerFunc {
	lg := zap.S().With("middleware", "cors")
	return func(c *gin.Context) {
		requestOrigin := c.Request.Header.Get("Origin")
		if match, _ := regexp.Match(allowedOriginExp, []byte(requestOrigin)); match {
			c.Writer.Header().Set("Access-Control-Allow-Origin", requestOrigin)
			lg.Debugw("cors request allowed", "origin", requestOrigin, "method", c.Request.Method)
		} else {
			lg.Warnw("cors request blocked", "origin", requestOrigin, "method", c.Request.Method)
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
	cfg := cors.Config{AllowOrigins: []string{"*"}, AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"}, AllowHeaders: []string{"*"}, AllowCredentials: true}
	corsMiddleware := cors.New(cfg)
	lg := zap.S().With("middleware", "tmp_cors")
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin != "" {
			lg.Debugw("temporary cors allow all", "origin", origin, "method", c.Request.Method)
		}
		corsMiddleware(c)
	}
}

func NewCorsMiddleware() gin.HandlerFunc {
	return cors.New(cors.Config{AllowOrigins: []string{"*"}, AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}, AllowHeaders: []string{"Origin", "Content-Type", "Authorization", "IP", "Client", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "accept", "Cache-Control", "X-Requested-With"}, ExposeHeaders: []string{"Content-Length"}, AllowCredentials: true})
}

func LogRequestBody() gin.HandlerFunc {
	lg := zap.S().With("middleware", "req_body")
	return func(c *gin.Context) {
		var bodyBuffer bytes.Buffer
		if _, err := io.Copy(&bodyBuffer, c.Request.Body); err != nil {
			lg.Errorw("copy body failed", "err", err)
			if err := c.AbortWithError(http.StatusBadRequest, err); err != nil {
				lg.Errorw("abort request failed", "err", err)
			}
			return
		}
		if err := c.Request.Body.Close(); err != nil {
			lg.Errorw("close body failed", "err", err)
		}
		c.Request.Body = io.NopCloser(&bodyBuffer)
		c.Next()
	}
}

func RateLimiterMiddleware(limit string) gin.HandlerFunc {
	rate, err := limiter.NewRateFromFormatted(limit)
	if err != nil {
		panic(err)
	}
	store := memory.NewStore()
	instance := limiter.New(store, rate)
	return limiterGin.NewMiddleware(instance)
}
