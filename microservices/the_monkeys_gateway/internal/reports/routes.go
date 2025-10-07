package reports

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/the-monkeys/the_monkeys/config"
	pb "github.com/the-monkeys/the_monkeys/generated/go"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/internal/auth"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ReportsServiceClient struct {
	Client pb.ReportsServiceClient
	config *config.Config
	log    *zap.SugaredLogger
}

func NewReportsServiceClient(cfg *config.Config, lg *zap.SugaredLogger) pb.ReportsServiceClient {
	addr := fmt.Sprintf("%s:%d", cfg.Microservices.ReportsService, cfg.Microservices.ReportsServicePort)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		lg.Errorw("cannot create gRPC recommendations client", "err", err, "addr", addr)
		return nil
	}

	lg.Debug("connected to recommendations service", "addr", addr)
	return pb.NewReportsServiceClient(conn)
}

func (rsc *ReportsServiceClient) GetReports(ctx *gin.Context) {
}

func (rsc *ReportsServiceClient) CreateReport(ctx *gin.Context) {
	type requestBody struct {
		ReasonType    string `json:"reason_type"`
		ReporterId    string `json:"reporter_id"`
		ReportedType  string `json:"reported_type"`
		ReportedId    string `json:"reported_id"`
		ReporterNotes string `json:"reporter_notes"`
	}

	var body requestBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		rsc.log.Error("Error parsing request body", err.Error())

		ctx.JSON(422, gin.H{
			"status":  "error",
			"message": "err",
		})
		return
	}

	rsc.log.Info("Received body: ", body)

	res, err := rsc.Client.CreateReport(context.Background(), &pb.CreateReportRequest{
		ReporterId:    body.ReporterId,
		ReasonType:    body.ReasonType,
		ReportedType:  body.ReportedType,
		ReporterNotes: body.ReporterNotes,
		ReportedId:    body.ReportedId,
	})
	rsc.log.Info(res)

	if err != nil {
		rsc.log.Error(err.Error())
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}
}

func (rsc *ReportsServiceClient) FlagReport(ctx *gin.Context) {
}

func RegisterReportsServiceRoutes(router *gin.Engine, cfg *config.Config, authClient *auth.ServiceClient, lg *zap.SugaredLogger) *ReportsServiceClient {
	mware := auth.InitAuthMiddleware(authClient, lg)

	reportsServiceClient := &ReportsServiceClient{
		Client: NewReportsServiceClient(cfg, lg),
		config: cfg,
		log:    lg,
	}

	routes := router.Group("/api/v1/reports")

	routes.Use(mware.AuthRequired)

	//	routes.GET("/", reportsServiceClient.GetReports)
	routes.POST("/", reportsServiceClient.CreateReport)
	routes.PATCH("/:report_id/flag", reportsServiceClient.FlagReport)

	return reportsServiceClient
}
