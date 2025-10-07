package main

import (
	"fmt"
	"net"

	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_activity/pb"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/logger"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_activity/internal/database"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_activity/internal/services"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func printBanner(host, env string) {
	banner := "\n" +
		"┌──────────────────────────────────────────────────────────┐\n" +
		"│   📊  The Monkeys Activity Service                       │\n" +
		"│   Status   : ONLINE                                      │\n" +
		fmt.Sprintf("│   Host     : %-44s│\n", host) +
		fmt.Sprintf("│   Env      : %-44s│\n", env) +
		"│   Logs     : zap (structured)                            │\n" +
		"│   Tip      : Set LOG_LEVEL=debug for verbose output      │\n" +
		"└──────────────────────────────────────────────────────────┘\n"
	fmt.Print(banner)
}

func main() {
	log := logger.ZapForService("tm-activity")

	cfg, err := config.GetConfig()
	if err != nil {
		log.Errorw("cannot load activity service config", "error", err)
		return
	}

	host := fmt.Sprintf("%s:%d", cfg.Microservices.TheMonkeysActivity, cfg.Microservices.ActivityPort)
	// Bind to all interfaces for health checks to work
	listenAddr := fmt.Sprintf("0.0.0.0:%d", cfg.Microservices.ActivityPort)
	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalw("activity service cannot listen", "address", listenAddr, "error", err)
	}

	// Initialize Elasticsearch database
	db, err := database.NewActivityDB(cfg, log)
	if err != nil {
		log.Fatalw("failed to initialize activity database", "error", err)
	}

	grpcServer := grpc.NewServer()

	// Register ActivityService
	activityServer := services.NewActivityServiceServer(cfg, log, db)
	pb.RegisterActivityServiceServer(grpcServer, activityServer)

	// Register health check service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)

	// Set the service as serving (healthy)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus("ActivityService", grpc_health_v1.HealthCheckResponse_SERVING)

	printBanner(host, cfg.AppEnv)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalw("gRPC activity server cannot start", "error", err)
	}
}
