package main

import (
	"log"
	"net"
	"time"

	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_blog/pb"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/microservices/rabbitmq"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_blog/internal/database"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_blog/internal/services"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

func main() {
	cfg, err := config.GetConfig()
	if err != nil {
		log.Fatalln("failed to load the config file, error: ", err)
		return
	}

	lis, err := net.Listen("tcp", cfg.Microservices.TheMonkeysBlog)
	if err != nil {
		log.Fatalf("article and service server failed to listen at port %v, error: %v",
			cfg.Microservices.TheMonkeysBlog, err)
		return
	}

	logger := logrus.New()

	osClient, err := database.NewOpenSearchClient(cfg.Opensearch.Host, cfg.Opensearch.Username, cfg.Opensearch.Password, logger)
	if err != nil {
		logger.Fatalf("cannot get the opensearch client, error: %v", err)
		return
	}

	var qConn rabbitmq.Conn
	for {
		qConn, err = rabbitmq.GetConn(cfg.RabbitMQ)
		if err != nil {
			logrus.Errorf("blog service cannot connect to rabbitMq service: %v", err)
			time.Sleep(time.Second)
			continue
		} else {
			logrus.Info("✅ blog service connected to rabbitMQ!")
			break
		}
	}

	blogService := services.NewBlogService(osClient, logger, cfg, qConn)
	// interservice := services.NewInterservice(*osClient, logger)

	grpcServer := grpc.NewServer()

	pb.RegisterBlogServiceServer(grpcServer, blogService)
	// isv.RegisterBlogServiceServer(grpcServer, interservice)

	logrus.Info("✅ the blog server started at: ", cfg.Microservices.TheMonkeysBlog)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalln("Failed to serve:", err)
		return
	}
}
