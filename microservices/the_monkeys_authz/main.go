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
)

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

	jwt := utils.JwtWrapper{
		SecretKey:       cfg.JWT.SecretKey,
		Issuer:          "tm-authz",
		ExpirationHours: 24 * 365,
	}

	host := fmt.Sprintf("%s:%d", cfg.Microservices.TheMonkeysAuthz, cfg.Microservices.AuthzPort)
	lis, err := net.Listen("tcp", host)
	if err != nil {
		log.Fatalw("auth service cannot listen", "address", host, "error", err)
	}

	qConn := rabbitmq.Reconnect(cfg.RabbitMQ)

	authServer := services.NewAuthzSvc(dbHandler, jwt, cfg, qConn, log)

	grpcServer := grpc.NewServer()
	pb.RegisterAuthServiceServer(grpcServer, authServer)

	log.Infow("authentication server started", "address", host)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalw("gRPC auth server cannot start", "error", err)
	}
}
