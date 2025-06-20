package main

import (
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

func main() {
	router := gin.Default()

	router.GET("/stream/:filename", func(c *gin.Context) {
		filename := c.Param("filename")
		filePath := filepath.Join("path_to_your_videos", filename)

		c.Header("Content-Type", "video/mp4")
		c.Header("Transfer-Encoding", "chunked")

		c.Stream(func(w io.Writer) bool {
			file, err := os.Open(filePath)
			if err != nil {
				panic(err)
			}
			defer func() {
				if err := file.Close(); err != nil {
					log.Printf("Error closing file: %v", err)
				}
			}()

			_, err = io.Copy(w, file)
			return err == nil

		})
	})

	if err := router.Run(":8080"); err != nil {
		log.Fatalf("Cannot start the stream server: %v", err)
	}
}
