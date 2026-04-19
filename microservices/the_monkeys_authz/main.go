package main

import (
	"fmt"
	"net"

	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_authz/pb"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/logger"
	"github.com/the-monkeys/the_monkeys/microservices/rabbitmq"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_authz/internal/db"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_authz/internal/services"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_authz/internal/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func printBanner(host, env string) {
	banner := "\n" +
		"┌──────────────────────────────────────────────────────────┐\n" +
		"│   🐒  The Monkeys Authz Service                          │\n" +
		"│   Status   : ONLINE                                      │\n" +
		fmt.Sprintf("│   Host     : %-44s│\n", host) +
		fmt.Sprintf("│   Env      : %-44s│\n", env) +
		"│   Logs     : zap (structured)                            │\n" +
		"│   Tip      : Set LOG_LEVEL=debug for verbose output      │\n" +
		"└──────────────────────────────────────────────────────────┘\n"
	fmt.Print(banner)
}

func main() {
	log := logger.ZapForService("tm-authz")

	cfg, err := config.GetConfig()
	if err != nil {
		log.Errorw("cannot load auth service config", "error", err)
		return
	}

	dbHandler, err := db.NewAuthDBHandler(cfg, log)
	if err != nil {
		log.Fatalw("cannot connect to db", "error", err)
	}

	otpRepo, err := db.NewOTPRepository(cfg, log)
	if err != nil {
		log.Fatalw("cannot connect to Redis for OTP storage", "error", err)
	}

	jwt := utils.JwtWrapper{
		SecretKey:       cfg.JWT.SecretKey,
		Issuer:          "tm-authz",
		ExpirationHours: 24 * 7,
	}

	host := fmt.Sprintf("%s:%d", cfg.Microservices.TheMonkeysAuthz, cfg.Microservices.AuthzPort)
	// Bind to all interfaces for health checks to work
	listenAddr := fmt.Sprintf("0.0.0.0:%d", cfg.Microservices.AuthzPort)
	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalw("auth service cannot listen", "address", listenAddr, "error", err)
	}

	qConn := rabbitmq.NewConnManager(cfg.RabbitMQ)

	authServer := services.NewAuthzSvc(dbHandler, otpRepo, jwt, cfg, qConn, log)

	grpcServer := grpc.NewServer()
	pb.RegisterAuthServiceServer(grpcServer, authServer)

	// Register health check service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)

	// Set the service as serving (healthy)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus("AuthService", grpc_health_v1.HealthCheckResponse_SERVING)

	printBanner(host, cfg.AppEnv)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalw("gRPC auth server cannot start", "error", err)
	}
}
