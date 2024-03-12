package web

import (
	"buildkansen/config"
	"buildkansen/internal/core"
	"buildkansen/internal/jobs"
	"buildkansen/models"
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
		Name            string      `json:"name"`
		HtmlUrl         string      `json:"html_url"`
		RunId           int64       `json:"run_id"`
		WorkflowName    string      `json:"workflow_name"`
		Status          string      `json:"status"`
		Conclusion      string      `json:"conclusion"`
		RunAttempt      int8        `json:"run_attempt"`
		CreatedAt       time.Time   `json:"created_at"`
		StartedAt       time.Time   `json:"started_at"`
		CompletedAt     time.Time   `json:"completed_at"`
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
	appError, newUser := core.CreateOrUpdateUser(uId, user.Name, user.Email)
	if appError != nil {
		c.JSON(appError.Code, gin.H{"error": appError.Message})
		return
	}

	session := sessions.Default(c)
	session.Set(config.C.AuthorizedUserInSessionKey, uId)
	_ = session.Save()

	if core.HasUserAlreadyInstalled(newUser) {
		c.Redirect(http.StatusFound, "/")
		return
	}

	c.Redirect(http.StatusFound, installationUrl())
}

func GithubAppsCallback(c *gin.Context) {
	fmt.Println("Received GitHub App callback:")
	queryParams := c.Request.URL.Query()

	for key, values := range queryParams {
		fmt.Printf("%s: %v\n", key, values)
	}

	if queryParams.Get("error") == "access_denied" {
		c.Redirect(http.StatusFound, "/")
		return
	}

	installationId, err := strconv.ParseInt(queryParams.Get("installation_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse installation id"})
		return
	}

	userValue, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not find a user in the session"})
		return
	}

	user, _ := userValue.(models.User)
	if core.HasUserAlreadyInstalled(&user) {
		c.Redirect(http.StatusFound, "/")
		return
	}

	appError := core.CreateInstallation(user.Id, installationId)
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

	if response.WorkflowJob.ID == 0 {
		if installationId != 0 && response.Installation.Account.ID != 0 {
			fmt.Println("Received an installation webhook, we don't handle this explicitly.")
		} else {
			fmt.Println("Received a webhook we don't handle explicitly.")
		}

		return
	}

	fmt.Printf("Received a workflow job webhook: %s", response.Action)
	runnerName, appError := core.FindValidRunnerName(response.WorkflowJob.Labels)
	if appError != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": appError.Message})
		return
	}

	installation, repository, appError := core.ValidateWorkflow(installationId, response.Repository.ID)
	if appError != nil {
		c.JSON(appError.Code, gin.H{"error": appError.Message})
		return
	}

	workflowJob := response.WorkflowJob

	switch response.Action {
	case "queued":
		fmt.Println("Processing the 'queued' workflow job...")
		jobs.NewJob(
			installation.AccountLogin,
			repository.InternalId,
			response.Repository.HtmlUrl,
			installationId,
			runnerName,
			workflowJob.RunId,
			workflowJob.WorkflowName,
			workflowJob.Status,
			workflowJob.ID,
			workflowJob.Name,
			workflowJob.HtmlUrl,
			workflowJob.StartedAt,
		).Process()
	case "completed":
		fmt.Println("Processing 'completed' workflow job...")
		core.CompleteWorkflow(
			workflowJob.ID,
			workflowJob.RunId,
			workflowJob.Status,
			repository.InternalId,
			workflowJob.CompletedAt)
	}

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
