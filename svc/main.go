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
	"time"

	"github.com/gin-gonic/gin"
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
	appPort         = ":8081"
	certFilePath    = "./config/certs/localhost.pem"
	certKeyFilePath = "./config/certs/localhost-key.pem"
)

var (
	appEnv                       string
	dbName                       string
	githubAppUrl                 string
	githubAppId                  string
	githubClientID               string
	githubClientSecret           string
	githubSessionSecret          string
	githubAuthRedirectUrl        string
	githubAppRedirectUrl         string
	githubAppInstallationBaseUrl string
	githubPrivateKeyBase64       string
	db                           *gorm.DB
)

type User struct {
	Id            string `gorm:"primary_key"`
	Name          string
	Email         string    `gorm:"type:varchar(100);unique_index"`
	CreatedAt     time.Time `gorm:"autoCreateTime"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime"`
	DeletedAt     time.Time
	Installations []Installation `gorm:"foreignKey:UserId"`
}

type Installation struct {
	Id               string `gorm:"primary_key"`
	AccountType      string
	AccountID        string
	AccountLogin     string
	AccountAvatarUrl string
	UserId           string
	User             User         `gorm:"foreignKey:UserId;references:Id"`
	Repositories     []Repository `gorm:"foreignKey:InstallationId"`
	CreatedAt        time.Time    `gorm:"autoCreateTime"`
	UpdatedAt        time.Time    `gorm:"autoUpdateTime"`
	DeletedAt        time.Time
}

type Repository struct {
	Id             string `gorm:"primary_key"`
	Name           string
	FullName       string
	Private        bool
	InstallationId string
	Installation   Installation `gorm:"foreignKey:InstallationId;references:Id"`
	CreatedAt      time.Time    `gorm:"autoCreateTime"`
	UpdatedAt      time.Time    `gorm:"autoUpdateTime"`
	DeletedAt      time.Time
}

type GithubInstallationResponseRepository struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Private  bool   `json:"private"`
}

type GithubInstallationResponseAccount struct {
	Login     string `json:"login"`
	ID        int    `json:"id"`
	AvatarURL string `json:"avatar_url"`
	Type      string `json:"type"`
}

type GithubInstallationResponseInstallation struct {
	ID        int                               `json:"id"`
	Account   GithubInstallationResponseAccount `json:"account"`
	CreatedAt time.Time                         `json:"created_at"`
	UpdatedAt time.Time                         `json:"updated_at"`
}

type GithubInstallationResponseSender struct {
	Login string `json:"login"`
	ID    int    `json:"id"`
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
	githubSessionSecret = os.Getenv("GITHUB_SESSION_SECRET")
	githubAppUrl = os.Getenv("GITHUB_APP_URL")
	githubAppId = os.Getenv("GITHUB_APP_ID")
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
	gothic.Store = sessions.NewCookieStore([]byte(githubSessionSecret))
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

	u := User{Id: user.UserID, Name: user.Name, Email: user.Email}
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
	value := queryParams.Get("installation_id")
	fmt.Println(value)

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func handleHome(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "you are home"})
}

func handleGithubHook(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read request body"})
		return
	}

	var response GithubInstallationResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		fmt.Println("Error:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse request body"})
		return
	}

	fmt.Println("Received GitHub App webhook event:")
	fmt.Println(string(body))

	if response.Action == "created" {
		err = updateInstallation(response, c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
	} else {
		fmt.Printf("Unhandled webhook event for app: %s", response.Action)
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func updateInstallation(response GithubInstallationResponse, c *gin.Context) error {
	fmt.Println("Received GitHub App webhook event: created, handling it")

	var user User

	if err := db.First(&user, "id = ?", fmt.Sprint(response.Sender.ID)).Error; err != nil {
		return errors.New("failed to find a registered user for this installation")
	}

	tx := db.Begin()

	installation := Installation{
		Id:               fmt.Sprint(response.Installation.ID),
		AccountType:      response.Installation.Account.Type,
		AccountID:        fmt.Sprint(response.Installation.Account.ID),
		AccountLogin:     response.Installation.Account.Login,
		AccountAvatarUrl: response.Installation.Account.AvatarURL,
		UserId:           user.Id,
	}
	result := tx.Create(&installation)

	if result.Error != nil {
		tx.Rollback()
		return errors.New("failed to save the installation")
	}

	for _, repo := range response.Repositories {
		repository := Repository{
			Id:             fmt.Sprint(repo.ID),
			Name:           repo.Name,
			FullName:       repo.FullName,
			Private:        repo.Private,
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

func main() {
	initEnv()
	initGithubAuth()
	initDB(dbName)
	initServer()
}
