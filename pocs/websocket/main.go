package main

import (
	"log"

	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{}

func main() {
	r := gin.Default()

	r.GET("/blog", func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Println(err)
			return
		}

		// Infinite loop to listen to WebSocket connection
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				log.Println(err)
				return
			}

			// Process the blog message received from the client
			// In this example, we'll just log the message
			log.Printf("Received blog message: %s", msg)

			// Send a response message to the client (optional)
			response := "Blog message received!"
			err = conn.WriteMessage(websocket.TextMessage, []byte(response))
			if err != nil {
				log.Println(err)
				return
			}
		}
	})

	r.Run(":8080")
}

func fileUpdateRealTime() {
	r := gin.Default()

	r.GET("/blog", func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Println(err)
			return
		}

		// Create a new watcher
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			log.Fatal(err)
		}
		defer watcher.Close()

		// Watch the file for changes
		err = watcher.Add("local/blog.txt")
		if err != nil {
			log.Fatal(err)
		}

		go func() {
			for {
				select {
				case event, ok := <-watcher.Events:
					if !ok {
						return
					}
					if event.Op&fsnotify.Write == fsnotify.Write {
						msg := "File modified: " + event.Name
						err = conn.WriteMessage(websocket.TextMessage, []byte(msg))
						if err != nil {
							log.Println(err)
							return
						}
					}
				case err, ok := <-watcher.Errors:
					if !ok {
						return
					}
					log.Println("error:", err)
				}
			}
		}()

		// Infinite loop to listen to WebSocket connection
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				log.Println(err)
				return
			}

			// Process the blog message received from the client
			// In this example, we'll just log the message
			log.Printf("Received blog message: %s", msg)

			// Send a response message to the client (optional)
			response := "Blog message received!"
			err = conn.WriteMessage(websocket.TextMessage, []byte(response))
			if err != nil {
				log.Println(err)
				return
			}
		}
	})

	r.Run(":8080")
}
