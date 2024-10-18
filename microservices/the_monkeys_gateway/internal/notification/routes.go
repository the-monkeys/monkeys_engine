package notification

import (
	"net/http"

	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_notification/pb"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/internal/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all connections (you may want to restrict this)
	},
}

type Notification struct {
	ID      string `json:"id"`
	UserID  string `json:"user_id"`
	Message string `json:"message"`
	Status  string `json:"status"` // e.g., "sent", "read"
}

type NotificationServiceClient struct {
	Client      pb.NotificationServiceClient
	mu          sync.Mutex
	connections map[*websocket.Conn]bool
}

// NewNotificationServiceClient creates a new instance of NotificationServiceClient
func NewNotificationServiceClient(cfg *config.Config) pb.NotificationServiceClient {
	cc, err := grpc.NewClient(cfg.Microservices.TheMonkeysNotification, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logrus.Errorf("cannot dial to grpc notification server: %v", err)
		return nil
	}
	logrus.Infof("✅ the monkeys gateway is dialing to notification service at: %v", cfg.Microservices.TheMonkeysNotification)
	return pb.NewNotificationServiceClient(cc)
}

// RegisterUserRouter sets up the user notification routes
func RegisterNotificationRoute(router *gin.Engine, cfg *config.Config, authClient *auth.ServiceClient) *NotificationServiceClient {
	mware := auth.InitAuthMiddleware(authClient)

	usc := &NotificationServiceClient{
		Client:      NewNotificationServiceClient(cfg),
		connections: make(map[*websocket.Conn]bool),
	}

	routes := router.Group("/api/v1/notification")
	routes.Use(mware.AuthRequired)

	routes.POST("/notifications", usc.CreateNotification) // New route to create notifications
	routes.GET("/notifications", usc.GetNotifications)
	routes.GET("/ws", usc.handleWebSocket) // WebSocket endpoint

	return usc
}

// CreateNotification handles the creation of notifications
func (nsc *NotificationServiceClient) CreateNotification(ctx *gin.Context) {
	var notification Notification
	if err := ctx.ShouldBindJSON(&notification); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Logic to save the notification to the database or service
	// Assuming the notification is created successfully:
	nsc.NotifyClients(notification) // Notify all WebSocket clients

	ctx.JSON(http.StatusCreated, notification)
}

// GetNotifications retrieves notifications for the user
func (nsc *NotificationServiceClient) GetNotifications(ctx *gin.Context) {
	// Implement your logic to retrieve notifications here.
	// This can call the underlying service to get the user's notifications.
	ctx.JSON(http.StatusOK, gin.H{"message": "This will return notifications."})
}

// handleWebSocket handles WebSocket connections
func (nsc *NotificationServiceClient) handleWebSocket(ctx *gin.Context) {
	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		logrus.Errorf("Failed to upgrade connection: %v", err)
		return
	}
	defer conn.Close()

	// Register the connection
	nsc.mu.Lock()
	nsc.connections[conn] = true
	nsc.mu.Unlock()
	logrus.Infof("New WebSocket connection established")

	// Keep the connection alive
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}

	// Unregister the connection when it is closed
	nsc.mu.Lock()
	delete(nsc.connections, conn)
	nsc.mu.Unlock()
	logrus.Infof("WebSocket connection closed")
}

// NotifyClients sends notifications to all connected WebSocket clients
func (nsc *NotificationServiceClient) NotifyClients(notification Notification) {
	nsc.mu.Lock()
	defer nsc.mu.Unlock()

	for conn := range nsc.connections {
		if err := conn.WriteJSON(notification); err != nil {
			logrus.Errorf("Error sending notification: %v", err)
			conn.Close()
			delete(nsc.connections, conn)
		}
	}
}
