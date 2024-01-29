package web

import (
	"buildkansen/config"
	"buildkansen/models"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"net/http"
)

func HandleHome(c *gin.Context) {
	userValue, exists := c.Get("user")

	if exists {
		user, _ := userValue.(models.User)
		installations, repositories := models.FetchInstallationsAndRepositories(&user)

		headers := gin.H{
			"user":          user,
			"installations": installations,
			"repositories":  repositories,
			"runnerLabels":  config.C.ValidRunnerNames,
		}

		c.HTML(http.StatusOK, "index.html", headers)
	} else {
		c.HTML(http.StatusOK, "login.html", gin.H{})
	}
}

func HandleLogout(c *gin.Context) {
	session := sessions.Default(c)
	session.Delete(config.C.AuthorizedUserInSessionKey)
	_ = session.Save()

	c.Redirect(http.StatusFound, "/")
}

func HandleLogin() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/")
	}
}
