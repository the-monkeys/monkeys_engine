package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/secure"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/logger"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/internal/admin"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/internal/auth"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/internal/blog"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/internal/file_server"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/internal/notification"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/internal/systems"

	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/internal/monkeys_ai"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/internal/user_service"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/middleware"
)

type Server struct {
	router *gin.Engine
}

func newServer() *Server {
	return &Server{router: gin.New()}
}

func main() {
	// Load API Gateway configuration
	cfg, err := config.GetConfig()
	if err != nil {
		logrus.Fatalf("failed to load the config: %v", err)
	}

	log := logger.GetLogger()
	// Set Gin to Release mode
	gin.SetMode(gin.ReleaseMode)

	// Create a gin router and add the Recovery middleware to recover from panics
	server := newServer()
	server.router.Use(gin.Recovery())
	server.router.Use(gin.Logger())
	server.router.MaxMultipartMemory = 8 << 20

	// Apply security middleware
	server.router.Use(secure.New(secure.Config{
		FrameDeny:             true,
		ContentTypeNosniff:    true,
		BrowserXssFilter:      true,
		ContentSecurityPolicy: "default-src 'self';", // Customize as needed
		ReferrerPolicy:        "strict-origin-when-cross-origin",
	}))

	// Enable CORS
	server.router.Use(middleware.CORSMiddleware(cfg.Cors.AllowedOriginExp))

	// Log request body
	server.router.Use(middleware.LogRequestBody())

	// Register REST routes for all the microservices
	authClient := auth.RegisterAuthRouter(server.router, cfg)
	authClient.Log.SetReportCaller(true)
	authClient.Log.SetFormatter(&logrus.TextFormatter{
		DisableColors: false,
		FullTimestamp: false,
	})

	userClient := user_service.RegisterUserRouter(server.router, cfg, authClient)
	blog.RegisterBlogRouter(server.router, cfg, authClient, userClient)
	file_server.RegisterFileStorageRouter(server.router, cfg, authClient)
	notification.RegisterNotificationRoute(server.router, cfg, authClient, log)
	monkeys_ai.RegisterRecommendationRoute(server.router, cfg, authClient, log)

	// Register admin routes (restricted to local network)
	admin.RegisterAdminRouter(server.router, cfg)

	// Register system routes (restricted with system key)
	systems.RegisterSystemRouter(server.router, cfg)

	// Health check endpoint
	server.router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
		})
	})

	server.start(context.Background(), cfg)
}

func (s *Server) start(ctx context.Context, config *config.Config) {
	// TLS certificate and key
	var tlsCert, tlsKey string
	if os.Getenv("NO_TLS") != "1" {
		tlsCert = os.Getenv("TLS_CERT")
		if tlsCert == "" {
			tlsCert = "config/certs/openssl/server.crt"
		}
		tlsKey = os.Getenv("TLS_KEY")
		if tlsKey == "" {
			tlsKey = "config/certs/openssl/server.key"
		}
	}

	// Launch the server (this is a blocking call)
	s.launchServer(ctx, config, tlsCert, tlsKey)
}

// Start the server
func (s *Server) launchServer(ctx context.Context, config *config.Config, tlsCert, tlsKey string) {
	// If we don't have a TLS certificate, don't enable TLS
	enableTLS := (tlsCert != "" && tlsKey != "")

	// HTTP server (no TLS)
	httpSrv := &http.Server{
		Addr:           config.TheMonkeysGateway.HTTP,
		Handler:        s.router,
		MaxHeaderBytes: 1 << 20,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
	}

	// HTTPS server (with TLS)
	httpsSrv := &http.Server{
		Addr:           config.TheMonkeysGateway.HTTPS,
		Handler:        s.router,
		MaxHeaderBytes: 1 << 20,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
	}

	// Start the HTTP server in a background goroutine
	go func() {
		logrus.Printf("✅ the monkeys gateway http is listening at http://%s\n", config.TheMonkeysGateway.HTTP)
		// Next call blocks until the server is shut down
		err := httpSrv.ListenAndServe()
		if err != http.ErrServerClosed {
			logrus.Errorf("cannot start the http server, error: %+v", err)
			panic(err)
		}
	}()

	// Start the HTTPS server in a background goroutine
	if enableTLS {
		go func() {
			logrus.Printf("✅ the monkeys gateway https is listening at https://%s\n", config.TheMonkeysGateway.HTTPS)
			err := httpsSrv.ListenAndServeTLS(tlsCert, tlsKey)
			if err != http.ErrServerClosed {
				logrus.Errorf("cannot start the http server, error: %+v", err)
				panic(err)
			}
		}()
	}

	// Listen to SIGINT and SIGTERM signals
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)

	// Block until we either get a termination signal, or until the context is canceled
	select {
	case <-ctx.Done():
	case <-ch:
	}

	// We received an interrupt signal, shut down both servers
	var errHttp, errHttps error
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	errHttp = httpSrv.Shutdown(shutdownCtx)
	if enableTLS {
		errHttps = httpsSrv.Shutdown(shutdownCtx)
	}
	shutdownCancel()
	// Log the errors (could be context canceled)
	if errHttp != nil {
		logrus.Println("HTTP server shutdown error:", errHttp)
	}
	if errHttps != nil {
		logrus.Println("HTTPS server shutdown error:", errHttps)
	}
}
