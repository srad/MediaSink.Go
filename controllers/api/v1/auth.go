package v1

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/srad/mediasink/app"
    "github.com/srad/mediasink/models/requests"
    "github.com/srad/mediasink/services"
)

// CreateUser godoc
// @Summary     Create new user
// @Description Create new user
// @Tags        auth
// @Param       AuthenticationRequest body requests.AuthenticationRequest true "Username and password"
// @Accept      json
// @Produce     json
// @Success     200 {} nil "JWT token for authentication"
// @Failure     400 {string} string "Error message"
// @Failure     500 {string} string "Error message"
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
// @Summary     User login
// @Description User login
// @Tags        auth
// @Param       AuthenticationRequest body requests.AuthenticationRequest true "Username and password"
// @Accept      json
// @Produce     json
// @Success     200 {object} responses.LoginResponse "JWT token for authentication"
// @Failure     401 {string} string "Error message"
// @Failure     400 {string} string "Error message"
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
