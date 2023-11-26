package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
)

const (
	appPort = ":8080"
)

func initServer() {
	r := gin.Default()
	r.GET("/", handleHome())

	var err error
	err = r.Run(appPort)
	if err != nil {
		fmt.Println(err)
	}
}

func handleHome() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "you are home"})
	}
}

func main() {
	initServer()
}
