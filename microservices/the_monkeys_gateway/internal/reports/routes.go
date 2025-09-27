package reports

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_reports/pb"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/internal/auth"
	"go.uber.org/zap"
)

type ReportsServiceClient struct {
	Client pb.ReportsServiceClient
}


func (rsc *ReportsServiceClient) GetReports(ctx *gin.Context) {
}

func (rsc *ReportsServiceClient) CreateReport(ctx *gin.Context) {
	rsc.Client.CreateReport(context.Background(), &pb.CreateReportRequest{
		Data: &pb.Report{
		},
	})
}

func (rsc *ReportsServiceClient) FlagReport(ctx *gin.Context) {
}


func RegisterReportsServiceRoutes(router *gin.Engine, cfg *config.Config, authClient *auth.ServiceClient, lg *zap.SugaredLogger) *ReportsServiceClient {
	mware := auth.InitAuthMiddleware(authClient, lg)

	reportsServiceClient := &ReportsServiceClient{
	}

	routes := router.Group("/api/v1/reports")

	routes.Use(mware.AuthRequired)

	routes.GET("/", reportsServiceClient.GetReports)
	routes.POST("/", reportsServiceClient.CreateReport)
	routes.PATCH("/:report_id/flag", reportsServiceClient.FlagReport)

	return reportsServiceClient
}
