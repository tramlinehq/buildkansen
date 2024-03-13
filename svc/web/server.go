package web

import (
	"buildkansen/config"
	"buildkansen/log"
	. "buildkansen/web/handlers"
	mw "buildkansen/web/middleware"
	"embed"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	gorrila "github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/github"
	"html/template"
	"net/http"
)

//go:embed assets views
var emb embed.FS

const (
	appPort         = ":8081"
	certFilePath    = "./config/certs/localhost.pem"
	certKeyFilePath = "./config/certs/localhost-key.pem"
)

func Run() {
	r := gin.Default()

	if config.C.AppEnv == "production" {
		templates := template.Must(template.New("").Funcs(templateFuncs()).ParseFS(emb, "views/*.html"))
		r.SetHTMLTemplate(templates)
		r.StaticFS("/public", http.FS(emb))
	} else {
		r.SetFuncMap(templateFuncs())
		r.Static("/public/assets", "./web/assets")
		r.LoadHTMLGlob("./web/views/*")
	}

	store := cookie.NewStore([]byte(config.C.SessionSecret))
	r.Use(sessions.Sessions(config.C.SessionName, store))
	initGithubAuth()

	r.GET("/", mw.SetEnv(), mw.SetUserFromSessionMiddleware(), HandleHome)
	r.GET("/logout", mw.SetEnv(), HandleLogout)
	r.POST("/account/destroy", mw.SetEnv(), mw.SetUserFromSessionMiddleware(), HandleAccountDestroy)
	r.GET("/github/auth", mw.SetEnv(), mw.InjectGithubProvider(), GithubAuth)
	r.GET("/github/auth/register", mw.SetEnv(), mw.InjectGithubProvider(), GithubAuthCallback)
	r.GET("/github/apps/register", mw.SetEnv(), mw.InjectGithubProvider(), mw.SetUserFromSessionMiddleware(), GithubAppsCallback)
	r.POST("/github/apps/hook", mw.SetEnv(), GithubHook)
	r.PUT("/v1/api/internal/vm/bind", mw.SetEnv(), mw.InternalApiAuthMiddleware(), BindVM)

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

func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"inc": func(i int) int {
			return i + 1
		},
	}
}
