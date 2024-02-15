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
	BaseVMName        string `json:"base_vm_name"`
	GithubRunnerLabel string `json:"github_runner_label"`
}

func BindVM(c *gin.Context) {
	response := parseBody(c)
	if response == nil {
		return
	}

	result := models.CreateVM(response.BaseVMName, response.GithubRunnerLabel)
	if result.Error != nil {
		fmt.Println("Error create:", result.Error)
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Could not create VM"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func parseBody(c *gin.Context) *vmRequest {
	body, err := io.ReadAll(c.Request.Body)

	if err != nil {
		fmt.Println("Error read:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read request body"})
		return nil
	}

	var response vmRequest
	err = json.Unmarshal(body, &response)
	if err != nil {
		fmt.Println("Error parse:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse request body"})
		return nil
	}

	return &response
}
