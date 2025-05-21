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

	// What kind of error do we have here
	if err != nil {
		var ve *jwt.ValidationError
		if errors.As(err, &ve) {
			if ve.Errors&jwt.ValidationErrorMalformed != 0 {
				log.Error("Malformed token")
				appG.Error(http.StatusUnauthorized, errors.New("malformed token"))
			} else if ve.Errors&(jwt.ValidationErrorExpired|jwt.ValidationErrorNotValidYet) != 0 {
				// Token is either expired or not active yet
				log.Warn("Token expired or not yet valid")
				appG.Error(http.StatusUnauthorized, errors.New("token expired or not yet valid"))
			} else {
				log.Errorf("Couldn't handle this token: %v", err)
				appG.Error(http.StatusUnauthorized, errors.New("couldn't handle this token"))
			}
		} else {
			log.Errorf("JWT parsing error: %v", err)
			appG.Error(http.StatusUnauthorized, errors.New("invalid token"))
		}
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		appG.Error(http.StatusUnauthorized, errors.New("invalid token"))
		return
	}

	exp, ok := claims["exp"].(float64)
	if !ok || float64(time.Now().Unix()) > exp {
		appG.Error(http.StatusUnauthorized, errors.New("token expired or invalid"))
		return
	}

	idFloat, ok := claims["id"].(float64)
	if !ok {
		appG.Error(http.StatusUnauthorized, errors.New("invalid token payload"))
		return
	}

	id := uint(idFloat)
	user, err := services.GetUserByID(id)
	if err != nil {
		appG.Error(http.StatusUnauthorized, errors.New("user not found or invalid"))
		return
	}

	c.Set("currentUser", user)
	c.Next()
}
