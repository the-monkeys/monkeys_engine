package main

import (
	"fmt"
	"net"
	"time"

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

	lis, err := net.Listen("tcp", cfg.Microservices.TheMonkeysAuthz)
	if err != nil {
		logrus.Fatalf("auth service cannot listen at address %s, error: %v", cfg.Microservices.TheMonkeysAuthz, err)
	}

	var qConn rabbitmq.Conn
	for {
		qConn, err = rabbitmq.GetConn(cfg.RabbitMQ)
		if err != nil {
			logrus.Errorf("auth service cannot connect to rabbitMq service: %v", err)
			time.Sleep(time.Second)
			continue
		} else {
			logrus.Errorf("✅ auth service connected to rabbitMQ!")
			break
		}
	}

	fmt.Printf("Successfully connected to rabbitMq services")

	authServer := services.NewAuthzSvc(dbHandler, jwt, cfg, qConn)

	grpcServer := grpc.NewServer()

	pb.RegisterAuthServiceServer(grpcServer, authServer)

	logrus.Info("✅ the authentication server started at address: ", cfg.Microservices.TheMonkeysAuthz)
	if err := grpcServer.Serve(lis); err != nil {
		logrus.Fatalf("gRPC auth server cannot start, error: %v", err)
	}
}
