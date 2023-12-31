package web

import (
	"buildkansen/config"
	"buildkansen/db"
	githubApi "buildkansen/github"
	"buildkansen/models"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/markbates/goth/gothic"
	"gorm.io/gorm/clause"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"time"
)

type GithubActionsWorkflowWebhookEvent struct {
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
		CustomProperties struct {
		} `json:"custom_properties"`
	} `json:"repository"`
	Organization struct {
		Login string `json:"login"`
		ID    int64  `json:"id"`
	} `json:"organization"`
}

type Resource struct {
	VMUsername        string
	VMIPAddress       string
	SSHKeyPath        string
	GitHubToken       string
	GitHubRunnerLabel string
	RepoURL           string
}

const (
	kickOffScript = "../host/runner.kickoff"
	vmUsername    = "admin"
	vmIPAddress   = "192.168.64.6"
)

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
	u := models.User{Id: uId, Name: user.Name, Email: user.Email}
	result := db.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"email",
			"name",
		}),
	}).Create(&u)

	if result.Error != nil {
		c.JSON(500, gin.H{"error": "user already exists"})
		return
	}

	session := sessions.Default(c)
	session.Set(config.C.AuthorizedUserInSessionKey, uId)
	_ = session.Save()

	c.Redirect(http.StatusFound, githubInstallationUrl())
}

func GithubAppsCallback(c *gin.Context) {
	fmt.Println("Received GitHub App callback:")

	queryParams := c.Request.URL.Query()
	installationId, err := strconv.ParseInt(queryParams.Get("installation_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse installation id"})
		return
	}

	userValue, exists := c.Get("user")
	if exists {
		user, _ := userValue.(models.User)
		var installation models.Installation
		result := db.DB.Where("user_id = ? AND id = ?", user.Id, installationId).First(&installation)

		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update installation"})
			return
		}

		c.Redirect(http.StatusFound, "/")
		return
	}

	client, err := githubApi.NewClient(config.C.GithubAppId, installationId, config.C.GithubPrivateKeyBase64)
	session := sessions.Default(c)
	err = updateInstallation(session.Get(config.C.AuthorizedUserInSessionKey).(int64), client)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update installation"})
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

	var response GithubActionsWorkflowWebhookEvent
	err = json.Unmarshal(body, &response)
	if err != nil {
		fmt.Println("Error:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse request body"})
		return
	}

	installationId := response.Installation.ID

	if response.WorkflowJob.ID != 0 {
		fmt.Println("Received a workflow job event")

		if response.Action == "queued" {
			_, err := models.FindEntity(models.Installation{}, response.Organization.ID, "account_id")
			if err != nil {
				fmt.Println("could not find an installation for this webhook")
				return
			}
			repo, err := models.FindEntityById(models.Repository{}, response.Repository.ID)
			if err != nil {
				fmt.Println("could not find a repository for this webhook")
				return
			}

			client, err := githubApi.NewClient(config.C.GithubAppId, installationId, config.C.GithubPrivateKeyBase64)
			token, _, err := client.GetActionsRegistrationToken(response.Organization.Login, repo.(models.Repository).Name)

			if err != nil {
				fmt.Println("could not get registration token: ", err.Error())
				return
			}

			fmt.Println(*token.Token)

			macosVm := Resource{
				VMUsername:        vmUsername,
				VMIPAddress:       vmIPAddress,
				SSHKeyPath:        "id_rsa_bullet",
				GitHubToken:       *token.Token,
				GitHubRunnerLabel: "tramline-runner",
				RepoURL:           "https://github.com/tramlinehq/dump",
			}

			args := []string{
				"-u", macosVm.VMUsername,
				"-i", macosVm.VMIPAddress,
				"-s", macosVm.SSHKeyPath,
				"-t", macosVm.GitHubToken,
				"-l", macosVm.GitHubRunnerLabel,
				"-r", macosVm.RepoURL,
			}

			cmd := exec.Command(kickOffScript, args...)
			err = cmd.Run()

			if err != nil {
				fmt.Println("Error:", err)
			}

			fmt.Println("kicked off the runner.kickoff script!")
		}
	} else if installationId != 0 && response.Installation.Account.ID != 0 {
		fmt.Println("Received an installation event")
	} else {
		fmt.Println("Received a webhook, don't know how to handle rn")
	}

	fmt.Printf("Received GitHub App webhook event: %s", response.Action)
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func updateInstallation(userId int64, client *githubApi.Client) error {
	githubInstallation, _, _ := client.GetInstallation()
	githubRepositories, _, _ := client.GetInstallationRepos()

	tx := db.DB.Begin()

	installation := models.Installation{
		Id:               *githubInstallation.ID,
		AccountType:      *githubInstallation.Account.Type,
		AccountID:        *githubInstallation.Account.ID,
		AccountLogin:     *githubInstallation.Account.Login,
		AccountAvatarUrl: *githubInstallation.Account.AvatarURL,
		UserId:           userId,
	}
	result := tx.Create(&installation)

	if result.Error != nil {
		tx.Rollback()
		return errors.New("failed to save the installation")
	}

	for _, repo := range githubRepositories.Repositories {
		repository := models.Repository{
			Id:             *repo.ID,
			Name:           *repo.Name,
			FullName:       *repo.FullName,
			Private:        *repo.Private,
			InstallationId: installation.Id,
		}

		result := tx.Create(&repository)

		if result.Error != nil {
			tx.Rollback()
			return errors.New("failed to save the installation")
		}
	}

	tx.Commit()

	return nil
}

func githubInstallationUrl() string {
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
