package middlewares

import (
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/srad/mediasink/app"
	"github.com/srad/mediasink/services"
)

func CheckAuthorizationHeader(c *gin.Context) {
	appG := app.Gin{C: c}
	var authHeader = c.GetHeader("Authorization")

	if authHeader == "" {
		// Workaround for JWT over websockets. The bearer can also be sent as get parameter.
		if getAuth, exists := c.GetQuery("Authorization"); exists && getAuth != "" {
			authHeader = "Bearer " + getAuth
			log.Info("Received authentication as get parameter. Likely from a socket.")
		} else {
			appG.Error(http.StatusUnauthorized, errors.New("authorization header is missing"))
			return
		}
	}

	authToken := strings.Split(authHeader, " ")
	if len(authToken) != 2 || authToken[0] != "Bearer" {
		appG.Error(http.StatusUnauthorized, errors.New("invalid token format"))
		return
	}

	tokenString := authToken[1]
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(os.Getenv("SECRET")), nil
	})
	if err != nil {
		appG.Error(http.StatusInternalServerError, errors.New("invalid or expired token"))
		return
	}
	if !token.Valid {
		appG.Error(http.StatusUnauthorized, errors.New("invalid or expired token"))
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		appG.Error(http.StatusUnauthorized, errors.New("invalid token"))
		return
	}

	if float64(time.Now().Unix()) > claims["exp"].(float64) {
		appG.Error(http.StatusUnauthorized, errors.New("token expired"))
		return
	}

	// Interface conversion for numbers on map interface seems to be float64, wtf?
	id := uint(claims["id"].(float64))
	user, err := services.GetUserByID(id)
	if err != nil {
		appG.Error(http.StatusUnauthorized, err)
		return
	}

	c.Set("currentUser", user)
	c.Next()
}
