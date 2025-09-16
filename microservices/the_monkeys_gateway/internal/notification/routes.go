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
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_notification/pb"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_gateway/internal/auth"
	"go.uber.org/zap"
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
	log         *zap.SugaredLogger
}

// NewNotificationServiceClient creates a new instance of NotificationServiceClient
func NewNotificationServiceClient(cfg *config.Config, lg *zap.SugaredLogger) pb.NotificationServiceClient {
	notificationSvc := fmt.Sprintf("%s:%d", cfg.Microservices.TheMonkeysNotification, cfg.Microservices.NotificationPort)
	cc, err := grpc.NewClient(notificationSvc, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		lg.Errorw("cannot dial gRPC notification server", "err", err, "addr", notificationSvc)
		return nil
	}
	lg.Debugw("dialing notification service", "addr", notificationSvc)
	return pb.NewNotificationServiceClient(cc)
}

// RegisterNotificationRoute sets up the notification routes
func RegisterNotificationRoute(router *gin.Engine, cfg *config.Config, authClient *auth.ServiceClient, lg *zap.SugaredLogger) *NotificationServiceClient {
	mware := auth.InitAuthMiddleware(authClient, lg)

	nsc := &NotificationServiceClient{
		Client:      NewNotificationServiceClient(cfg, lg),
		connections: make(map[string][]*websocket.Conn), // Map of user ID to WebSocket connections
		log:         lg,
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
		nsc.log.Errorw("fetch notifications failed", "user", userName, "err", err)
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
		nsc.log.Debugw("no active websocket connections", "user", userID)
		return
	}

	// Step 2: Send the notification to each active connection
	for _, conn := range conns {
		if err := conn.WriteJSON(notification); err != nil {
			nsc.log.Errorw("send ws notification failed", "user", userID, "err", err)
			if err := conn.Close(); err != nil {
				nsc.log.Errorw("close ws conn failed", "user", userID, "err", err)
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
		nsc.log.Errorw("mark notification seen failed", "err", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark notification as seen"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Notification seen"})
}

func (nsc *NotificationServiceClient) GetNotificationsStream(ctx *gin.Context) {
	// Upgrade the HTTP connection to a WebSocket connection
	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		nsc.log.Errorw("upgrade to websocket failed", "err", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to establish WebSocket connection"})
		return
	}
	defer func() {
		if err := conn.Close(); err != nil {
			nsc.log.Errorw("close websocket failed", "err", err)
		}
	}()

	// Get the username from the context (assumes middleware sets this)
	userName := ctx.GetString("userName")
	if userName == "" {
		nsc.log.Errorw("unauthenticated websocket connection")
		_ = conn.WriteJSON(gin.H{"error": "Unauthorized"})
		return
	}
	nsc.log.Debugw("websocket connection established", "user", userName)

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
			nsc.log.Errorw("create gRPC stream failed", "user", userName, "err", err)
			_ = conn.WriteJSON(gin.H{"error": "Failed to establish notification stream"})
			return
		}

		// Receive notifications from the gRPC stream
		for {
			notification, err := stream.Recv()
			if err != nil {
				nsc.log.Debugw("no new notifications or stream closed", "user", userName)
				break // Exit inner loop if no new notifications or error occurs
			}

			// Check if the notification is new
			if notification.Notification[0].Id != lastNotificationID {
				// Update the last notification ID
				lastNotificationID = notification.Notification[0].Id

				// Forward the notification to the WebSocket client
				if err := conn.WriteJSON(notification); err != nil {
					nsc.log.Errorw("forward notification to ws client failed", "user", userName, "err", err)
					return
				}
				nsc.log.Debugw("notification forwarded", "user", userName)
			}
		}

		// Wait for a short period before checking for new notifications again
		select {
		case <-time.After(5 * time.Second): // Poll every 5 seconds
			continue
		case <-ctx.Done(): // Exit if the client disconnects
			nsc.log.Debugw("websocket connection closed by user", "user", userName)
			return
		}
	}
}
