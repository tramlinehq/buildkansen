package middleware

import (
	"buildkansen/config"
	"buildkansen/models"
	"fmt"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"net/http"
)

func InjectGithubProvider() gin.HandlerFunc {
	return func(c *gin.Context) {
		q := c.Request.URL.Query()
		q.Add("provider", "github")
		c.Request.URL.RawQuery = q.Encode()
	}
}

func SetUserFromSessionMiddleware() gin.HandlerFunc {
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

func getUserFromSession(c *gin.Context) (interface{}, error) {
	uId := sessions.Default(c).Get(config.C.AuthorizedUserInSessionKey)

	if uId == nil {
		return nil, fmt.Errorf("no user found")
	}

	result, err := models.FindEntityById(models.User{}, uId.(int64))
	if err != nil {
		return nil, err
	}

	return result, nil
}
