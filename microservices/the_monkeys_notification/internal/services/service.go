package services

import (
	"context"

	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_notification/pb"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_notification/internal/database"
)

type NotificationSvc struct {
	db     database.NotificationDB
	log    *logrus.Logger
	config *config.Config
	pb.UnimplementedNotificationServiceServer
}

func NewNotificationSvc(dbConn database.NotificationDB, log *logrus.Logger, config *config.Config) *NotificationSvc {
	return &NotificationSvc{
		db:     dbConn,
		log:    log,
		config: config,
	}
}

func (ns *NotificationSvc) SendNotification(context.Context, *pb.SendNotificationReq) (*pb.SendNotificationRes, error) {
	panic("not implemented") // TODO: Implement
}
func (ns *NotificationSvc) GetNotification(context.Context, *pb.GetNotificationReq) (*pb.GetNotificationRes, error) {
	panic("not implemented") // TODO: Implement
}
func (ns *NotificationSvc) DeleteNotification(context.Context, *pb.DeleteNotificationReq) (*pb.DeleteNotificationRes, error) {
	panic("not implemented") // TODO: Implement
}
