package main

import (
	"fmt"
	"net"

	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_authz/pb"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/microservices/rabbitmq"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_authz/internal/db"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_authz/internal/services"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_authz/internal/utils"
	"google.golang.org/grpc"
)

func main() {
	cfg, err := config.GetConfig()
	if err != nil {
		logrus.Errorf("cannot load auth service config, error: %v", err)
		return
	}

	dbHandler, err := db.NewAuthDBHandler(cfg)
	if err != nil {
		logrus.Fatalf("cannot connect the the db: %+v", err)
	}

	jwt := utils.JwtWrapper{
		SecretKey:       cfg.JWT.SecretKey,
		Issuer:          "go-grpc-auth-svc",
		ExpirationHours: 24 * 365,
	}

	host := fmt.Sprintf("%s:%d", cfg.Microservices.TheMonkeysAuthz, cfg.Microservices.AuthzPort)
	lis, err := net.Listen("tcp", host)
	if err != nil {
		logrus.Fatalf("auth service cannot listen at address %s, error: %v", host, err)
	}

	qConn := rabbitmq.Reconnect(cfg.RabbitMQ)

	authServer := services.NewAuthzSvc(dbHandler, jwt, cfg, qConn)

	grpcServer := grpc.NewServer()

	pb.RegisterAuthServiceServer(grpcServer, authServer)

	logrus.Info("✅ the authentication server started at address: ", host)
	if err := grpcServer.Serve(lis); err != nil {
		logrus.Fatalf("gRPC auth server cannot start, error: %v", err)
	}
}
