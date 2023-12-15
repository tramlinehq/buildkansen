package web

import (
	"buildkansen/models"
	"github.com/gin-gonic/gin"
	"net/http"
)

func HandleHome(c *gin.Context) {
	userValue, exists := c.Get("user")

	if exists {
		user, _ := userValue.(models.User)
		c.HTML(http.StatusOK, "index.html", gin.H{"user": user})

	} else {
		c.HTML(http.StatusOK, "index.html", gin.H{})
	}
}

func RegisterGuestVM(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}
