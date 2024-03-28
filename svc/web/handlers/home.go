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
		installations, repositories, runs := models.FetchUserData(&user)

		headers := gin.H{
			"user":            user,
			"dataAvailable":   haveAvailableInstallationData(installations, repositories),
			"installationUrl": InstallationUrl(),
			"installations":   installations,
			"repositories":    repositories,
			"runs":            runs,
			"runnerLabels":    config.C.ValidRunnerNames,
			"isProduction":    isProduction.(bool),
		}

		c.HTML(http.StatusOK, "index.html", headers)
	} else {
		c.HTML(http.StatusOK, "login.html", gin.H{"isProduction": isProduction.(bool)})
	}
}

func haveAvailableInstallationData(installations []models.Installation, repositories []models.Repository) bool {
	if len(installations) == 0 && len(repositories) == 0 {
		return false
	}

	return true
}

func HandleLogout(c *gin.Context) {
	session := sessions.Default(c)
	session.Delete(config.C.AuthorizedUserInSessionKey)
	_ = session.Save()

	c.Redirect(http.StatusFound, "/")
}

func HandleAccountDestroy(c *gin.Context) {
	userValue, exists := c.Get("user")

	if exists {
		user, _ := userValue.(models.User)
		err := models.DestroyUserData(&user)

		if err != nil {
			c.String(http.StatusNotFound, "Failed to destroy user data")
			return
		}
	}

	c.Redirect(http.StatusFound, "/")
}
