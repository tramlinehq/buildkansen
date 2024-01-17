package web

import (
	"buildkansen/models"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
)

type vmRequest struct {
	VMIPAddress       string `json:"vm_ip_address"`
	GithubRunnerLabel string `json:"github_runner_label"`
}

func RegisterVM(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)

	if err != nil {
		fmt.Println("Error read:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read request body"})
		return
	}

	var response vmRequest
	err = json.Unmarshal(body, &response)
	if err != nil {
		fmt.Println("Error parse:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse request body"})
		return
	}

	result := models.CreateVM(response.VMIPAddress, response.GithubRunnerLabel)
	if result.Error != nil {
		fmt.Println("Error create:", err)
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Could not create VM"})
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}
