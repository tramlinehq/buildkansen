package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"time"

	ginSession "github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	goGithub "github.com/google/go-github/v57/github"
	"github.com/gorilla/sessions"
	"github.com/joho/godotenv"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/github"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	appPort           = ":8081"
	certFilePath      = "./config/certs/localhost.pem"
	certKeyFilePath   = "./config/certs/localhost-key.pem"
	authorizedUserKey = "AUTHORIZED_USER_ID"
	kickOffScript     = "../host/kickoff"
	vmUsername        = "admin"
	vmIPAddress       = "192.168.64.6"
)

var (
	appEnv                       string
	dbName                       string
	sessionName                  string
	sessionSecret                string
	githubAppUrl                 string
	githubAppId                  int64
	githubClientID               string
	githubClientSecret           string
	githubAuthRedirectUrl        string
	githubAppRedirectUrl         string
	githubAppInstallationBaseUrl string
	githubPrivateKeyBase64       string
	db                           *gorm.DB
)

type User struct {
	Id            int64 `gorm:"primary_key"`
	Name          string
	Email         string    `gorm:"type:varchar(100);unique_index"`
	CreatedAt     time.Time `gorm:"autoCreateTime"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime"`
	DeletedAt     time.Time
	Installations []Installation `gorm:"foreignKey:UserId"`
}

type Installation struct {
	Id               int64 `gorm:"primary_key"`
	AccountType      string
	AccountID        int64
	AccountLogin     string
	AccountAvatarUrl string
	UserId           int64
	User             User         `gorm:"foreignKey:UserId;references:Id"`
	Repositories     []Repository `gorm:"foreignKey:InstallationId"`
	CreatedAt        time.Time    `gorm:"autoCreateTime"`
	UpdatedAt        time.Time    `gorm:"autoUpdateTime"`
	DeletedAt        time.Time
}

type Repository struct {
	Id             int64 `gorm:"primary_key"`
	Name           string
	FullName       string
	Private        bool
	InstallationId int64
	Installation   Installation `gorm:"foreignKey:InstallationId;references:Id"`
	CreatedAt      time.Time    `gorm:"autoCreateTime"`
	UpdatedAt      time.Time    `gorm:"autoUpdateTime"`
	DeletedAt      time.Time
}

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

func initEnv() {
	e := godotenv.Load()

	if e != nil {
		log.Fatalf("Error loading .env file: %s", e)
	}

	appEnv = os.Getenv("ENV")
	dbName = os.Getenv("APP_NAME")
	sessionName = os.Getenv("APP_NAME")
	sessionSecret = os.Getenv("SESSION_SECRET")
	githubAppUrl = os.Getenv("GITHUB_APP_URL")
	githubAppId, _ = strconv.ParseInt(os.Getenv("GITHUB_APP_ID"), 10, 64)
	githubClientID = os.Getenv("GITHUB_CLIENT_ID")
	githubClientSecret = os.Getenv("GITHUB_CLIENT_SECRET")
	githubAuthRedirectUrl = os.Getenv("GITHUB_AUTH_REDIRECT_URL")
	githubAppRedirectUrl = os.Getenv("GITHUB_APP_REDIRECT_URL")
	githubPrivateKeyBase64 = os.Getenv("GITHUB_PRIVATE_KEY_BASE64")
	githubAppInstallationBaseUrl = os.Getenv("GITHUB_NEW_INSTALLATION_URL")
}

func initDB(name string) *gorm.DB {
	var err error
	db, err = gorm.Open(sqlite.Open(name), &gorm.Config{})
	if err != nil {
		panic(err)
	}

	if db.AutoMigrate(&User{}) != nil {
		panic("couldn't migrate User table")
	}
	if db.AutoMigrate(&Installation{}) != nil {
		panic("couldn't migrate Installation table")
	}
	if db.AutoMigrate(&Repository{}) != nil {
		panic("couldn't migrate Repository table")
	}

	return db
}

func initGithubAuth() {
	gothic.Store = sessions.NewCookieStore([]byte(sessionSecret))
	goth.UseProviders(
		github.New(githubClientID, githubClientSecret, githubAuthRedirectUrl),
	)
}

func tapProviderParam(provider string) gin.HandlerFunc {
	return func(c *gin.Context) {
		q := c.Request.URL.Query()
		q.Add("provider", provider)
		c.Request.URL.RawQuery = q.Encode()
	}
}

func initServer() {
	r := gin.Default()

	r.Use(ginSession.Sessions(sessionName, cookie.NewStore([]byte(sessionSecret))))
	r.Static("/assets", "./assets")
	r.LoadHTMLGlob("views/*")

	r.GET("/", handleHome)
	r.GET("/github/auth", tapProviderParam("github"), handleGithubAuth)
	r.GET("/github/auth/register", tapProviderParam("github"), handleGithubAuthCallback)
	r.GET("/github/app/register", tapProviderParam("github"), handleGithubAppCallback)
	r.POST("/github/hook", handleGithubHook)

	var err error
	if appEnv == "production" {
		err = r.Run(appPort)
	} else {
		err = r.RunTLS(appPort, certFilePath, certKeyFilePath)
	}

	if err != nil {
		fmt.Println(err)
	}
}

