package recommendations_client

import (
	"context"
	"net/http"
	"time"

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
	addr := cfg.Microservices.TheMonkeysRecommEngine

	// Create a longer timeout for development
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	// Try connection with more detailed logging
	cc, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock())

	if err != nil {
		logrus.Errorf("❌ Cannot dial to gRPC Recommendations server: %v", err)
		return nil
	}

	logrus.Infof("✅ the monkeys gateway is successfully connected to Recommendations service at: %v", addr)
	return pb.NewRecommendationServiceClient(cc)
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
