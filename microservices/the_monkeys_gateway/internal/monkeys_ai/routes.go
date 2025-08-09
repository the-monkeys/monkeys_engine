package monkeys_ai

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_recom/pb"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/internal/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// var upgrader = websocket.Upgrader{
// 	CheckOrigin: func(r *http.Request) bool {
// 		return true // Consider restricting this based on your use case
// 	},
// }

type RecommendationsClient struct {
	Client pb.RecommendationServiceClient
	log    *logrus.Logger
	pb.UnimplementedRecommendationServiceServer
}

func NewRecommendationsClient(cfg *config.Config) pb.RecommendationServiceClient {
	addr := cfg.Microservices.TheMonkeysAIEngine

	// Try connection with more detailed logging
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logrus.Errorf("❌ Cannot create gRPC Recommendations client: %v", err)
		return nil
	}

	logrus.Infof("✅ the monkeys gateway is successfully connected to Recommendations service at: %v", addr)
	return pb.NewRecommendationServiceClient(conn)
}

func RegisterRecommendationRoute(router *gin.Engine, cfg *config.Config, authClient *auth.ServiceClient, log *logrus.Logger) *RecommendationsClient {
	// mware := auth.InitAuthMiddleware(authClient)

	nsc := &RecommendationsClient{
		Client: NewRecommendationsClient(cfg),
		log:    log,
	}

	routes := router.Group("/api/v1/recommendations")
	// routes.Use(mware.AuthRequired)

	routes.GET("/by-username/:username", nsc.GetRecommendations)

	return nsc
}

func (rc *RecommendationsClient) GetRecommendations(ctx *gin.Context) {
	id := ctx.Param("username")
	if id == "" {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "User ID is required"})
		return
	}

	resp, err := rc.Client.GetRecommendations(ctx.Request.Context(), &pb.UserProfileReq{Username: id})
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to get recommendations"})
		return
	}

	ctx.JSON(200, resp)
}
