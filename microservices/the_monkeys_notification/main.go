package main

import (
	"fmt"
	"net"

	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_notification/pb"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/logger"
	"github.com/the-monkeys/the_monkeys/microservices/rabbitmq"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_notification/internal/consumer"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_notification/internal/database"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_notification/internal/services"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func printBanner(cfg *config.Config, _ *zap.SugaredLogger) {
	banner := `
┌────────────────────────────────────────────────────────────┐
│   🐒  The Monkeys Notification Service                      │
│   Status   : ONLINE                                         │
│   Service  : ` + cfg.Microservices.TheMonkeysNotification + `
│   Port     : ` + fmt.Sprintf("%d", cfg.Microservices.NotificationPort) + `
│   Env      : ` + cfg.AppEnv + `
│   Logs     : zap (structured)                               │
│   Tip      : set LOG_LEVEL=debug for verbose logs           │
└────────────────────────────────────────────────────────────┘`
	fmt.Printf("%s\nEnvironment: %s\nService: %s\nPort: %d\n", banner, cfg.AppEnv, cfg.Microservices.TheMonkeysNotification, cfg.Microservices.NotificationPort)
}

func main() {
	cfg, err := config.GetConfig()
	if err != nil {
		logger.ZapSugar().Errorf("failed to load notification config, error: %+v", err)
	}
	log := logger.ZapForService("tm_notification")

	db, err := database.NewNotificationDb(cfg, log)
	if err != nil {
		log.Fatalf("failed to connect to the database: %v", err)
	}

	// Bind to all interfaces for health checks to work
	listenAddr := fmt.Sprintf("0.0.0.0:%d", cfg.Microservices.NotificationPort)
	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Errorf("failed to listen at port %v, error: %+v", listenAddr, err)
	}

	// Connect to rabbitmq server
	qConn := rabbitmq.Reconnect(cfg.RabbitMQ)
	go consumer.ConsumeFromQueue(qConn, cfg.RabbitMQ, log, db)

	notificationSvc := services.NewNotificationSvc(db, log, cfg)

	grpcServer := grpc.NewServer()

	pb.RegisterNotificationServiceServer(grpcServer, notificationSvc)

	// Register health check service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)

	// Set the service as serving (healthy)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus("NotificationService", grpc_health_v1.HealthCheckResponse_SERVING)

	printBanner(cfg, log)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}

func BlogServiceConn(addr string) (*grpc.ClientConn, error) {
	logger.ZapSugar().Debugf("gRPC dialing to the blog server: %v", addr)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return conn, nil
}
