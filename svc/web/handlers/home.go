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
	isProduction, _ := c.Get("isProduction")

	if exists {
		user, _ := userValue.(models.User)
		installations, repositories, runs := models.FetchDashboardData(&user)

		headers := gin.H{
			"user":          user,
			"installations": installations,
			"repositories":  repositories,
			"runs":          runs,
			"runnerLabels":  config.C.ValidRunnerNames,
		}

		c.HTML(http.StatusOK, "index.html", headers)
	} else {
		c.HTML(http.StatusOK, "login.html", gin.H{"isProduction": isProduction.(bool)})
	}
}

func HandleLogout(c *gin.Context) {
	session := sessions.Default(c)
	session.Delete(config.C.AuthorizedUserInSessionKey)
	_ = session.Save()

	c.Redirect(http.StatusFound, "/")
}
