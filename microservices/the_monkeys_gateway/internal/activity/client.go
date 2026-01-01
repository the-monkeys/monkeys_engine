package activity

import (
	"fmt"

	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_activity/pb"
	"github.com/the-monkeys/the_monkeys/config"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewActivityServiceClient(cfg *config.Config, lg *zap.SugaredLogger) pb.ActivityServiceClient {
	addr := fmt.Sprintf("%s:%d", cfg.Microservices.TheMonkeysActivity, cfg.Microservices.ActivityPort)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		lg.Errorw("cannot create gRPC activity client", "err", err, "addr", addr)
		return nil
	}

	lg.Debugw("connected to activity service", "addr", addr)
	return pb.NewActivityServiceClient(conn)
}
