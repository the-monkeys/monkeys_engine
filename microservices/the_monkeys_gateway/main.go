package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/secure"
	"github.com/gin-gonic/gin"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/logger"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/internal/admin"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/internal/auth"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/internal/blog"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/internal/monkeys_ai"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/internal/notification"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/internal/reports"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/internal/storage"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/internal/storage_v2"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/internal/systems"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/internal/user_service"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/middleware"

	"go.uber.org/zap"
)

type Server struct {
	router *gin.Engine
}

func newServer() *Server {
	return &Server{router: gin.New()}
}

func printBanner(cfg *config.Config) {
	banner := "\n" +
		"┌────────────────────────────────────────────────────────────┐\n" +
		"│   🐒  The Monkeys API Gateway                               │\n" +
		"│   Status   : ONLINE                                         │\n" +
		"│   HTTP     : http://" + cfg.TheMonkeysGateway.HTTP + "\n" +
		"│   HTTPS    : https://" + cfg.TheMonkeysGateway.HTTPS + "\n" +
		"│   Env      : " + cfg.AppEnv + "\n" +
		"│   Logs     : zap (structured)                               │\n" +
		"│   Tip      : export LOG_LEVEL=debug for verbose logs        │\n" +
		"└────────────────────────────────────────────────────────────┘\n"
	fmt.Print(banner)
}

func main() {
	// Load API Gateway configuration
	cfg, err := config.GetConfig()
	if err != nil {
		panic("failed to load config: " + err.Error())
	}
	log := logger.ZapForService("gateway")
	defer logger.Sync()

	// Set Gin to Release mode
	gin.SetMode(gin.ReleaseMode)

	// Create a gin router and add the Recovery middleware to recover from panics
	server := newServer()
	server.router.Use(gin.Recovery())
	// retain default gin logger? use custom zap middleware later
	// server.router.Use(gin.Logger())
	server.router.MaxMultipartMemory = 8 << 20

	// Apply security middleware
	server.router.Use(secure.New(secure.Config{
		FrameDeny:             true,
		ContentTypeNosniff:    true,
		BrowserXssFilter:      true,
		ContentSecurityPolicy: "default-src 'self';", // Customize as needed
		ReferrerPolicy:        "strict-origin-when-cross-origin",
	}))

	// Enable CORS - conditionally use temp CORS or strict CORS based on config
	if cfg.Cors.UseTempCors {
		log.Debug("using temporary CORS middleware (allow all origins)")
		server.router.Use(middleware.TmpCORSMiddleware())
	} else {
		log.Debug("using strict CORS middleware (regex)")
		server.router.Use(middleware.CORSMiddleware(cfg.Cors.AllowedOriginExp))
	}

	// Log request body
	server.router.Use(middleware.LogRequestBody())

	// Register REST routes for all the microservices
	authClient := auth.RegisterAuthRouter(server.router, cfg, log)
	userClient := user_service.RegisterUserRouter(server.router, cfg, authClient, log)
	blog.RegisterBlogRouter(server.router, cfg, authClient, userClient, log)
	storage.RegisterFileStorageRouter(server.router, cfg, authClient, log)
	storage_v2.RegisterRoutes(server.router, cfg, authClient, log)
	notification.RegisterNotificationRoute(server.router, cfg, authClient, log)
	monkeys_ai.RegisterRecommendationRoute(server.router, cfg, authClient, log)
	admin.RegisterAdminRouter(server.router, cfg, log)
	systems.RegisterSystemRouter(server.router, cfg, log)
	reports.RegisterReportsServiceRoutes(server.router, cfg, authClient, log)

	// Health check endpoint
	server.router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
		})
	})

	printBanner(cfg)
	server.start(context.Background(), cfg, log)
}

func (s *Server) start(ctx context.Context, cfg *config.Config, log *zap.SugaredLogger) {
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
	s.launchServer(ctx, cfg, tlsCert, tlsKey, log)
}

// Start the server
func (s *Server) launchServer(ctx context.Context, cfg *config.Config, tlsCert, tlsKey string, log *zap.SugaredLogger) {
	// If we don't have a TLS certificate, don't enable TLS
	enableTLS := (tlsCert != "" && tlsKey != "")

	common := func(srv *http.Server, name string) {
		log.Infow(name+" listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Errorw(name+" server start failed", "err", err)
			panic(err)
		}
	}

	// HTTP server (no TLS)
	httpSrv := &http.Server{
		Addr:           cfg.TheMonkeysGateway.HTTP,
		Handler:        s.router,
		MaxHeaderBytes: 1 << 20,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
	}

	// HTTPS server (with TLS)
	httpsSrv := &http.Server{
		Addr:           cfg.TheMonkeysGateway.HTTPS,
		Handler:        s.router,
		MaxHeaderBytes: 1 << 20,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
	}

	// Start the HTTP server in a background goroutine
	go common(httpSrv, "http")

	// Start the HTTPS server in a background goroutine
	if enableTLS {
		go func() {
			log.Infow("https listening", "addr", httpsSrv.Addr)
			if err := httpsSrv.ListenAndServeTLS(tlsCert, tlsKey); err != nil && err != http.ErrServerClosed {
				log.Errorw("https server start failed", "err", err)
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
		log.Errorw("http shutdown error", "err", errHttp)
	}
	if errHttps != nil {
		log.Errorw("https shutdown error", "err", errHttps)
	}
	log.Infow("gateway shutdown complete")
}
