package services

import (
	"context"
	"net/http"
	"strconv"

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

	var notifications []*pb.Notification
	for _, r := range res {
		s1 := strconv.FormatInt(int64(r.ID), 10)
		notifications = append(notifications, &pb.Notification{
			Id:      s1,
			UserId:  req.Username,
			Message: r.Message,
			Status:  r.DeliveryStatus,
			Seen:    r.Seen,
		})
	}
	return &pb.GetNotificationRes{
		Notification: notifications,
	}, nil
}

func (ns *NotificationSvc) NotificationSeen(ctx context.Context, req *pb.WatchNotificationReq) (*pb.NotificationResponse, error) {
	ns.log.Infof("NotificationSeen request received for user: %s", req.UserId)

	ids := make([]int64, 0)
	for _, n := range req.Notification {
		// Convert req.Id into int64
		id, err := strconv.ParseInt(n.Id, 10, 64)
		if err != nil {
			ns.log.Errorf("Error converting notification ID to int64: %v", err)
			return nil, err
		}

		ids = append(ids, id)
	}

	err := ns.db.MarkNotificationAsSeen(ids, req.UserId)
	if err != nil {
		return nil, err
	}

	return &pb.NotificationResponse{
		Status:  http.StatusOK,
		Message: "Notification seen",
	}, nil
}

func (ns *NotificationSvc) DeleteNotification(context.Context, *pb.DeleteNotificationReq) (*pb.DeleteNotificationRes, error) {
	panic("not implemented") // TODO: Implement
}
