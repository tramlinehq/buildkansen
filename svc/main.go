package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
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

type GithubInstallationResponseRepository struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Private  bool   `json:"private"`
}

type GithubInstallationResponseAccount struct {
	Login     string `json:"login"`
	ID        int64  `json:"id"`
	AvatarURL string `json:"avatar_url"`
	Type      string `json:"type"`
}

type GithubInstallationResponseInstallation struct {
	ID        int64                             `json:"id"`
	Account   GithubInstallationResponseAccount `json:"account"`
	CreatedAt time.Time                         `json:"created_at"`
	UpdatedAt time.Time                         `json:"updated_at"`
}

type GithubInstallationResponseSender struct {
	Login string `json:"login"`
	ID    int64  `json:"id"`
	Type  string `json:"type"`
}

type GithubInstallationResponse struct {
	Installation GithubInstallationResponseInstallation `json:"installation"`
	Repositories []GithubInstallationResponseRepository `json:"repositories"`
	Sender       GithubInstallationResponseSender       `json:"sender"`
	Action       string                                 `json:"action"`
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

	db.AutoMigrate(&User{})
	db.AutoMigrate(&Installation{})
	db.AutoMigrate(&Repository{})

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
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read request body"})
		return
	}

	fmt.Println("Received GitHub App callback:")
	fmt.Println(string(body))

	queryParams := c.Request.URL.Query()
	installationId, err := strconv.ParseInt(queryParams.Get("installation_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse installation id"})
		return
	}

	var jwtClient *goGithub.Client
	var client *goGithub.Client
	jwtClient, client, _ = InitGithubClients(installationId)
	installation, _, _ := jwtClient.Apps.GetInstallation(context.Background(), installationId)
	repos, _, _ := client.Apps.ListRepos(context.Background(), nil)
	session := ginSession.Default(c)
	updateInstallation(session.Get(authorizedUserKey).(int64), installation, repos)

	c.JSON(http.StatusOK, gin.H{"status": "success"})
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
	c.JSON(http.StatusOK, gin.H{"message": "you are home"})
}

func handleGithubHook(c *gin.Context) {
	// body, err := io.ReadAll(c.Request.Body)

	// if err != nil {
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read request body"})
	// 	return
	// }

	// var response GithubInstallationResponse
	// err = json.Unmarshal(body, &response)
	// if err != nil {
	// 	fmt.Println("Error:", err)
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse request body"})
	// 	return
	// }

	// fmt.Println("Received GitHub App webhook event:")
	// fmt.Println(string(body))

	// if response.Action == "created" {
	// 	err = updateInstallation(response, c)
	// 	if err != nil {
	// 		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	// 	}
	// } else {
	// 	fmt.Printf("Unhandled webhook event for app: %s", response.Action)
	// }

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func main() {
	initEnv()
	initGithubAuth()
	initDB(dbName)
	initServer()
}
