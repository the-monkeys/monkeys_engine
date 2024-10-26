package notification

import (
	"context"
	"net/http"
	"strconv"
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
		return true // Consider restricting this based on your use case
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
	connections map[string][]*websocket.Conn // Map user ID to WebSocket connections
}

// NewNotificationServiceClient creates a new instance of NotificationServiceClient
func NewNotificationServiceClient(cfg *config.Config) pb.NotificationServiceClient {
	cc, err := grpc.Dial(cfg.Microservices.TheMonkeysNotification, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logrus.Errorf("Cannot dial to gRPC notification server: %v", err)
		return nil
	}
	logrus.Infof("✅ The monkeys gateway is dialing to notification service at: %v", cfg.Microservices.TheMonkeysNotification)
	return pb.NewNotificationServiceClient(cc)
}

// RegisterNotificationRoute sets up the notification routes
func RegisterNotificationRoute(router *gin.Engine, cfg *config.Config, authClient *auth.ServiceClient) *NotificationServiceClient {
	mware := auth.InitAuthMiddleware(authClient)

	nsc := &NotificationServiceClient{
		Client:      NewNotificationServiceClient(cfg),
		connections: make(map[string][]*websocket.Conn), // Map of user ID to WebSocket connections
	}

	routes := router.Group("/api/v1/notification")
	routes.Use(mware.AuthRequired)

	routes.POST("/notifications", nsc.CreateNotification) // Create notifications
	routes.GET("/notifications", nsc.GetNotifications)    // Get notifications
	routes.GET("/ws", nsc.handleWebSocket)                // WebSocket endpoint

	return nsc
}

// CreateNotification handles the creation of notifications
func (nsc *NotificationServiceClient) CreateNotification(ctx *gin.Context) {
	var notification Notification
	if err := ctx.ShouldBindJSON(&notification); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Notify the user via WebSocket
	// nsc.NotifyUser(notification.UserID, notification)

	ctx.JSON(http.StatusCreated, notification)
}

// GetNotifications retrieves notifications for the user and pushes them via WebSocket if connected
func (nsc *NotificationServiceClient) GetNotifications(ctx *gin.Context) {
	userName := ctx.GetString("userName")
	// get params like limit, offset, etc.
	limit := ctx.Query("limit")
	offset := ctx.Query("offset")

	// Convert to int64
	limitInt, err := strconv.ParseInt(limit, 10, 64)
	if err != nil {
		limitInt = 10
	}
	offsetInt, err := strconv.ParseInt(offset, 10, 64)
	if err != nil {
		offsetInt = 0
	}

	// Step 2: Fetch notifications from the database for the user (e.g., only unread ones)
	notifications, err := nsc.Client.GetNotification(context.Background(), &pb.GetNotificationReq{
		Username: userName,
		Limit:    limitInt,
		Offset:   offsetInt,
	})
	if err != nil {
		logrus.Errorf("Error fetching notifications for user ID %s: %v", userName, err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch notifications"})
		return
	}

	// Step 3: Send notifications via WebSocket if the user has an active connection
	if len(notifications.Notification) > 0 {
		nsc.NotifyUser(userName, notifications.Notification)
	}

	// Step 4: Return the notifications via HTTP as well
	ctx.JSON(http.StatusOK, gin.H{"notifications": notifications})
}

// handleWebSocket handles WebSocket connections
func (nsc *NotificationServiceClient) handleWebSocket(ctx *gin.Context) {
	// Step 1: Extract user information from the context (Assuming middleware injects userName)
	userName := ctx.GetString("userName")

	// Step 2: Upgrade the HTTP connection to a WebSocket
	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		logrus.Errorf("Failed to upgrade connection: %v", err)
		return
	}
	defer conn.Close()

	// Step 3: Register the connection for the user
	nsc.mu.Lock()
	nsc.connections[userName] = append(nsc.connections[userName], conn)
	nsc.mu.Unlock()
	logrus.Infof("New WebSocket connection established for user: %s", userName)

	// Step 4: Send a "logged in" message to the user
	initialMessage := "You are logged in!"
	if err := conn.WriteMessage(websocket.TextMessage, []byte(initialMessage)); err != nil {
		logrus.Errorf("Error sending initial message to user %s: %v", userName, err)
		return
	}

	// Keep the connection alive (handle incoming messages)
	for {
		// Wait for incoming messages or ping/pong
		messageType, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logrus.Errorf("Unexpected WebSocket closure for user %s: %v", userName, err)
			}
			break
		}

		// Respond to pings or other control messages to keep connection alive
		if messageType == websocket.PingMessage {
			if err := conn.WriteMessage(websocket.PongMessage, nil); err != nil {
				logrus.Errorf("Error sending pong to user %s: %v", userName, err)
				break
			}
		}
	}

	// Step 5: Unregister the connection when closed
	nsc.mu.Lock()
	conns := nsc.connections[userName]
	for i, c := range conns {
		if c == conn {
			nsc.connections[userName] = append(conns[:i], conns[i+1:]...)
			break
		}
	}
	nsc.mu.Unlock()
	logrus.Infof("WebSocket connection closed for user: %s", userName)
}

// NotifyUser sends notifications to a specific user via WebSocket
func (nsc *NotificationServiceClient) NotifyUser(userID string, notification []*pb.Notification) {
	nsc.mu.Lock()
	defer nsc.mu.Unlock()

	// Step 1: Check if the user has active WebSocket connections
	conns, ok := nsc.connections[userID]
	if !ok {
		logrus.Infof("No active WebSocket connections for user ID: %s", userID)
		return
	}

	// Step 2: Send the notification to each active connection
	for _, conn := range conns {
		if err := conn.WriteJSON(notification); err != nil {
			logrus.Errorf("Error sending notification to user ID %s: %v", userID, err)
			conn.Close()
			// Remove closed connection from list
			nsc.connections[userID] = removeConn(nsc.connections[userID], conn)
		}
	}
}

// removeConn removes a WebSocket connection from the list
func removeConn(conns []*websocket.Conn, conn *websocket.Conn) []*websocket.Conn {
	for i, c := range conns {
		if c == conn {
			return append(conns[:i], conns[i+1:]...)
		}
	}
	return conns
}
