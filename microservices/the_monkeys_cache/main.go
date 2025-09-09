package main

import (
	"net"

	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_cache/pb"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/logger"
	"github.com/the-monkeys/the_monkeys/microservices/rabbitmq"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_cache/internal/consumer"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_cache/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	log := logger.ZapForService("tm_cache")

	cfg, err := config.GetConfig()
	if err != nil {
		log.Fatalf("Failed to load file server config, error: %+v", err)
	}

	lis, err := net.Listen("tcp", cfg.Microservices.TheMonkeysCache)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// Connect to rabbitmq server
	qConn := rabbitmq.Reconnect(cfg.RabbitMQ)
	go consumer.ConsumeFromQueue(qConn, cfg, log)

	s := grpc.NewServer()
	cacheServer := service.NewCacheServer(log)

	grpcServer := service.NewGRPCServer(cacheServer)

	pb.RegisterCacheServiceServer(s, grpcServer)

	reflection.Register(s)

	log.Debugf("✅ the monkey's cache server started at: %v", cfg.Microservices.TheMonkeysCache)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
