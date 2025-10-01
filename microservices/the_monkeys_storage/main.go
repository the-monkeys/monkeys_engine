package main

import (
	"fmt"
	"net"
	"os"

	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_file_service/pb"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/constants"
	"github.com/the-monkeys/the_monkeys/logger"
	"github.com/the-monkeys/the_monkeys/microservices/rabbitmq"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_storage/constant"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_storage/internal/consumer"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_storage/internal/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func init() {
	// Define the complete path including `/` and the folder name
	folderPath := "/" + constant.ProfileDir
	blogPath := "/" + constant.BlogDir

	// Check if the directory already exists
	_, err := os.Stat(folderPath)

	// If the directory doesn't exist, create it with permissions 0755
	if os.IsNotExist(err) {
		err = os.MkdirAll(folderPath, 0755)
		if err != nil {
			logger.ZapSugar().Fatalf("Error creating folder path: %v", err)
		}
	}

	// Check if the blogPath directory already exists
	_, err = os.Stat(blogPath)

	// If the blogPath directory doesn't exist, create it with permissions 0755
	if os.IsNotExist(err) {
		err = os.MkdirAll(blogPath, 0755)
		if err != nil {
			logger.ZapSugar().Fatalf("Error creating blog path: %v", err)
		}
	}
}

func printBanner(cfg *config.Config) {
	banner := `
┌────────────────────────────────────────────────────────────┐
│   🐒  The Monkeys Storage Service                           │
│   Status   : ONLINE                                         │
│   Service  : ` + cfg.Microservices.TheMonkeysFileStore + `
│   Port     : ` + fmt.Sprintf("%d", cfg.Microservices.StoragePort) + `
│   Env      : ` + cfg.AppEnv + `
│   Logs     : zap (structured)                               │
│   Tip      : set LOG_LEVEL=debug for verbose logs           │
└────────────────────────────────────────────────────────────┘`
	fmt.Printf("%s\nEnvironment: %s\nService: %s\nPort: %d\n", banner, cfg.AppEnv, cfg.Microservices.TheMonkeysFileStore, cfg.Microservices.StoragePort)
}

func main() {
	cfg, err := config.GetConfig()
	if err != nil {
		logger.ZapSugar().Errorf("Failed to load file server config, error: %+v", err)
	}
	log := logger.ZapForService("tm_storage")

	// Connect to rabbitmq server
	qConn := rabbitmq.Reconnect(cfg.RabbitMQ)
	go consumer.ConsumeFromQueue(qConn, cfg.RabbitMQ, log)

	// Bind to all interfaces for health checks to work
	listenAddr := fmt.Sprintf("0.0.0.0:%d", cfg.Microservices.StoragePort)
	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Errorf("File server failed to listen at port %v, error: %+v", listenAddr, err)
	}

	fileService := server.NewFileService(constant.BlogDir, constant.ProfileDir, log)

	grpcServer := grpc.NewServer(grpc.MaxRecvMsgSize(constants.MaxMsgSize), grpc.MaxSendMsgSize(constants.MaxMsgSize))
	pb.RegisterUploadBlogFileServer(grpcServer, fileService)

	// Register health check service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)

	// Set the service as serving (healthy)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus("StorageService", grpc_health_v1.HealthCheckResponse_SERVING)

	log.Debugf("the file storage server started at: %v:%d", cfg.Microservices.TheMonkeysFileStore, cfg.Microservices.StoragePort)
	printBanner(cfg)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
