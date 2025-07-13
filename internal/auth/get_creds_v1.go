package auth

import (
	"errors"
	"os"
)

var (
	errNoUsername = errors.New("VA_USERNAME is not set")
	errNoPassword = errors.New("VA_PASSWORD is not set")
)

var (
	_userName string
	_password string
)

func Load() error {
	// read from env
	username := os.Getenv("VA_USERNAME")
	if username == "" {
		return errNoUsername
	}
	_userName = username

	password := os.Getenv("VA_PASSWORD")
	if password == "" {
		return errNoPassword
	}
	_password = password
	return nil
}

func GetUserName() string {
	return _userName
}

func GetPassword() string {
	// read from env
	return _password
}
