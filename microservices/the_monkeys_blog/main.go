package main

import (
	"fmt"
	"log"
	"net"

	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_blog/pb"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/logger"
	"github.com/the-monkeys/the_monkeys/microservices/rabbitmq"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_blog/internal/consumer"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_blog/internal/database"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_blog/internal/seo"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_blog/internal/services"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func printBanner(host, env string) {
	banner := "\n" +
		"┌──────────────────────────────────────────────────────────┐\n" +
		"│   🐒  The Monkeys Blog Service                            │\n" +
		"│   Status   : ONLINE                                       │\n" +
		fmt.Sprintf("│   Host     : %-44s│\n", host) +
		fmt.Sprintf("│   Env      : %-44s│\n", env) +
		"│   Logs     : zap (structured)                             │\n" +
		"│   Tip      : Set LOG_LEVEL=debug for verbose output       │\n" +
		"└──────────────────────────────────────────────────────────┘\n"
	fmt.Print(banner)
}

func main() {
	cfg, err := config.GetConfig()
	if err != nil {
		log.Fatalln("failed to load the config file, error: ", err)
		return
	}

	logg := logger.ZapForService("blog")
	defer logger.Sync()

	host := fmt.Sprintf("%s:%d", cfg.Microservices.TheMonkeysBlog, cfg.Microservices.BlogPort)
	// Bind to all interfaces for health checks to work
	listenAddr := fmt.Sprintf("0.0.0.0:%d", cfg.Microservices.BlogPort)
	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		logg.Fatalf("failed to listen", "host", listenAddr, "err", err)
		return
	}

	osClient, err := database.NewElasticsearchClient(cfg.Opensearch.Host, cfg.Opensearch.Username, cfg.Opensearch.Password, logg)
	if err != nil {
		logg.Fatalf("cannot get the opensearch client", "err", err)
		return
	}

	qConn := rabbitmq.Reconnect(cfg.RabbitMQ)
	go consumer.ConsumeFromQueue(qConn, cfg.RabbitMQ, logg, osClient)

	seoManager := seo.NewSEOManager(logg, cfg)
	blogService := services.NewBlogService(osClient, seoManager, logg, cfg, qConn)

	grpcServer := grpc.NewServer()
	pb.RegisterBlogServiceServer(grpcServer, blogService)

	// Register health check service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)

	// Set the service as serving (healthy)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus("BlogService", grpc_health_v1.HealthCheckResponse_SERVING)

	printBanner(host, cfg.AppEnv)
	if err := grpcServer.Serve(lis); err != nil {
		logg.Fatalw("failed to serve", "err", err)
		return
	}
}
