package database

import (
	"errors"
	"time"
)

type User struct {
	UserId    uint   `json:"userId" gorm:"autoIncrement;primaryKey;column:user_id" extensions:"!x-nullable"`
	Username  string `json:"username" gorm:"unique"`
	Password  string `json:"password"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func ExistsUsername(username string) error {
	var count int64
	if err := Db.Model(&User{}).Where("username = ?", username).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("username already exists")
	}
	return nil
}

func CreateUser(user *User) error {
	if err := Db.Create(user).Error; err != nil {
		return err
	}

	return nil
}

func FindUserByUsername(username string) (*User, error) {
	var user *User
	if err := Db.Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}

	return user, nil
}

func FindUserById(id uint) (*User, error) {
	var user *User
	if err := Db.Where("user_id = ?", id).First(user).Error; err != nil {
		return nil, err
	}

	return user, nil
}
