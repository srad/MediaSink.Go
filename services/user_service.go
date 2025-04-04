package services

import (
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/srad/mediasink/database"
	"github.com/srad/mediasink/models/requests"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func CreateUser(auth requests.AuthenticationRequest) error {
	if err := database.ExistsUsername(auth.Username); err != nil {
		return err
	}

	if passwordHash, err := bcrypt.GenerateFromPassword([]byte(auth.Password), bcrypt.DefaultCost); err != nil {
		return err
	} else {
		user := &database.User{
			Username: auth.Username,
			Password: string(passwordHash),
		}

		if err := database.CreateUser(user); err != nil {
			return err
		}

		return nil
	}
}

// AuthenticateUser Returns a JWT string if the authentication was successful.
func AuthenticateUser(auth requests.AuthenticationRequest) (string, error) {
	user, errUser := database.FindUserByUsername(auth.Username)

	if errors.Is(errUser, gorm.ErrRecordNotFound) {
		return "", errors.New("user not found")
	}

	if errUser != nil {
		return "", errUser
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(auth.Password)); err != nil {
		return "", err
	}

	generateToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":  user.UserID,
		"exp": time.Now().Add(time.Hour * 24).Unix(),
	})

	return generateToken.SignedString([]byte(os.Getenv("SECRET")))
}

func GetUserByID(userId uint) (*database.User, error) {
	return database.FindUserByID(userId)
}
