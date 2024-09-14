package v1

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/srad/streamsink/app"
	"net/http"
)

// GetUserProfile godoc
// @Summary     Get user profile
// @Description Get user profile
// @Tags        user
// @Accept      json
// @Produce     json
// @Success     200 {any} JWT token for authentication
// @Failure     400 {} http.StatusBadRequest
// @Router      /user/profile [post]
func GetUserProfile(c *gin.Context) {
	appG := app.Gin{C: c}
	user, exists := c.Get("currentUser")

	if !exists {
		appG.Error(http.StatusBadRequest, errors.New("user does not exist"))
		return
	}

	appG.Response(http.StatusOK, user)
}
