package main

import (
	"net"

	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_notification/pb"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/microservices/rabbitmq"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_notification/internal/consumer"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_notification/internal/database"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_notification/internal/services"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	cfg, err := config.GetConfig()
	if err != nil {
		logrus.Errorf("failed to load notification config, error: %+v", err)
	}
	log := logrus.New()

	db, err := database.NewNotificationDb(cfg, log)
	if err != nil {
		log.Fatalln("failed to connect to the database:", err)
	}

	lis, err := net.Listen("tcp", cfg.Microservices.TheMonkeysNotification)
	if err != nil {
		log.Errorf("failed to listen at port %v, error: %+v", cfg.Microservices.TheMonkeysNotification, err)
	}

	// Connect to rabbitmq server
	qConn := rabbitmq.Reconnect(cfg.RabbitMQ)
	go consumer.ConsumeFromQueue(qConn, cfg.RabbitMQ, log, db)

	notificationSvc := services.NewNotificationSvc(db, log, cfg)

	grpcServer := grpc.NewServer()

	pb.RegisterNotificationServiceServer(grpcServer, notificationSvc)

	log.Infof("✅ the notification service started at: %v", cfg.Microservices.TheMonkeysNotification)
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