func handleGithubAuth(c *gin.Context) {
	q := c.Request.URL.Query()
	q.Add("provider", "github")
	c.Request.URL.RawQuery = q.Encode()
	gothic.BeginAuthHandler(c.Writer, c.Request)
}

func handleGithubAuthCallback(c *gin.Context) {
	user, err := gothic.CompleteUserAuth(c.Writer, c.Request)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	uId, _ := strconv.ParseInt(user.UserID, 10, 64)
	u := User{Id: uId, Name: user.Name, Email: user.Email}
	result := db.Clauses(clause.OnConflict{
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

	session := ginSession.Default(c)
	session.Set(authorizedUserKey, uId)
	session.Save()

	c.Redirect(http.StatusFound, githubInstallationUrl())
}

func githubInstallationUrl() string {
	u := &url.URL{
		Scheme: "https",
		Host:   "github.com",
		Path:   githubAppInstallationBaseUrl,
	}

	rq := u.Query()

	rq.Set("state", "1")
	rq.Set("redirect_uri", githubAppRedirectUrl)

	u.RawQuery = rq.Encode()

	return u.String()
}

func handleGithubAppCallback(c *gin.Context) {
	fmt.Println("Received GitHub App callback:")

	queryParams := c.Request.URL.Query()
	installationId, err := strconv.ParseInt(queryParams.Get("installation_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse installation id"})
		return
	}

	var jwtClient *goGithub.Client
	var client *goGithub.Client
	jwtClient, client, _ = InitGithubClients(installationId)
	installation, _, _ := getInstallation(c, jwtClient, installationId)
	repos, _, _ := getInstallationRepos(c, client)
	session := ginSession.Default(c)
	err = updateInstallation(session.Get(authorizedUserKey).(int64), installation, repos)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update installation"})
		return
	}

	c.Redirect(http.StatusFound, "/")
}

func updateInstallation(userId int64, ghInstallation *goGithub.Installation, ghRepos *goGithub.ListRepositories) error {
	tx := db.Begin()

	installation := Installation{
		Id:               *ghInstallation.ID,
		AccountType:      *ghInstallation.Account.Type,
		AccountID:        *ghInstallation.Account.ID,
		AccountLogin:     *ghInstallation.Account.Login,
		AccountAvatarUrl: *ghInstallation.Account.AvatarURL,
		UserId:           userId,
	}
	result := tx.Create(&installation)

	if result.Error != nil {
		tx.Rollback()
		return errors.New("failed to save the installation")
	}

	for _, repo := range ghRepos.Repositories {
		repository := Repository{
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

func handleHome(c *gin.Context) {
	user, err := getUserFromSession(c)
	/*	localSession := ginSession.Default(c)
		localSession.Save()
	*/

	if err != nil {
		c.HTML(http.StatusOK, "index.html", gin.H{})
	} else {
		c.HTML(http.StatusOK, "index.html", gin.H{"user": user})
	}
}

func handleGithubHook(c *gin.Context) {
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
			_, err := findEntity(response.Organization.ID, "account_id", Installation{})
			if err != nil {
				fmt.Println("could not find an installation for this webhook")
				return
			}
			repo, err := findEntity(response.Repository.ID, "id", Repository{})
			if err != nil {
				fmt.Println("could not find a repository for this webhook")
				return
			}

			_, client, _ := InitGithubClients(installationId)
			token, _, err := getActionsRegistrationToken(c, client, response.Organization.Login, repo.(Repository).Name)

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

			fmt.Println("kick'd off the kickoff script!")
		}
	} else if installationId != 0 && response.Installation.Account.ID != 0 {
		fmt.Println("Received an installation event")
	} else {
		fmt.Println("Received a webhook, don't know how to handle rn")
	}

	fmt.Printf("Received GitHub App webhook event: %s", response.Action)
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

type Entity interface {
	Installation | Repository | User
}

func findEntity[U Entity](value int64, by string, model U) (interface{}, error) {
	condition := by + " = ?"
	result := db.Where(condition, value).First(&model)

	if result.Error != nil {
		return nil, result.Error
	}

	return model, nil
}

func getUserFromSession(c *gin.Context) (interface{}, error) {
	localSession := ginSession.Default(c)
	uId := localSession.Get(authorizedUserKey)

	if uId == nil {
		return nil, fmt.Errorf("no user found")
	}

	result, err := findEntity(uId.(int64), "id", User{})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func getUserFromSessionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := getUserFromSession(c)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("no user found"))
			return
		}

		c.Set("user", user)
		c.Next()
	}
}

func main() {
	initEnv()
	initGithubAuth()
	initDB(dbName)
	initServer()
}
