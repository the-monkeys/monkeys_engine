package main

// import (
// 	"net/http"
// 	"sync"

// 	"github.com/gin-gonic/gin"
// )

// var (
// 	// upgraderr  = websocket.Upgrader{}
// 	connMutex  sync.Mutex
// 	connStatus = make(map[string]bool)
// )

// func main1() {
// 	r := gin.Default()

// 	r.GET("/ws/:userId", func(c *gin.Context) {
// 		userId := c.Param("userId")
// 		wsHandler(c.Writer, c.Request, userId)
// 	})

// 	r.GET("/status/:userId", func(c *gin.Context) {
// 		userId := c.Param("userId")
// 		c.JSON(http.StatusOK, gin.H{
// 			"userId":   userId,
// 			"isOnline": isUserOnline(userId),
// 		})
// 	})

// 	r.Run(":8080")
// }

// func wsHandler(w http.ResponseWriter, r *http.Request, userId string) {
// 	conn, err := upgraderr.Upgrade(w, r, nil)
// 	if err != nil {
// 		http.Error(w, "Could not open websocket connection", http.StatusBadRequest)
// 		return
// 	}
// 	defer conn.Close()

// 	setUserOnlineStatus(userId, true)
// 	defer setUserOnlineStatus(userId, false)

// 	for {
// 		_, _, err := conn.ReadMessage()
// 		if err != nil {
// 			break
// 		}
// 	}
// }

// func setUserOnlineStatus(userId string, isOnline bool) {
// 	connMutex.Lock()
// 	connStatus[userId] = isOnline
// 	connMutex.Unlock()
// 	updateDatabaseStatus(userId, isOnline)
// }

// func isUserOnline(userId string) bool {
// 	connMutex.Lock()
// 	defer connMutex.Unlock()
// 	return connStatus[userId]
// }

// func updateDatabaseStatus(userId string, isOnline bool) {
// 	// Update the user_account_status table
// 	// Assume you have a function updateStatus(userId string, isOnline bool)
// 	go updateStatus(userId, isOnline)
// }

// func updateStatus(userId string, isOnline bool) {
// 	// Perform the database update operation
// 	// For example, using GORM:
// 	// db.Model(&UserAccountStatus{}).Where("user_id = ?", userId).Update("is_online", isOnline)
// }
