package main

import (
	"fmt"
	"net"
	"os"

	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_file_service/pb"
	"github.com/the-monkeys/the_monkeys/constants"

	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/microservices/rabbitmq"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_storage/constant"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_storage/internal/consumer"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_storage/internal/server"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
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
			logrus.Fatalf("Error creating folder path: %v", err)
		}
	}

	// Check if the blogPath directory already exists
	_, err = os.Stat(blogPath)

	// If the blogPath directory doesn't exist, create it with permissions 0755
	if os.IsNotExist(err) {
		err = os.MkdirAll(blogPath, 0755)
		if err != nil {
			logrus.Fatalf("Error creating blog path: %v", err)
		}
	}
}

func main() {
	cfg, err := config.GetConfig()
	if err != nil {
		logrus.Errorf("Failed to load file server config, error: %+v", err)
	}

	log := logrus.New()

	// Connect to rabbitmq server
	qConn := rabbitmq.Reconnect(cfg.RabbitMQ)
	go consumer.ConsumeFromQueue(qConn, cfg.RabbitMQ, log)

	host := fmt.Sprintf("%s:%d", cfg.Microservices.TheMonkeysFileStore, cfg.Microservices.StoragePort)
	lis, err := net.Listen("tcp", host)
	if err != nil {
		log.Errorf("File server failed to listen at port %v, error: %+v", host, err)
	}

	fileService := server.NewFileService(constant.BlogDir, constant.ProfileDir)
	// newFileServer := server.NewFileServer(common.PROFILE_PIC_DIR, common.BLOG_FILES, log)

	grpcServer := grpc.NewServer(grpc.MaxRecvMsgSize(constants.MaxMsgSize), grpc.MaxSendMsgSize(constants.MaxMsgSize))

	pb.RegisterUploadBlogFileServer(grpcServer, fileService)
	// fs.RegisterFileServiceServer(grpcServer, newFileServer)

	log.Infof("✅ the file storage server started at: %v:%d", cfg.Microservices.TheMonkeysFileStore, cfg.Microservices.StoragePort)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalln("Failed to serve:", err)
	}
}
