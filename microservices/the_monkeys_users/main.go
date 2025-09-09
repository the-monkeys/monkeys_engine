package main

import (
	"fmt"
	"net"

	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_user/pb"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/logger"
	"github.com/the-monkeys/the_monkeys/microservices/rabbitmq"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_users/internal/consumer"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_users/internal/database"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_users/internal/services"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func printBanner(cfg *config.Config) {
	banner := `
┌────────────────────────────────────────────────────────────┐
│   🐒  The Monkeys User Service                             │
│   Status   : ONLINE                                         │
│   Service  : ` + cfg.Microservices.TheMonkeysUser + `
│   Port     : ` + fmt.Sprintf("%d", cfg.Microservices.UserPort) + `
│   Env      : ` + cfg.AppEnv + `
│   Logs     : zap (structured)                               │
│   Tip      : set LOG_LEVEL=debug for verbose logs           │
└────────────────────────────────────────────────────────────┘`
	fmt.Printf("%s\nEnvironment: %s\nService: %s\nPort: %d\n", banner, cfg.AppEnv, cfg.Microservices.TheMonkeysUser, cfg.Microservices.UserPort)
}

func main() {
	cfg, err := config.GetConfig()
	if err != nil {
		fmt.Printf("failed to load user config, error: %+v\n", err)
	}
	log := logger.ZapForService("tm_users")

	db, err := database.NewUserDbHandler(cfg, log)
	if err != nil {
		log.Fatalf("failed to connect to the database: %v", err)
	}

	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.Microservices.TheMonkeysUser, cfg.Microservices.UserPort))
	if err != nil {
		log.Errorf("failed to listen at port %v, error: %+v", cfg.Microservices.TheMonkeysUser, err)
	}

	printBanner(cfg)

	qConn := rabbitmq.Reconnect(cfg.RabbitMQ)
	go consumer.ConsumeFromQueue(qConn, cfg, log, db)

	userService := services.NewUserSvc(db, log, cfg, qConn)

	grpcServer := grpc.NewServer()
	pb.RegisterUserServiceServer(grpcServer, userService)

	log.Debugf("✅ the user service started at: %v", cfg.Microservices.TheMonkeysUser+":"+fmt.Sprint(cfg.Microservices.UserPort))
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}

func BlogServiceConn(addr string) (*grpc.ClientConn, error) {
	log := logger.ZapForService("tm_users")
	log.Debugf("gRPC dialing to the blog server: %v", addr)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return conn, nil
}
