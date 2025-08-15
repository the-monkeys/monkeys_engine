package notification

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

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
	log         *logrus.Logger
}

// NewNotificationServiceClient creates a new instance of NotificationServiceClient
func NewNotificationServiceClient(cfg *config.Config) pb.NotificationServiceClient {
	notificationSvc := fmt.Sprintf("%s:%d", cfg.Microservices.TheMonkeysNotification, cfg.Microservices.NotificationPort)
	cc, err := grpc.NewClient(notificationSvc, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logrus.Errorf("Cannot dial to gRPC notification server: %v", err)
		return nil
	}
	logrus.Debugf("✅ The monkeys gateway is dialing to notification service at: %v", notificationSvc)
	return pb.NewNotificationServiceClient(cc)
}

// RegisterNotificationRoute sets up the notification routes
func RegisterNotificationRoute(router *gin.Engine, cfg *config.Config, authClient *auth.ServiceClient, log *logrus.Logger) *NotificationServiceClient {
	mware := auth.InitAuthMiddleware(authClient)

	nsc := &NotificationServiceClient{
		Client:      NewNotificationServiceClient(cfg),
		connections: make(map[string][]*websocket.Conn), // Map of user ID to WebSocket connections
		log:         log,
	}

	routes := router.Group("/api/v1/notification")
	routes.Use(mware.AuthRequired)

	routes.POST("/notifications", nsc.CreateNotification)      // Create notifications
	routes.GET("/notifications", nsc.GetNotifications)         // Get notifications
	routes.GET("/ws-notification", nsc.GetNotificationsStream) // WebSocket endpoint
	routes.PUT("/notifications", nsc.ViewNotification)         // Get notifications

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
		nsc.log.Errorf("Error fetching notifications for user ID %s: %v", userName, err)
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

// NotifyUser sends notifications to a specific user via WebSocket
func (nsc *NotificationServiceClient) NotifyUser(userID string, notification []*pb.Notification) {
	nsc.mu.Lock()
	defer nsc.mu.Unlock()

	// Step 1: Check if the user has active WebSocket connections
	conns, ok := nsc.connections[userID]
	if !ok {
		nsc.log.Debugf("No active WebSocket connections for user ID: %s", userID)
		return
	}

	// Step 2: Send the notification to each active connection
	for _, conn := range conns {
		if err := conn.WriteJSON(notification); err != nil {
			nsc.log.Errorf("Error sending notification to user ID %s: %v", userID, err)
			if err := conn.Close(); err != nil {
				nsc.log.Errorf("Error closing WebSocket connection for user ID %s: %v", userID, err)
			}
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

func (nsc *NotificationServiceClient) ViewNotification(ctx *gin.Context) {
	var req pb.WatchNotificationReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Step 2: Mark the notification as seen in the database
	_, err := nsc.Client.NotificationSeen(context.Background(), &req)
	if err != nil {
		nsc.log.Errorf("Error marking notification as seen: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark notification as seen"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Notification seen"})
}

func (nsc *NotificationServiceClient) GetNotificationsStream(ctx *gin.Context) {
	// Upgrade the HTTP connection to a WebSocket connection
	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		nsc.log.Errorf("Failed to upgrade to WebSocket: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to establish WebSocket connection"})
		return
	}
	defer func() {
		if err := conn.Close(); err != nil {
			nsc.log.Errorf("Error closing WebSocket connection: %v", err)
		}
	}()

	// Get the username from the context (assumes middleware sets this)
	userName := ctx.GetString("userName")
	if userName == "" {
		nsc.log.Error("User not authenticated for WebSocket connection")
		if err := conn.WriteJSON(gin.H{"error": "Unauthorized"}); err != nil {
			nsc.log.Errorf("Error sending unauthorized message to WebSocket client: %v", err)
		}
		return
	}

	nsc.log.Debugf("WebSocket connection established for user: %s", userName)

	// Track the last notification ID to avoid sending duplicates
	var lastNotificationID string

	for {
		// Call the gRPC streaming method
		stream, err := nsc.Client.GetNotificationStream(context.Background(), &pb.GetNotificationReq{
			Username: userName,
			Limit:    10, // Fetch 10 notifications at a time
			Offset:   0,
		})
		if err != nil {
			nsc.log.Errorf("Failed to establish gRPC stream for user %s: %v", userName, err)
			if err := conn.WriteJSON(gin.H{"error": "Failed to establish notification stream"}); err != nil {
				nsc.log.Errorf("Error sending error message to WebSocket client: %v", err)
			}
			return
		}

		// Receive notifications from the gRPC stream
		for {
			notification, err := stream.Recv()
			if err != nil {
				nsc.log.Debugf("No new notifications or gRPC stream closed for user: %s", userName)
				break // Exit inner loop if no new notifications or error occurs
			}

			// Check if the notification is new
			if notification.Notification[0].Id != lastNotificationID {
				// Update the last notification ID
				lastNotificationID = notification.Notification[0].Id

				// Forward the notification to the WebSocket client
				if err := conn.WriteJSON(notification); err != nil {
					nsc.log.Errorf("Error sending notification to WebSocket client for user %s: %v", userName, err)
					return
				}
				nsc.log.Debugf("New notification sent to WebSocket client for user: %s", userName)
			}
		}

		// Wait for a short period before checking for new notifications again
		select {
		case <-time.After(5 * time.Second): // Poll every 5 seconds
			continue
		case <-ctx.Done(): // Exit if the client disconnects
			logrus.Debugf("WebSocket connection closed by user: %s", userName)
			return
		}
	}
}
