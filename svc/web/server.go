package web

import (
	"buildkansen/config"
	"buildkansen/log"
	. "buildkansen/web/handlers"
	mw "buildkansen/web/middleware"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	gorrila "github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/github"
)

const (
	appPort         = ":8081"
	certFilePath    = "./config/certs/localhost.pem"
	certKeyFilePath = "./config/certs/localhost-key.pem"
)

func Run() {
	r := gin.Default()

	store := cookie.NewStore([]byte(config.C.SessionSecret))
	r.Use(sessions.Sessions(config.C.SessionName, store))
	initGithubAuth()

	r.Static("/assets", "./assets")
	r.LoadHTMLGlob("views/*")

	r.GET("/", mw.SetUserFromSessionMiddleware(), HandleHome)
	r.GET("/logout", HandleLogout)
	r.GET("/github/auth", mw.InjectGithubProvider(), GithubAuth)
	r.GET("/github/auth/register", mw.InjectGithubProvider(), GithubAuthCallback)
	r.GET("/github/apps/register", mw.InjectGithubProvider(), mw.SetUserFromSessionMiddleware(), GithubAppsCallback)
	r.POST("/github/apps/hook", GithubHook)
	r.PUT("/v1/api/internal/vm/register", mw.InternalApiAuthMiddleware(), RegisterVM)

	var err error

	if config.C.AppEnv == "production" {
		err = r.Run(appPort)
	} else {
		err = r.RunTLS(appPort, certFilePath, certKeyFilePath)
	}

	if err != nil {
		log.Fatalf("Error in starting the server")
		panic(err)
	}
}

func initGithubAuth() {
	gothic.Store = gorrila.NewCookieStore([]byte(config.C.SessionSecret))
	goth.UseProviders(
		github.New(config.C.GithubClientID, config.C.GithubClientSecret, config.C.GithubAuthRedirectUrl),
	)
}
