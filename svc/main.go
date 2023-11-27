package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

const (
	appPort         = ":8081"
	certFilePath    = "./config/certs/localhost.pem"
	certKeyFilePath = "./config/certs/localhost-key.pem"
)

var (
	appEnv                 string
	githubAppId            string
	githubAppSecret        string
	githubPrivateKeyBase64 string
	githubUrl              string
)

func initEnv() {
	e := godotenv.Load()

	if e != nil {
		log.Fatalf("Error loading .env file: %s", e)
	}

	appEnv = os.Getenv("ENV")
	githubAppId = os.Getenv("GITHUB_APP_ID")
	githubAppSecret = os.Getenv("GITHUB_APP_SECRET")
	githubPrivateKeyBase64 = os.Getenv("GITHUB_PRIVATE_KEY_BASE64")
	githubUrl = os.Getenv("GITHUB_URL")
}

func initServer() {
	r := gin.Default()
	r.GET("/", handleHome)
	r.POST("/github/hook", handleGithubHook)
  r.GET("/github/register", handleGithubRegistration)

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

func handleHome(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "you are home"})
	return
}

func handleGithubRegistration(c *gin.Context) {
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read request body"})
		return
	}

	fmt.Println("Received GitHub App callback event:")
	fmt.Println(string(body))

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func handleGithubHook(c *gin.Context) {
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read request body"})
		return
	}

	fmt.Println("Received GitHub App callback event:")
	fmt.Println(string(body))

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}


func main() {
	initEnv()
	initServer()
}
