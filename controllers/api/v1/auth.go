package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/srad/streamsink/app"
	"github.com/srad/streamsink/models/requests"
	"github.com/srad/streamsink/services"
	"net/http"
)

// CreateUser godoc
// @Summary     Create new user
// @Description Create new user
// @Tags        auth
// @Param       AuthenticationRequest body requests.AuthenticationRequest true "Username and password"
// @Accept      json
// @Produce     json
// @Success     200
// @Failure     400 {} http.StatusBadRequest
// @Failure     500 {} http.StatusInternalServerError
// @Router      /auth/signup [post]
func CreateUser(c *gin.Context) {
	appG := app.Gin{C: c}
	var auth requests.AuthenticationRequest

	if err := c.BindJSON(&auth); err != nil {
		appG.Error(http.StatusBadRequest, err)
		return
	}

	if err := services.CreateUser(auth); err != nil {
		appG.Error(http.StatusInternalServerError, err)
		return
	} else {
		appG.Response(http.StatusOK, nil)
	}
}

// Login godoc
// @Summary     Create new user
// @Description Create new user
// @Tags        auth
// @Param       AuthenticationRequest body requests.AuthenticationRequest true "Username and password"
// @Accept      json
// @Produce     json
// @Success     200 {string} JWT token for authentication
// @Failure     400 {} http.StatusBadRequest
// @Failure     500 {} http.StatusInternalServerError
// @Router      /auth/login [post]
func Login(c *gin.Context) {
	appG := app.Gin{C: c}

	var auth requests.AuthenticationRequest
	if err := c.BindJSON(&auth); err != nil {
		appG.Error(http.StatusBadRequest, err)
		return
	}

	jwt, err := services.AuthenticateUser(auth)
	if err != nil {
		appG.Error(http.StatusUnauthorized, err)
		return
	}

	appG.Response(http.StatusOK, gin.H{"token": jwt})
}
