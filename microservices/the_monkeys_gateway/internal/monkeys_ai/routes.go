package monkeys_ai

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_recom/pb"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/internal/auth"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type RecommendationsClient struct {
	Client pb.RecommendationServiceClient
	log    *zap.SugaredLogger
	pb.UnimplementedRecommendationServiceServer
}

func NewRecommendationsClient(cfg *config.Config, lg *zap.SugaredLogger) pb.RecommendationServiceClient {
	addr := fmt.Sprintf("%s:%d", cfg.Microservices.TheMonkeysAIEngine, cfg.Microservices.AIEnginePort)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		lg.Errorw("cannot create gRPC recommendations client", "err", err, "addr", addr)
		return nil
	}

	lg.Debugw("connected to recommendations service", "addr", addr)
	return pb.NewRecommendationServiceClient(conn)
}

func RegisterRecommendationRoute(router *gin.Engine, cfg *config.Config, authClient *auth.ServiceClient, lg *zap.SugaredLogger) *RecommendationsClient {
	// mware := auth.InitAuthMiddleware(authClient)

	client := &RecommendationsClient{
		Client: NewRecommendationsClient(cfg, lg),
		log:    lg,
	}

	group := router.Group("/api/v1/recommendations")
	// group.Use(mware.AuthRequired)

	group.GET("/by-username/:username", client.GetRecommendations)

	return client
}

func (rc *RecommendationsClient) GetRecommendations(ctx *gin.Context) {
	id := ctx.Param("username")
	if id == "" {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "User ID is required"})
		return
	}

	resp, err := rc.Client.GetRecommendations(ctx.Request.Context(), &pb.UserProfileReq{Username: id})
	if err != nil {
		rc.log.Errorw("get recommendations failed", "user", id, "err", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to get recommendations"})
		return
	}

	ctx.JSON(200, resp)
}
