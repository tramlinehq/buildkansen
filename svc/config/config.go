package config

import (
	"buildkansen/log"
	"github.com/joho/godotenv"
	"os"
	"strconv"
)

type AppConfig struct {
	AppEnv                       string
	DbName                       string
	DbConnectionString           string
	SessionName                  string
	SessionSecret                string
	GithubAppUrl                 string
	GithubAppId                  int64
	GithubClientID               string
	GithubClientSecret           string
	GithubAuthRedirectUrl        string
	GithubAppRedirectUrl         string
	GithubAppInstallationBaseUrl string
	GithubPrivateKeyBase64       string
	GithubNewInstallationUrl     string
	AuthorizedUserInSessionKey   string
	InternalApiToken             string
	ValidRunnerNames             []string
}

var C *AppConfig

func Load() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file")
		panic(err)
	}

	C = &AppConfig{
		AppEnv:                       getEnv("ENV", "development"),
		DbName:                       getEnv("APP_NAME", ""),
		DbConnectionString:           getEnv("DB_CONNECTION_STRING", ""),
		SessionName:                  getEnv("APP_NAME", ""),
		SessionSecret:                getEnv("SESSION_SECRET", ""),
		GithubAppUrl:                 getEnv("GITHUB_APP_URL", ""),
		GithubAppId:                  parseInt64Env("GITHUB_APP_ID", 0),
		GithubClientID:               getEnv("GITHUB_CLIENT_ID", ""),
		GithubClientSecret:           getEnv("GITHUB_CLIENT_SECRET", ""),
		GithubAuthRedirectUrl:        getEnv("GITHUB_AUTH_REDIRECT_URL", ""),
		GithubAppRedirectUrl:         getEnv("GITHUB_APP_REDIRECT_URL", ""),
		GithubPrivateKeyBase64:       getEnv("GITHUB_PRIVATE_KEY_BASE64", ""),
		GithubAppInstallationBaseUrl: getEnv("GITHUB_NEW_INSTALLATION_URL", ""),
		InternalApiToken:             getEnv("INTERNAL_API_TOKEN", ""),
		AuthorizedUserInSessionKey:   "User ID",
		ValidRunnerNames:             []string{"tramline-macos-sonoma-md"},
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func parseInt64Env(key string, defaultValue int64) int64 {
	if valueStr, exists := os.LookupEnv(key); exists {
		if value, err := strconv.ParseInt(valueStr, 10, 64); err == nil {
			return value
		}
	}
	return defaultValue
}
