package web

import (
	"buildkansen/config"
	"buildkansen/internal/core"
	"buildkansen/internal/jobs"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/markbates/goth/gothic"
)

type githubActionsWorkflowWebhookEvent struct {
	Action       string `json:"action"`
	Installation struct {
		ID      int64 `json:"id"`
		Account struct {
			Login     string `json:"login"`
			ID        int64  `json:"id"`
			AvatarURL string `json:"avatar_url"`
			Type      string `json:"type"`
		} `json:"account"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	} `json:"installation"`
	Sender struct {
		Login string `json:"login"`
		ID    int64  `json:"id"`
		Type  string `json:"type"`
	} `json:"sender"`
	Repositories []struct {
		ID       int64  `json:"id"`
		Name     string `json:"name"`
		FullName string `json:"full_name"`
		Private  bool   `json:"private"`
	} `json:"repositories"`
	WorkflowJob struct {
		ID              int64       `json:"id"`
		RunId           int64       `json:"run_id"`
		WorkflowName    string      `json:"workflow_name"`
		Status          string      `json:"status"`
		Conclusion      string      `json:"conclusion"`
		CreatedAt       time.Time   `json:"created_at"`
		StartedAt       time.Time   `json:"started_at"`
		CompletedAt     interface{} `json:"completed_at"`
		Labels          []string    `json:"labels"`
		RunnerId        interface{} `json:"runner_id"`
		RunnerName      interface{} `json:"runner_name"`
		RunnerGroupId   interface{} `json:"runner_group_id"`
		RunnerGroupName interface{} `json:"runner_group_name"`
	} `json:"workflow_job"`
	Repository struct {
		ID               int64  `json:"id"`
		DefaultBranch    string `json:"default_branch"`
		HtmlUrl          string `json:"html_url"`
		CustomProperties struct {
		} `json:"custom_properties"`
	} `json:"repository"`
	Organization struct {
		Login string `json:"login"`
		ID    int64  `json:"id"`
	} `json:"organization"`
}

func GithubAuth(c *gin.Context) {
	gothic.BeginAuthHandler(c.Writer, c.Request)
}

func GithubAuthCallback(c *gin.Context) {
	user, err := gothic.CompleteUserAuth(c.Writer, c.Request)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	uId, _ := strconv.ParseInt(user.UserID, 10, 64)
	appError := core.CreateOrUpdateUser(uId, user.Name, user.Email)
	if appError != nil {
		c.JSON(appError.Code, gin.H{"error": appError.Message})
		return
	}

	session := sessions.Default(c)
	session.Set(config.C.AuthorizedUserInSessionKey, uId)
	_ = session.Save()

	c.Redirect(http.StatusFound, installationUrl())
}

func GithubAppsCallback(c *gin.Context) {
	fmt.Println("Received GitHub App callback:")

	queryParams := c.Request.URL.Query()
	installationId, err := strconv.ParseInt(queryParams.Get("installation_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse installation id"})
		return
	}

	session := sessions.Default(c)
	appError := core.CreateInstallation(session.Get(config.C.AuthorizedUserInSessionKey).(int64), installationId)
	if appError != nil {
		c.JSON(appError.Code, gin.H{"error": appError.Message})
		return
	}

	c.Redirect(http.StatusFound, "/")
}

func GithubHook(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read request body"})
		return
	}

	var response githubActionsWorkflowWebhookEvent
	err = json.Unmarshal(body, &response)
	if err != nil {
		fmt.Println("Error:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse request body"})
		return
	}

	installationId := response.Installation.ID

	if response.WorkflowJob.ID != 0 {
		fmt.Println("Received a workflow job event")

		runnerName, appError := core.FindValidRunnerName(response.WorkflowJob.Labels)
		if appError != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": appError.Message})
			return
		}

		appError = core.ValidateWorkflow(response.Organization.ID, response.Repository.ID)
		if appError != nil {
			c.JSON(appError.Code, gin.H{"error": appError.Message})
			return
		}

		if response.Action == "queued" {
			fmt.Println("Received a queued workflow job event for tramline runner")
			job := jobs.NewJob(response.Organization.Login, response.Repository.ID, response.Repository.HtmlUrl, installationId, runnerName, response.WorkflowJob.RunId)
			job.Process()
		} else if response.Action == "completed" {
			fmt.Println("Received a completed workflow job event for tramline runner")
			core.CompleteWorkflow(response.WorkflowJob.RunId, response.Repository.ID)
		}
	} else if installationId != 0 && response.Installation.Account.ID != 0 {
		fmt.Println("Received an installation event")
	} else {
		fmt.Println("Received a webhook, don't know how to handle rn")
	}

	fmt.Printf("Received GitHub App webhook event: %s", response.Action)
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func installationUrl() string {
	u := &url.URL{
		Scheme: "https",
		Host:   "github.com",
		Path:   config.C.GithubAppInstallationBaseUrl,
	}
	rq := u.Query()
	rq.Set("state", "1") // TODO: Add additional state as necessary
	rq.Set("redirect_uri", config.C.GithubAppRedirectUrl)
	u.RawQuery = rq.Encode()

	return u.String()
}
