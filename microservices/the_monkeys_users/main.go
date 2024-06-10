package main

import (
	"net"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_user/pb"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/microservices/rabbitmq"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_users/internal/database"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_users/internal/services"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	cfg, err := config.GetConfig()
	if err != nil {
		logrus.Errorf("failed to load user config, error: %+v", err)
	}
	log := logrus.New()

	db, err := database.NewUserDbHandler(cfg, log)
	if err != nil {
		log.Fatalln("failed to connect to the database:", err)
	}

	lis, err := net.Listen("tcp", cfg.Microservices.TheMonkeysUser)
	if err != nil {
		log.Errorf("failed to listen at port %v, error: %+v", cfg.Microservices.TheMonkeysUser, err)
	}

	var qConn rabbitmq.Conn
	for {
		qConn, err = rabbitmq.GetConn(cfg.RabbitMQ)
		if err != nil {
			logrus.Errorf("auth service cannot connect to rabbitMq service: %v", err)
			time.Sleep(time.Second)
			continue
		} else {
			logrus.Info("✅ the user service connected to rabbitMQ!")
			break
		}
	}

	// conn, err := grpc.Dial(cfg.Microservices.TheMonkeysBlog, grpc.WithInsecure())
	// if err != nil {
	// 	log.Errorf("failed to dial to blog service at %v, error: %+v", cfg.Microservices.TheMonkeysBlog, err)
	// 	return
	// }

	// userService := database.NewUserDbHandler(db, log, isv.NewBlogServiceClient(conn))
	userService := services.NewUserSvc(db, log, cfg, qConn)

	grpcServer := grpc.NewServer()

	pb.RegisterUserServiceServer(grpcServer, userService)

	log.Infof("✅ the user service started at: %v", cfg.Microservices.TheMonkeysUser)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalln("Failed to serve:", err)
	}
}

func BlogServiceConn(addr string) (*grpc.ClientConn, error) {
	logrus.Infof("gRPC dialing to the blog server: %v", addr)
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	return conn, err
}
