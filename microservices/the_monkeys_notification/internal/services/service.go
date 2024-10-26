package services

import (
	"context"
	"fmt"

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

func (ns *NotificationSvc) GetNotification(ctx context.Context, req *pb.GetNotificationReq) (*pb.GetNotificationRes, error) {
	ns.log.Infof("GetNotification request received for user: %s", req.Username)

	res, err := ns.db.GetUserNotifications(req.Username, req.Limit, req.Offset)
	if err != nil {
		return nil, err
	}

	fmt.Printf("res: %+v\n", res)
	var notifications []*pb.Notification
	for _, r := range res {
		notifications = append(notifications, &pb.Notification{
			UserId:  req.Username,
			Message: r.Message,
			Status:  r.DeliveryStatus,
		})
	}
	return &pb.GetNotificationRes{
		Notification: notifications,
	}, nil
}

func (ns *NotificationSvc) DeleteNotification(context.Context, *pb.DeleteNotificationReq) (*pb.DeleteNotificationRes, error) {
	panic("not implemented") // TODO: Implement
}
